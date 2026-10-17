package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/httpapi"
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	ttl, err := strconv.Atoi(env("SESSION_TTL_HOURS", "168"))
	if err != nil || ttl < 1 || ttl > 720 {
		return errors.New("SESSION_TTL_HOURS must be between 1 and 720")
	}
	secure, err := strconv.ParseBool(env("COOKIE_SECURE", "false"))
	if err != nil {
		return errors.New("invalid COOKIE_SECURE")
	}
	duration := time.Duration(ttl) * time.Hour
	service, err := auth.NewService(&auth.PostgresRepository{Pool: pool}, duration)
	if err != nil {
		return err
	}
	healthy := func(ctx context.Context) error {
		var version int
		return pool.QueryRow(ctx, "SELECT version FROM schema_migrations WHERE version=1").Scan(&version)
	}
	handler, err := httpapi.New(service, httpapi.Config{Origin: env("APP_ORIGIN", "http://localhost:3000"), CookieSecure: secure, SessionTTL: duration}, healthy)
	if err != nil {
		return err
	}
	if err = healthy(ctx); err != nil {
		return errors.New("apply migrations before starting server")
	}
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
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
