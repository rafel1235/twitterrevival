package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/mailer"
	"radura.local/api/internal/notifications"
	"radura.local/api/internal/posts"
	"radura.local/api/internal/users"
	"radura.local/api/migrations"
)

type capturingSender struct{ messages chan mailer.Message }

func (s *capturingSender) Send(ctx context.Context, m mailer.Message) error {
	select {
	case s.messages <- m:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type sprintFixture struct {
	t        *testing.T
	pool     *pgxpool.Pool
	handler  http.Handler
	accounts *auth.AccountService
	mail     *capturingSender
	people   *users.Service
}

func sprintDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; PostgreSQL integration not executed")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS citext"); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	schema := fmt.Sprintf("sprint2_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	if err = migrations.Run(ctx, pool, "up"); err != nil {
		t.Fatal(err)
	}
	return pool
}
func newSprintFixture(t *testing.T, emails bool) *sprintFixture {
	t.Helper()
	pool := sprintDatabase(t)
	repo := &auth.PostgresRepository{Pool: pool}
	passwords, err := auth.NewService(repo, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	people := users.NewService(&users.Repository{Pool: pool})
	options := []Option{WithUsers(people), WithPosts(posts.NewService(&posts.Repository{Pool: pool})), WithNotifications(notifications.NewService(pool))}
	f := &sprintFixture{t: t, pool: pool, people: people}
	if emails {
		f.mail = &capturingSender{make(chan mailer.Message, 16)}
		f.accounts = auth.NewAccountService(repo, passwords, f.mail, "http://localhost:3000")
		ctx, cancel := context.WithCancel(context.Background())
		f.accounts.Start(ctx)
		t.Cleanup(func() { cancel(); f.accounts.Wait() })
		options = append(options, WithAccounts(f.accounts))
	}
	f.handler, err = New(passwords, Config{"http://localhost:3000", false, time.Hour}, pool.Ping, options...)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *sprintFixture) call(method, path, body string, cookie *http.Cookie, want int) *httptest.ResponseRecorder {
	f.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Protection", "1")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != want {
		f.t.Fatalf("%s %s: got %d, expected %d: %s", method, path, w.Code, want, w.Body.String())
	}
	return w
}
func (f *sprintFixture) register(handle string) (auth.User, *http.Cookie) {
	f.t.Helper()
	body, _ := json.Marshal(auth.RegisterInput{Handle: handle, Email: handle + "@example.test", DisplayName: handle, Password: "a strong local password 🔐"})
	w := f.call("POST", "/api/v1/auth/register", string(body), nil, 201)
	var payload struct {
		Data struct {
			User auth.User `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		f.t.Fatal(err)
	}
	return payload.Data.User, w.Result().Cookies()[0]
}
func profilePayload(t *testing.T, w *httptest.ResponseRecorder) users.Profile {
	t.Helper()
	var p struct {
		Data struct {
			User users.Profile `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p.Data.User
}
func pagePayload(t *testing.T, w *httptest.ResponseRecorder) users.Page {
	t.Helper()
	var p struct {
		Data users.Page `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p.Data
}

func TestProfilesAndGraphAuthorization(t *testing.T) {
	f := newSprintFixture(t, false)
	a, ca := f.register("person_a")
	b, cb := f.register("person_b")
	c, _ := f.register("person_c")
	ctx := context.Background()
	f.call("GET", "/api/v1/users/person_b", "", nil, 401)
	for _, mutation := range []struct{ method, path string }{{"PATCH", "/api/v1/users/me"}, {"POST", "/api/v1/users/" + b.ID + "/follow"}, {"DELETE", "/api/v1/users/" + b.ID + "/block"}, {"POST", "/api/v1/auth/password/forgot"}} {
		request := httptest.NewRequest(mutation.method, mutation.path, strings.NewReader("{}"))
		request.AddCookie(ca)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-CSRF-Protection", "1")
		request.Header.Set("Origin", "https://untrusted.test")
		response := httptest.NewRecorder()
		f.handler.ServeHTTP(response, request)
		if response.Code != 403 {
			t.Fatal("Sprint 2 mutation bypassed origin check", mutation.path, response.Code)
		}
	}
	public := f.call("GET", "/api/v1/users/PERSON_B", "", ca, 200)
	if strings.Contains(public.Body.String(), "email") || strings.Contains(public.Body.String(), "password") {
		t.Fatal("private account data leaked")
	}
	updated := profilePayload(t, f.call("PATCH", "/api/v1/users/me", `{"display_name":"Ada 🌿","bio":"Caffè e idee","location_text":"Roma","website_url":"https://example.test","avatar_style":"lavender"}`, ca, 200))
	if updated.DisplayName != "Ada 🌿" || updated.AvatarStyle != "lavender" {
		t.Fatal("profile update not persisted")
	}
	f.call("PATCH", "/api/v1/users/me", `{"display_name":"Hacked"}`, nil, 401)
	f.call("PATCH", "/api/v1/users/me", `{"user_id":"`+b.ID+`","display_name":"Hacked"}`, ca, 400)
	f.call("PATCH", "/api/v1/users/me", `{"website_url":"javascript:alert(1)"}`, ca, 422)
	if p := profilePayload(t, f.call("GET", "/api/v1/users/person_b", "", ca, 200)); p.DisplayName != b.DisplayName {
		t.Fatal("another profile modified")
	}
	p := profilePayload(t, f.call("GET", "/api/v1/users/person_a", "", ca, 200))
	if p.Bio != "Caffè e idee" || !p.IsSelf {
		t.Fatal("profile did not survive fetch")
	}
	for i := 0; i < 2; i++ {
		f.call("POST", "/api/v1/users/"+b.ID+"/follow", "{}", ca, 204)
	}
	f.call("POST", "/api/v1/users/"+a.ID+"/follow", "{}", cb, 204)
	p = profilePayload(t, f.call("GET", "/api/v1/users/person_b", "", ca, 200))
	if !p.Following || !p.FollowedBy || p.FollowersCount != 1 || p.FollowingCount != 1 {
		t.Fatal("follow idempotence/counts")
	}
	f.call("POST", "/api/v1/users/"+a.ID+"/follow", "{}", ca, 422)
	f.call("POST", "/api/v1/users/not-a-uuid/follow", "{}", ca, 422)
	f.call("POST", "/api/v1/users/00000000-0000-0000-0000-000000000001/follow", "{}", ca, 404)
	f.call("POST", "/api/v1/users/"+c.ID+"/follow", "{}", ca, 204)
	first := pagePayload(t, f.call("GET", "/api/v1/users/"+a.ID+"/following?limit=1", "", ca, 200))
	if len(first.Users) != 1 || first.NextCursor == "" {
		t.Fatal("first cursor page")
	}
	second := pagePayload(t, f.call("GET", "/api/v1/users/"+a.ID+"/following?limit=1&cursor="+url.QueryEscape(first.NextCursor), "", ca, 200))
	if len(second.Users) != 1 || second.Users[0].ID == first.Users[0].ID || second.NextCursor != "" {
		t.Fatal("second cursor page")
	}
	f.call("GET", "/api/v1/users/"+a.ID+"/following?cursor=invalid!", "", ca, 422)
	for i := 0; i < 2; i++ {
		f.call("POST", "/api/v1/users/"+b.ID+"/block", "{}", ca, 204)
	}
	f.call("GET", "/api/v1/users/person_b", "", ca, 404)
	f.call("GET", "/api/v1/users/person_a", "", cb, 404)
	f.call("GET", "/api/v1/users/"+a.ID+"/followers", "", cb, 404)
	f.call("POST", "/api/v1/users/"+a.ID+"/follow", "{}", cb, 403)
	f.call("DELETE", "/api/v1/users/"+a.ID+"/block", "{}", cb, 204)
	f.call("GET", "/api/v1/users/person_a", "", cb, 404)
	blocked := pagePayload(t, f.call("GET", "/api/v1/users/me/blocks", "", ca, 200))
	if len(blocked.Users) != 1 || blocked.Users[0].ID != b.ID {
		t.Fatal("private blocked list")
	}
	other := pagePayload(t, f.call("GET", "/api/v1/users/me/blocks", "", cb, 200))
	if len(other.Users) != 0 {
		t.Fatal("blocked list leaked across users")
	}
	var count int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM follows WHERE (follower_id=$1 AND followed_id=$2) OR (follower_id=$2 AND followed_id=$1)`, a.ID, b.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("block did not remove both follows", err)
	}
	for i := 0; i < 2; i++ {
		f.call("DELETE", "/api/v1/users/"+b.ID+"/block", "{}", ca, 204)
	}
	p = profilePayload(t, f.call("GET", "/api/v1/users/person_b", "", ca, 200))
	if p.Following || p.FollowedBy {
		t.Fatal("unblock restored follows")
	}
	for _, table := range []string{"follows", "blocks"} {
		columns := "follower_id,followed_id"
		if table == "blocks" {
			columns = "blocker_id,blocked_id"
		}
		if _, err := f.pool.Exec(ctx, "INSERT INTO "+table+"("+columns+") VALUES($1,$1)", a.ID); err == nil {
			t.Fatal("self relationship constraint missing")
		}
	}
	if _, err := f.pool.Exec(ctx, "UPDATE users SET status='suspended' WHERE id=$1", c.ID); err != nil {
		t.Fatal(err)
	}
	f.call("GET", "/api/v1/users/person_c", "", ca, 404)
	if page := pagePayload(t, f.call("GET", "/api/v1/users/"+a.ID+"/following", "", ca, 200)); len(page.Users) != 0 {
		t.Fatal("inactive account exposed in list")
	}
}

func TestConcurrentFollowAndBlock(t *testing.T) {
	f := newSprintFixture(t, false)
	a, _ := f.register("race_a")
	b, _ := f.register("race_b")
	ctx := context.Background()
	for i := 0; i < 12; i++ {
		if err := f.people.Mutate(ctx, a.ID, b.ID, "unblock"); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		results := make(chan error, 2)
		var wg sync.WaitGroup
		for _, action := range []string{"follow", "block"} {
			wg.Add(1)
			go func(action string) {
				defer wg.Done()
				<-start
				actor, target := a.ID, b.ID
				if action == "follow" && i%2 == 1 {
					actor, target = target, actor
				}
				results <- f.people.Mutate(ctx, actor, target, action)
			}(action)
		}
		close(start)
		wg.Wait()
		close(results)
		for err := range results {
			if err != nil && !errors.Is(err, users.ErrBlocked) {
				t.Fatal(err)
			}
		}
		var n int
		if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM follows WHERE (follower_id=$1 AND followed_id=$2) OR (follower_id=$2 AND followed_id=$1)", a.ID, b.ID).Scan(&n); err != nil || n != 0 {
			t.Fatal("concurrent follow bypassed block", err)
		}
	}
}
func TestConcurrentAccountTokenConsumption(t *testing.T) {
	f := newSprintFixture(t, true)
	_, _ = f.register("single_use")
	token := f.emailToken(auth.VerificationPurpose)
	ctx := context.Background()
	results := make(chan error, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() { <-start; results <- f.accounts.VerifyEmail(ctx, token) }()
	}
	close(start)
	successes, invalid := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			successes++
		} else if errors.Is(err, auth.ErrInvalidToken) {
			invalid++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || invalid != 1 {
		t.Fatal("account token is not single use under concurrent requests")
	}
}

func (f *sprintFixture) emailToken(purpose string) string {
	f.t.Helper()
	select {
	case message := <-f.mail.messages:
		expected := "/verify-email"
		if purpose == auth.ResetPurpose {
			expected = "/reset-password"
		}
		for _, word := range strings.Fields(message.Body) {
			parsed, err := url.Parse(word)
			if err == nil && parsed.Path == expected && auth.ValidToken(parsed.Query().Get("token")) {
				return parsed.Query().Get("token")
			}
		}
		f.t.Fatal("email contains no valid action link")
	case <-time.After(5 * time.Second):
		f.t.Fatal("account email not delivered")
	}
	return ""
}
func TestEmailVerificationAndPasswordRecovery(t *testing.T) {
	f := newSprintFixture(t, true)
	a, cookie := f.register("mail_a")
	ctx := context.Background()
	verification := f.emailToken(auth.VerificationPurpose)
	var stored []byte
	if err := f.pool.QueryRow(ctx, "SELECT token_hash FROM auth_tokens WHERE user_id=$1 AND purpose=$2 AND consumed_at IS NULL", a.ID, auth.VerificationPurpose).Scan(&stored); err != nil || !bytes.Equal(stored, auth.TokenHash(verification)) {
		t.Fatal("account token not stored as hash", err)
	}
	f.call("POST", "/api/v1/auth/password/reset", `{"token":"`+verification+`","password":"a new strong password"}`, nil, 400)
	f.call("POST", "/api/v1/auth/verification/confirm", `{"token":"`+verification+`"}`, nil, 200)
	f.call("POST", "/api/v1/auth/verification/confirm", `{"token":"`+verification+`"}`, nil, 400)
	me := f.call("GET", "/api/v1/auth/me", "", cookie, 200)
	if !strings.Contains(me.Body.String(), `"email_verified":true`) {
		t.Fatal("email verification not persisted")
	}
	oldLogin := f.call("POST", "/api/v1/auth/login", `{"email":"mail_a@example.test","password":"a strong local password 🔐"}`, nil, 200).Result().Cookies()[0]
	var oldHash string
	if err := f.pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE id=$1", a.ID).Scan(&oldHash); err != nil {
		t.Fatal(err)
	}
	known := f.call("POST", "/api/v1/auth/password/forgot", `{"email":"mail_a@example.test"}`, nil, 202)
	unknown := f.call("POST", "/api/v1/auth/password/forgot", `{"email":"missing@example.test"}`, nil, 202)
	if known.Body.String() != unknown.Body.String() {
		t.Fatal("reset leaks account existence")
	}
	reset := f.emailToken(auth.ResetPurpose)
	f.call("POST", "/api/v1/auth/password/reset", `{"token":"`+reset+`","password":"a replacement password 🔐"}`, nil, 200)
	f.call("POST", "/api/v1/auth/password/reset", `{"token":"`+reset+`","password":"a replacement password 🔐"}`, nil, 400)
	f.call("GET", "/api/v1/auth/me", "", cookie, 401)
	f.call("GET", "/api/v1/auth/me", "", oldLogin, 401)
	f.call("POST", "/api/v1/auth/login", `{"email":"mail_a@example.test","password":"a strong local password 🔐"}`, nil, 401)
	f.call("POST", "/api/v1/auth/login", `{"email":"mail_a@example.test","password":"a replacement password 🔐"}`, nil, 200)
	raw, hash, err := auth.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	_ = raw
	if err := (&auth.PostgresRepository{Pool: f.pool}).CreateSession(ctx, a.ID, auth.NewSession{Hash: hash, ExpiresAt: time.Now().Add(time.Hour)}, oldHash); !errors.Is(err, auth.ErrUnauthorized) {
		t.Fatal("stale concurrent login can bypass password reset", err)
	}
	var newHash string
	if err := f.pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE id=$1", a.ID).Scan(&newHash); err != nil || newHash == oldHash || !auth.VerifyPassword(newHash, "a replacement password 🔐") {
		t.Fatal("password not rehashed", err)
	}
}
func TestExpiredAccountTokenAndMigrationUpgrade(t *testing.T) {
	f := newSprintFixture(t, true)
	a, cookie := f.register("expired_a")
	token := f.emailToken(auth.VerificationPurpose)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, "UPDATE auth_tokens SET created_at=now()-interval '2 hours',expires_at=now()-interval '1 hour' WHERE user_id=$1", a.ID); err != nil {
		t.Fatal(err)
	}
	f.call("POST", "/api/v1/auth/verification/confirm", `{"token":"`+token+`"}`, nil, 400)
	if err := migrations.Run(ctx, f.pool, "down"); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := f.pool.QueryRow(ctx, "SELECT max(version) FROM schema_migrations").Scan(&version); err != nil || version != migrations.CurrentVersion-1 {
		t.Fatal("down should remove only the latest version", err)
	}
	if err := migrations.Run(ctx, f.pool, "up"); err != nil {
		t.Fatal(err)
	}
	f.call("GET", "/api/v1/auth/me", "", cookie, 200)
	if _, err := f.pool.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES(99)"); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Run(ctx, f.pool, "up"); err == nil {
		t.Fatal("unknown migration version accepted")
	}
}
