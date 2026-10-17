package httpapi

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"radura.local/api/internal/auth"
	"radura.local/api/migrations"
)

func TestPostgresAuthLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; real PostgreSQL integration not executed")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err = admin.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS citext"); err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("auth_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for i := 0; i < 2; i++ {
		if err = migrations.Run(ctx, pool, "up"); err != nil {
			t.Fatal(err)
		}
	}
	makeHandler := func() http.Handler {
		t.Helper()
		s, err := auth.NewService(&auth.PostgresRepository{Pool: pool}, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		h, err := New(s, Config{"http://localhost:3000", false, time.Hour}, pool.Ping)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	h := makeHandler()
	call := func(method, path, body string, cookie *http.Cookie, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Origin", "http://localhost:3000")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Protection", "1")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: status %d want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		return w
	}
	call("GET", "/healthz", "", nil, 200)
	call("GET", "/api/v1/auth/me", "", nil, 401)
	call("POST", "/api/v1/auth/register", `{"handle":"ab","email":"wrong","display_name":"","password":"short"}`, nil, 422)
	body := `{"handle":"Ada_Riva","email":"Ada@Example.test","display_name":"Ada 🌿","password":"very strong password 🔐"}`
	registered := call("POST", "/api/v1/auth/register", body, nil, 201)
	cookie := registered.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.MaxAge != 3600 || !auth.ValidToken(cookie.Value) {
		t.Fatalf("bad development cookie flags: %+v", cookie)
	}
	var response struct {
		Data struct {
			User auth.User `json:"user"`
		} `json:"data"`
	}
	if err = json.Unmarshal(registered.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	userID := response.Data.User.ID
	var hash, storedToken string
	if err = pool.QueryRow(ctx, `SELECT u.password_hash,encode(s.token_hash,'hex') FROM users u JOIN sessions s ON s.user_id=u.id WHERE u.id=$1`, userID).Scan(&hash, &storedToken); err != nil {
		t.Fatal(err)
	}
	if !auth.VerifyPassword(hash, "very strong password 🔐") || storedToken != hex.EncodeToString(auth.TokenHash(cookie.Value)) || strings.Contains(registered.Body.String(), "password_hash") {
		t.Fatal("credential storage or response unsafe")
	}
	call("GET", "/api/v1/auth/me", "", cookie, 200)
	// Fresh service instance must find the persistent session after a restart.
	h = makeHandler()
	call("GET", "/api/v1/auth/me", "", cookie, 200)
	call("POST", "/api/v1/auth/register", strings.ReplaceAll(body, "Ada_Riva", "ADA_RIVA"), nil, 409)
	call("POST", "/api/v1/auth/register", strings.ReplaceAll(body, "Ada_Riva", "other_handle"), nil, 409)
	call("POST", "/api/v1/auth/login", `{"email":"ada@example.test","password":"incorrect password"}`, nil, 401)
	call("POST", "/api/v1/auth/login", `{"email":"unknown@example.test","password":"incorrect password"}`, nil, 401)
	loggedIn := call("POST", "/api/v1/auth/login", `{"email":"ADA@example.test","password":"very strong password 🔐"}`, nil, 200)
	loginCookie := loggedIn.Result().Cookies()[0]
	if loginCookie.Value == cookie.Value {
		t.Fatal("login reused existing session token")
	}
	call("GET", "/api/v1/auth/me", "", loginCookie, 200)
	logout := call("POST", "/api/v1/auth/logout", "{}", loginCookie, 204)
	if logout.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout did not expire cookie")
	}
	call("GET", "/api/v1/auth/me", "", loginCookie, 401)
	call("POST", "/api/v1/auth/logout", "{}", loginCookie, 204)
	if _, err = pool.Exec(ctx, `UPDATE users SET status='suspended' WHERE id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	call("GET", "/api/v1/auth/me", "", cookie, 401)
	call("POST", "/api/v1/auth/login", `{"email":"ada@example.test","password":"very strong password 🔐"}`, nil, 401)
	if _, err = pool.Exec(ctx, `UPDATE users SET status='active' WHERE id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE sessions SET created_at=now()-interval '2 hours', expires_at=now()-interval '1 hour' WHERE token_hash=$1`, auth.TokenHash(cookie.Value)); err != nil {
		t.Fatal(err)
	}
	call("GET", "/api/v1/auth/me", "", cookie, 401)
	for _, query := range []string{
		`INSERT INTO users(handle,email,password_hash) VALUES('ééé','invalid@example.test','$argon2id$invalid')`,
		`INSERT INTO profiles(user_id,display_name) VALUES(gen_random_uuid(),'Orphan')`,
		`INSERT INTO sessions(user_id,token_hash,expires_at) VALUES('` + userID + `',decode('aa','hex'),now()+interval '1 hour')`,
	} {
		if _, err = pool.Exec(ctx, query); err == nil {
			t.Fatalf("constraint not enforced: %s", query)
		}
	}
	// Forced session failure proves registration rolls user/profile back atomically.
	_, err = (&auth.PostgresRepository{Pool: pool}).Register(ctx, auth.NewUser{Handle: "rollback_user", Email: "rollback@example.test", DisplayName: "Rollback", PasswordHash: hash}, auth.NewSession{Hash: []byte{1}, ExpiresAt: time.Now().Add(time.Hour)})
	if err == nil {
		t.Fatal("expected invalid token constraint")
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE handle='rollback_user'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("registration was not atomic", err)
	}
	if err = migrations.Run(ctx, pool, "down"); err != nil {
		t.Fatal(err)
	}
	if err = migrations.Run(ctx, pool, "down"); err != nil {
		t.Fatal(err)
	}
	if err = migrations.Run(ctx, pool, "up"); err != nil {
		t.Fatal(err)
	}
}
