package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/backup"
	"radura.local/api/internal/beta"
	"radura.local/api/internal/httpapi"
	"radura.local/api/internal/mailer"
	"radura.local/api/internal/media"
	"radura.local/api/internal/moderation"
	"radura.local/api/internal/monitoring"
	"radura.local/api/internal/notifications"
	"radura.local/api/internal/posts"
	"radura.local/api/internal/privacy"
	"radura.local/api/internal/safety"
	"radura.local/api/internal/users"
	"radura.local/api/migrations"
)

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func run() error {
	migrate := flag.String("migrate", "", "apply migrations: up or down")
	healthcheck := flag.Bool("healthcheck", false, "check a running server")
	roleHandle := flag.String("role-user", "", "local operator: existing handle")
	role := flag.String("role", "", "local operator: moderator or member")
	reason := flag.String("reason", "", "audited reason for role change (10-2000 characters)")
	maintenance := flag.Bool("maintenance", false, "run privacy retention and media purge once")
	reconcile := flag.Bool("reconcile-media", false, "remove S3 orphans older than 24 hours")
	manifest := flag.Bool("backup-manifest", false, "print read-only database and S3 SHA-256 verification manifest")
	inviteEmail := flag.String("invite-email", "", "issue one email-bound beta invite; prints token once")
	inviteCohort := flag.String("invite-cohort", "pilot", "pilot or a local e2e-/load- cohort")
	testCohort := flag.Bool("test-cohort", false, "local-only cohort excluded from pilot metrics")
	flag.Parse()
	addr := env("API_ADDR", "127.0.0.1:8080")
	if *healthcheck {
		client := &http.Client{Timeout: 3 * time.Second}
		response, err := client.Get("http://127.0.0.1:8080/healthz")
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return fmt.Errorf("health status %d", response.StatusCode)
		}
		return nil
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	operationTimeout := 30 * time.Second
	if *manifest || *reconcile || *maintenance {
		operationTimeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return errors.New("invalid DATABASE_URL")
	}
	poolConfig.MaxConns = 10
	poolConfig.ConnConfig.ConnectTimeout = 5 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return errors.New("could not create database pool")
	}
	defer pool.Close()
	if err = pool.Ping(ctx); err != nil {
		return errors.New("database connection failed")
	}
	if *migrate != "" {
		return migrations.Run(ctx, pool, *migrate)
	}
	origin := env("APP_ORIGIN", "http://localhost:3000")
	u, e := url.Parse(origin)
	localOrigin := e == nil && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
	inviteOnly, err := strconv.ParseBool(env("BETA_INVITE_ONLY", "true"))
	if err != nil {
		return errors.New("invalid BETA_INVITE_ONLY")
	}
	if !localOrigin && !inviteOnly {
		return errors.New("public origins require invite-only beta")
	}
	betaService := &beta.Service{Pool: pool, RequireInvite: inviteOnly, AllowTestCohorts: localOrigin}
	if *inviteEmail != "" {
		result, err := betaService.IssueLocal(ctx, *inviteCohort, *testCohort, beta.Input{Email: *inviteEmail})
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	moderators := &moderation.Service{Pool: pool}
	if *roleHandle != "" || *role != "" {
		return moderators.SetRole(ctx, *roleHandle, *role, *reason)
	}
	ttl, err := strconv.Atoi(env("SESSION_TTL_HOURS", "168"))
	if err != nil || ttl < 1 || ttl > 720 {
		return errors.New("SESSION_TTL_HOURS must be between 1 and 720")
	}
	secure, err := strconv.ParseBool(env("COOKIE_SECURE", "false"))
	if err != nil {
		return errors.New("invalid COOKIE_SECURE")
	}
	duration := time.Duration(ttl) * time.Hour
	service, err := auth.NewService(&auth.PostgresRepository{Pool: pool, Beta: betaService}, duration)
	if err != nil {
		return err
	}
	storage, err := media.NewS3(env("S3_ENDPOINT", "http://127.0.0.1:8333"), env("S3_REGION", "us-east-1"), env("S3_BUCKET", "radura-media"), os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"))
	if err != nil {
		return err
	}
	if *manifest {
		if err = storage.Ready(ctx); err != nil {
			return errors.New("backup storage unavailable")
		}
		result, e := backup.Build(ctx, pool, storage)
		if e != nil {
			return errors.New("backup verification failed")
		}
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	if err = storage.Ensure(ctx); err != nil {
		return err
	}
	privacyService := privacy.NewService(pool, service, storage)
	if *reconcile {
		count, err := privacyService.Reconcile(ctx)
		if err != nil {
			return errors.New("media reconciliation failed")
		}
		slog.Info("media reconciliation", "purged", count)
		return nil
	}
	if *maintenance {
		result, err := privacyService.Sweep(ctx)
		if err != nil {
			return errors.New("privacy maintenance failed")
		}
		slog.Info("privacy maintenance", "purged", result.Purged, "pending", result.Pending)
		if result.Pending > 0 {
			return errors.New("storage deletions pending retry")
		}
		return nil
	}
	healthy := func(ctx context.Context) error {
		var version int
		if err := pool.QueryRow(ctx, "SELECT COALESCE(max(version),0) FROM schema_migrations").Scan(&version); err != nil {
			return err
		}
		if version != migrations.CurrentVersion {
			return errors.New("schema version mismatch")
		}
		return storage.Ready(ctx)
	}
	startTLS, err := strconv.ParseBool(env("SMTP_STARTTLS", "false"))
	if err != nil {
		return errors.New("invalid SMTP_STARTTLS")
	}
	localMail, err := strconv.ParseBool(env("SMTP_ALLOW_LOCAL_PLAINTEXT", "false"))
	if err != nil {
		return errors.New("invalid SMTP_ALLOW_LOCAL_PLAINTEXT")
	}
	if secure && !startTLS && !(localOrigin && localMail && env("SMTP_ADDR", "127.0.0.1:1025") == "mailpit:1025") {
		return errors.New("SMTP_STARTTLS is required with HTTPS")
	}
	sender, err := mailer.NewSMTP(env("SMTP_ADDR", "127.0.0.1:1025"), env("SMTP_FROM", "Radura <no-reply@radura.test>"), os.Getenv("SMTP_USERNAME"), os.Getenv("SMTP_PASSWORD"), startTLS)
	if err != nil {
		return err
	}
	accounts := auth.NewAccountService(&auth.PostgresRepository{Pool: pool}, service, sender, origin)
	profiles := users.NewService(&users.Repository{Pool: pool})
	postService := posts.NewService(&posts.Repository{Pool: pool})
	metricsToken := os.Getenv("METRICS_TOKEN")
	if metricsToken != "" && len(metricsToken) < 32 {
		return errors.New("METRICS_TOKEN must have at least 32 characters")
	}
	metrics := monitoring.New(pool, metricsToken)
	handler, err := httpapi.New(service, httpapi.Config{Origin: origin, CookieSecure: secure, SessionTTL: duration}, healthy, httpapi.WithUsers(profiles), httpapi.WithAccounts(accounts), httpapi.WithPosts(postService), httpapi.WithMedia(media.NewService(&media.Repository{Pool: pool, Store: storage})), httpapi.WithNotifications(notifications.NewService(pool)), httpapi.WithSafety(moderators, privacyService, &safety.Guard{Pool: pool}), httpapi.WithBeta(betaService, metrics))
	if err != nil {
		return err
	}
	if err = healthy(ctx); err != nil {
		return errors.New("apply migrations before starting server")
	}
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 70 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	accounts.Start(shutdown)
	maintenanceDone := make(chan struct{})
	go func() { defer close(maintenanceDone); privacyService.RunMaintenance(shutdown) }()
	defer func() { stop(); <-maintenanceDone }()
	defer func() { stop(); accounts.Wait() }()
	done := make(chan error, 1)
	go func() { slog.Info("API listening", "addr", addr); done <- server.ListenAndServe() }()
	select {
	case err = <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-shutdown.Done():
		timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(timeout)
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		slog.Error("API stopped", "error", err)
		os.Exit(1)
	}
}
