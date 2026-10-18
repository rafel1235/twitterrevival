package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"radura.local/api/internal/auth"
	"radura.local/api/internal/beta"
	"radura.local/api/internal/mailer"
	"radura.local/api/internal/moderation"
	"radura.local/api/internal/monitoring"
	"radura.local/api/internal/posts"
	"radura.local/api/internal/privacy"
	"radura.local/api/migrations"
)

func betaFixture(t *testing.T) (*sprintFixture, *beta.Service, auth.User, *http.Cookie) {
	t.Helper()
	f := newSprintFixture(t, false)
	moderator, cookie := f.register("beta_moderator")
	f.role(moderator.Handle, "moderator")
	b := &beta.Service{Pool: f.pool, RequireInvite: true, AllowTestCohorts: true}
	a, err := auth.NewService(&auth.PostgresRepository{Pool: f.pool, Beta: b}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	f.handler, err = New(a, Config{"http://localhost:3000", false, time.Hour}, f.pool.Ping, WithUsers(f.people), WithPosts(posts.NewService(&posts.Repository{Pool: f.pool})), WithBeta(b, monitoring.New(f.pool, strings.Repeat("m", 64))))
	if err != nil {
		t.Fatal(err)
	}
	return f, b, moderator, cookie
}
func betaInput(handle, token string) string {
	data, _ := json.Marshal(auth.RegisterInput{Handle: handle, Email: handle + "@example.test", DisplayName: handle, Password: accountPassword, InviteToken: token})
	return string(data)
}
func betaRegistered(t *testing.T, w *httptest.ResponseRecorder) (auth.User, *http.Cookie) {
	t.Helper()
	var payload struct {
		Data struct {
			User auth.User `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Data.User, w.Result().Cookies()[0]
}
func betaInvite(t *testing.T, b *beta.Service, handle string) beta.Issued {
	t.Helper()
	i, err := b.IssueLocal(context.Background(), "pilot", false, beta.Input{Email: handle + "@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	return i
}
func TestBetaInvitationLifecycleAndAuthorization(t *testing.T) {
	f, b, mod, cm := betaFixture(t)
	ctx := context.Background()
	f.call("GET", "/api/v1/beta", "", nil, 200)
	f.call("POST", "/api/v1/auth/register", betaInput("no_invite", ""), nil, 403)
	invite := betaInvite(t, b, "pilot_one")
	var hash []byte
	if err := f.pool.QueryRow(ctx, "SELECT token_hash FROM beta_invites WHERE id=$1", invite.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	expected := sha256.Sum256([]byte(invite.Token))
	if string(hash) != string(expected[:]) || strings.Contains(string(hash), invite.Token) {
		t.Fatal("invite is not hashed")
	}
	f.call("POST", "/api/v1/auth/register", betaInput("wrong_email", invite.Token), nil, 403)
	// Unique handle conflict rolls back invite consumption.
	conflict := strings.Replace(betaInput("pilot_one", invite.Token), `"handle":"pilot_one"`, `"handle":"beta_moderator"`, 1)
	f.call("POST", "/api/v1/auth/register", conflict, nil, 409)
	_, cp := betaRegistered(t, f.call("POST", "/api/v1/auth/register", betaInput("pilot_one", invite.Token), nil, 201))
	f.call("POST", "/api/v1/auth/register", betaInput("pilot_one", invite.Token), nil, 403)
	f.call("GET", "/api/v1/moderation/beta", "", nil, 401)
	f.call("GET", "/api/v1/moderation/beta", "", cp, 403)
	f.call("POST", "/api/v1/moderation/beta/invites", `{"email":"forbidden@example.test"}`, cp, 403)
	list := f.call("GET", "/api/v1/moderation/beta/invites", "", cm, 200).Body.String()
	if strings.Contains(list, invite.Token) || strings.Contains(list, "token_hash") || !strings.Contains(list, "used") {
		t.Fatal("unsafe invite projection")
	}
	revoke := betaInvite(t, b, "revoked_pilot")
	f.call("DELETE", "/api/v1/moderation/beta/invites/"+revoke.ID, `{}`, cm, 204)
	f.call("DELETE", "/api/v1/moderation/beta/invites/"+revoke.ID, `{}`, cm, 204)
	f.call("POST", "/api/v1/auth/register", betaInput("revoked_pilot", revoke.Token), nil, 403)
	expired := betaInvite(t, b, "expired_pilot")
	if _, err := f.pool.Exec(ctx, "UPDATE beta_invites SET created_at=now()-interval '2 days',expires_at=now()-interval '1 day' WHERE id=$1", expired.ID); err != nil {
		t.Fatal(err)
	}
	f.call("POST", "/api/v1/auth/register", betaInput("expired_pilot", expired.Token), nil, 403)
	paused := betaInvite(t, b, "paused_pilot")
	f.call("PATCH", "/api/v1/moderation/beta", `{"capacity":30,"enrollment_open":false}`, cm, 204)
	f.call("POST", "/api/v1/auth/register", betaInput("paused_pilot", paused.Token), nil, 403)
	f.call("PATCH", "/api/v1/moderation/beta", `{"capacity":29,"enrollment_open":true}`, cm, 422)
	f.call("PATCH", "/api/v1/moderation/beta", `{"capacity":30,"enrollment_open":true}`, cm, 204)
	req := httptest.NewRequest("PATCH", "/api/v1/moderation/beta", strings.NewReader(`{"capacity":30,"enrollment_open":false}`))
	req.AddCookie(cm)
	req.Header.Set("Origin", "https://evil.test")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("beta CSRF accepted")
	}
	login, _ := json.Marshal(auth.LoginInput{Email: mod.Email, Password: accountPassword})
	f.call("POST", "/api/v1/auth/login", string(login), nil, 200)
	var events int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM beta_invite_events WHERE invite_id=$1", revoke.ID).Scan(&events); err != nil || events != 2 {
		t.Fatal("revocation audit not idempotent", events, err)
	}
}

func TestBetaConcurrentConsumptionAndLastReservation(t *testing.T) {
	f, b, _, _ := betaFixture(t)
	ctx := context.Background()
	invite := betaInvite(t, b, "race_pilot")
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(betaInput("race_pilot", invite.Token)))
			r.Header.Set("Origin", "http://localhost:3000")
			r.Header.Set("X-CSRF-Protection", "1")
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			f.handler.ServeHTTP(w, r)
			results <- w.Code
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for code := range results {
		if code == 201 {
			success++
		} else if code != 403 {
			t.Fatalf("unexpected concurrent result %d", code)
		}
	}
	if success != 1 {
		t.Fatal("invite admitted more than once")
	}
	// Test cohorts can use capacity 1 without altering the pilot constraint.
	if _, err := f.pool.Exec(ctx, "INSERT INTO beta_cohorts(slug,kind,capacity) VALUES('load-last','test',1)"); err != nil {
		t.Fatal(err)
	}
	outcomes := make(chan error, 2)
	for _, email := range []string{"slot_a@example.test", "slot_b@example.test"} {
		wg.Add(1)
		go func(email string) {
			defer wg.Done()
			_, err := b.IssueLocal(ctx, "load-last", true, beta.Input{Email: email})
			outcomes <- err
		}(email)
	}
	wg.Wait()
	close(outcomes)
	success = 0
	for err := range outcomes {
		if err == nil {
			success++
		} else if !errors.Is(err, beta.ErrClosed) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatal("last slot overbooked")
	}
	b.AllowTestCohorts = false
	if _, err := b.IssueLocal(ctx, "load-other", true, beta.Input{Email: "other@example.test"}); !errors.Is(err, beta.ErrForbidden) {
		t.Fatal("nonlocal test bypass allowed", err)
	}
}

func TestBetaMilestonesRetentionAndRollback(t *testing.T) {
	f, b, mod, _ := betaFixture(t)
	ctx := context.Background()
	i := betaInvite(t, b, "metrics_pilot")
	pilot, cp := betaRegistered(t, f.call("POST", "/api/v1/auth/register", betaInput("metrics_pilot", i.Token), nil, 201))
	ti, err := b.IssueLocal(ctx, "e2e-excluded", true, beta.Input{Email: "excluded@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	f.call("POST", "/api/v1/auth/register", betaInput("excluded", ti.Token), nil, 201)
	f.call("POST", "/api/v1/users/"+mod.ID+"/follow", `{}`, cp, 204)
	post := f.post(cp, "Primo post della beta", "", "")
	f.call("POST", "/api/v1/posts/"+post.ID+"/likes", `{}`, cp, 200)
	if _, err = f.pool.Exec(ctx, "UPDATE users SET email_verified_at=now() WHERE id=$1", pilot.ID); err != nil {
		t.Fatal(err)
	}
	stats, err := b.Stats(ctx, mod.ID)
	if err != nil || stats.Members != 1 || stats.Activated != 1 || stats.FirstFollow != 1 || stats.FirstPost != 1 || stats.FirstInteraction != 1 || stats.D7Rate != nil {
		t.Fatalf("bad pilot stats %+v %v", stats, err)
	}
	if _, err = f.pool.Exec(ctx, "UPDATE beta_members SET joined_at=now()-interval '7 days 12 hours',last_seen_at=NULL WHERE user_id=$1", pilot.ID); err != nil {
		t.Fatal(err)
	}
	f.call("GET", "/api/v1/auth/me", "", cp, 200)
	if _, err = f.pool.Exec(ctx, "UPDATE beta_members SET joined_at=now()-interval '8 days 1 hour' WHERE user_id=$1", pilot.ID); err != nil {
		t.Fatal(err)
	}
	stats, err = b.Stats(ctx, mod.ID)
	if err != nil || stats.D7Eligible != 1 || stats.D7Returned != 1 || stats.D7Rate == nil || *stats.D7Rate != 1 {
		t.Fatalf("bad D7 %+v %v", stats, err)
	}
	if err = migrations.Run(ctx, f.pool, "down"); err != nil {
		t.Fatal(err)
	}
	legacy, err := auth.NewService(&auth.PostgresRepository{Pool: f.pool}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = legacy.Me(ctx, cp.Value); err != nil {
		t.Fatal("rollback invalidated session", err)
	}
	// Coordinated rollback retains existing identities and session hashes.
	var count int
	if err = f.pool.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE user_id=$1", pilot.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("rollback lost sessions", err)
	}
	if err = migrations.Run(ctx, f.pool, "up"); err != nil {
		t.Fatal(err)
	}
	if err = f.pool.QueryRow(ctx, "SELECT count(*) FROM beta_members").Scan(&count); err != nil || count != 0 {
		t.Fatal("rollback invented cohorts", err)
	}
}

type failingSender struct{ called chan struct{} }

type mailSenderFunc func(context.Context, mailer.Message) error

func (f mailSenderFunc) Send(ctx context.Context, m mailer.Message) error { return f(ctx, m) }

func TestMailAcknowledgementCannotEraseNewerLease(t *testing.T) {
	f := newSprintFixture(t, false)
	user, _ := f.register("mail_lease")
	ctx := context.Background()
	repo := &auth.PostgresRepository{Pool: f.pool}
	a, err := auth.NewService(repo, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	delivered := make(chan error, 1)
	sender := mailSenderFunc(func(ctx context.Context, _ mailer.Message) error {
		// Simulate a new worker claiming an expired lease while the old worker's
		// SMTP response is delayed. The old acknowledgement must not own the row.
		_, err := f.pool.Exec(ctx, "UPDATE mail_jobs SET attempts=attempts+1,leased_until=now()+interval '1 hour' WHERE user_id=$1", user.ID)
		delivered <- err
		return err
	})
	worker := auth.NewAccountService(repo, a, sender, "http://localhost:3000")
	run, stop := context.WithCancel(ctx)
	worker.Start(run)
	if err = worker.RequestVerification(user.ID); err != nil {
		stop()
		worker.Wait()
		t.Fatal(err)
	}
	select {
	case err = <-delivered:
	case <-time.After(5 * time.Second):
		stop()
		worker.Wait()
		t.Fatal("mail not attempted")
	}
	stop()
	worker.Wait()
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = f.pool.QueryRow(ctx, "SELECT count(*) FROM mail_jobs WHERE user_id=$1 AND attempts=2", user.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("stale worker erased newer lease", count, err)
	}
}

func (s *failingSender) Send(context.Context, mailer.Message) error {
	select {
	case s.called <- struct{}{}:
	default:
	}
	return errors.New("test SMTP unavailable")
}
func TestDurableMailRetryAfterWorkerRestart(t *testing.T) {
	f := newSprintFixture(t, false)
	user, _ := f.register("durable_mail")
	ctx := context.Background()
	repo := &auth.PostgresRepository{Pool: f.pool}
	a, err := auth.NewService(repo, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	failed := &failingSender{make(chan struct{}, 1)}
	worker := auth.NewAccountService(repo, a, failed, "http://localhost:3000")
	run, cancel := context.WithCancel(ctx)
	worker.Start(run)
	if err = worker.RequestVerification(user.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-failed.called:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not attempt mail")
	}
	cancel()
	worker.Wait()
	var attempts, count int
	if err = f.pool.QueryRow(ctx, "SELECT attempts FROM mail_jobs WHERE user_id=$1", user.ID).Scan(&attempts); err != nil || attempts != 1 {
		t.Fatal("mail intent lost", attempts, err)
	}
	// Advance only the isolated fixture's retry/cooldown clock.
	if _, err = f.pool.Exec(ctx, "UPDATE mail_jobs SET available_at=now()-interval '1 minute',leased_until=NULL"); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, "UPDATE auth_tokens SET created_at=now()-interval '2 minutes'"); err != nil {
		t.Fatal(err)
	}
	captured := &capturingSender{make(chan mailer.Message, 1)}
	replacement := auth.NewAccountService(repo, a, captured, "http://localhost:3000")
	resumed, stop := context.WithCancel(ctx)
	replacement.Start(resumed)
	defer func() { stop(); replacement.Wait() }()
	select {
	case message := <-captured.messages:
		if message.To != user.Email || !strings.Contains(message.Body, "verify-email?token=") {
			t.Fatal("bad retry delivery")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("restart did not recover durable intent")
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err = f.pool.QueryRow(ctx, "SELECT count(*) FROM mail_jobs").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("successful retry not acknowledged")
}

func TestMetricsAuthorizationAndPrivateRouteLabels(t *testing.T) {
	f, _, _, cm := betaFixture(t)
	m := monitoring.New(f.pool, strings.Repeat("m", 64))
	fault, err := New(nil, Config{"http://localhost:3000", false, time.Hour}, func(context.Context) error { panic("fixture panic") }, WithBeta(nil, m))
	if err != nil {
		t.Fatal(err)
	}
	wf := httptest.NewRecorder()
	fault.ServeHTTP(wf, httptest.NewRequest("GET", "/healthz", nil))
	snapshot, err := m.Snapshot(context.Background())
	if err != nil || wf.Code != 500 || snapshot.Errors != 1 {
		t.Fatal("recovered server faults missing from metrics", err)
	}
	f.call("GET", "/api/v1/users/private_handle?token=secret", "", cm, 404)
	f.call("GET", "/api/v1/moderation/beta", "", cm, 200)
	f.call("GET", "/metrics", "", nil, 401)
	r := httptest.NewRequest("GET", "/metrics", nil)
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("m", 64))
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, `route="/api/v1/users/{handle}"`) || !strings.Contains(body, "radura_mail_jobs_pending 0") || !strings.Contains(body, `le="+Inf"`) {
		t.Fatal("invalid Prometheus exposition", w.Code, body)
	}
	for _, private := range []string{"private_handle", "secret", strings.Repeat("m", 64), "@example.test", "password_hash"} {
		if strings.Contains(body, private) {
			t.Fatal("private value in metrics", private)
		}
	}
}

func TestBetaPrivacyExportAndErasure(t *testing.T) {
	f, store, _ := newSafetyFixture(t)
	b := &beta.Service{Pool: f.pool, RequireInvite: true, AllowTestCohorts: true}
	a, err := auth.NewService(&auth.PostgresRepository{Pool: f.pool, Beta: b}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	p := privacy.NewService(f.pool, a, store)
	f.handler, err = New(a, Config{"http://localhost:3000", false, time.Hour}, f.pool.Ping, WithBeta(b, nil), WithSafety(&moderation.Service{Pool: f.pool}, p, nil))
	if err != nil {
		t.Fatal(err)
	}
	i := betaInvite(t, b, "private_pilot")
	ctx := context.Background()
	user, cookie := betaRegistered(t, f.call("POST", "/api/v1/auth/register", betaInput("private_pilot", i.Token), nil, 201))
	betaInvite(t, b, "unrelated_email")
	if _, err = f.pool.Exec(ctx, "INSERT INTO mail_jobs(email,purpose) VALUES($1,'password_reset')", user.Email); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"password": accountPassword})
	exported := f.call("POST", "/api/v1/account/export", string(body), cookie, 200)
	z, err := zip.NewReader(bytes.NewReader(exported.Body.Bytes()), int64(exported.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range z.File {
		if strings.HasSuffix(file.Name, ".json") {
			r, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(r)
			r.Close()
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			found = strings.Contains(text, `"beta"`) && strings.Contains(text, `"pilot"`)
			if strings.Contains(text, i.Token) || strings.Contains(text, "unrelated_email") || strings.Contains(text, "token_hash") {
				t.Fatal("export exposed private invite")
			}
		}
	}
	if !found {
		t.Fatal("own beta metrics missing from export")
	}
	deletion, _ := json.Marshal(map[string]string{"password": accountPassword, "confirmation": "ELIMINA"})
	f.call("DELETE", "/api/v1/account", string(deletion), cookie, 204)
	var count int
	err = f.pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM beta_members WHERE user_id=$1)+(SELECT count(*) FROM beta_invites WHERE used_by=$1 OR email=$2)+(SELECT count(*) FROM mail_jobs WHERE user_id=$1 OR email=$2)", user.ID, user.Email).Scan(&count)
	if err != nil || count != 0 {
		t.Fatal("beta/email personal data retained after erasure", count, err)
	}
}
