package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	h, err := New(nil, Config{"http://localhost:3000", false, time.Hour}, func(context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestCanceledRequestsAndDeadlines(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{context.Canceled, 499}, {context.DeadlineExceeded, 504}} {
		w := httptest.NewRecorder()
		(&Server{}).handleError(w, tc.err)
		if w.Code != tc.status || strings.Contains(w.Body.String(), tc.err.Error()) {
			t.Fatalf("wrong cancellation/deadline response: %d", w.Code)
		}
	}
}

func TestOriginAndCSRF(t *testing.T) {
	for _, tc := range []struct {
		name, origin, referer, csrf, fetch string
		status                             int
	}{
		{name: "missing", status: 403},
		{name: "null", origin: "null", csrf: "1", status: 403},
		{name: "foreign", origin: "https://evil.test", csrf: "1", status: 403},
		{name: "prefix attack", origin: "http://localhost:3000.evil.test", csrf: "1", status: 403},
		{name: "missing custom header", origin: "http://localhost:3000", status: 403},
		{name: "cross site metadata", origin: "http://localhost:3000", csrf: "1", fetch: "cross-site", status: 403},
		{name: "valid origin", origin: "http://localhost:3000", csrf: "1", status: 415},
		{name: "referer fallback", referer: "http://localhost:3000/login", csrf: "1", status: 415},
		{name: "origin wins over referer", origin: "https://evil.test", referer: "http://localhost:3000/", csrf: "1", status: 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Referer", tc.referer)
			r.Header.Set("X-CSRF-Protection", tc.csrf)
			r.Header.Set("Sec-Fetch-Site", tc.fetch)
			w := httptest.NewRecorder()
			testRouter(t).ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status %d, want %d", w.Code, tc.status)
			}
		})
	}
}

func TestDecodeAndRoutes(t *testing.T) {
	for _, tc := range []struct {
		body   string
		status int
	}{
		{"{", 400}, {`{"extra":true}`, 400}, {`{} {}`, 400}, {`{} garbage`, 400}, {strings.Repeat(" ", 8193), 413},
	} {
		r := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(tc.body))
		r.Header.Set("Origin", "http://localhost:3000")
		r.Header.Set("X-CSRF-Protection", "1")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		testRouter(t).ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("body %q: %d != %d", tc.body[:min(len(tc.body), 30)], w.Code, tc.status)
		}
	}
	h := testRouter(t)
	for _, tc := range []struct {
		method, path string
		status       int
	}{{"GET", "/healthz", 200}, {"POST", "/healthz", 405}, {"GET", "/api/v1/missing", 404}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status {
			t.Fatalf("%s: %d", tc.path, w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cacheable API response")
		}
	}
	unhealthy, _ := New(nil, Config{"http://localhost:3000", false, time.Hour}, func(context.Context) error { return errors.New("private database details") })
	w := httptest.NewRecorder()
	unhealthy.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "private") {
		t.Fatal("health leaks details or incorrect status")
	}
}

func TestConfig(t *testing.T) {
	for _, cfg := range []Config{{"https://example.test", false, time.Hour}, {"http://example.test", false, time.Hour}, {"http://localhost:3000/path", false, time.Hour}, {"https://user@example.test", true, time.Hour}, {"http://localhost:3000", true, time.Hour}, {"http://localhost:3000", false, 0}} {
		if _, err := New(nil, cfg, nil); err == nil {
			t.Fatalf("accepted config %+v", cfg)
		}
	}
	if _, err := New(nil, Config{"https://example.test", true, time.Hour}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestLimiter(t *testing.T) {
	limiter := NewLimiter(2, time.Minute, 2)
	now := time.Now()
	if ok, _ := limiter.allow("a", now); !ok {
		t.Fatal("first request denied")
	}
	limiter.allow("a", now)
	if ok, retry := limiter.allow("a", now); ok || retry < 1 {
		t.Fatal("rate limit bypass")
	}
	limiter.allow("b", now)
	if ok, _ := limiter.allow("c", now); ok {
		t.Fatal("unbounded bucket allocation")
	}
	if ok, _ := limiter.allow("c", now.Add(time.Minute)); !ok {
		t.Fatal("expired buckets not reclaimed")
	}
	h := NewLimiter(1, time.Minute, 2).Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest("POST", "/", nil)
		r.Header.Set("X-Forwarded-For", []string{"1.2.3.4", "5.6.7.8"}[i])
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if i == 1 && (w.Code != 429 || w.Header().Get("Retry-After") == "") {
			t.Fatal("spoofed IP bypassed limiter")
		}
	}
}

func TestSecureCookie(t *testing.T) {
	s := &Server{cfg: Config{Origin: "https://example.test", CookieSecure: true, SessionTTL: time.Hour}, cookieName: "__Host-radura_session"}
	w := httptest.NewRecorder()
	s.setCookie(w, "opaque-token", time.Now().Add(time.Hour))
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "__Host-radura_session" || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].Domain != "" || cookies[0].Path != "/" || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("invalid HTTPS cookie")
	}
}
