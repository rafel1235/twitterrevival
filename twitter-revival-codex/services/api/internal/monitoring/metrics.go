package monitoring

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"runtime"
	"sort"
	"sync"
	"time"
)

var bounds = []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 60}

type key struct {
	method, route string
	status        int
}
type sample struct {
	count   uint64
	sum     float64
	buckets [9]uint64
}
type Metrics struct {
	Pool    *pgxpool.Pool
	Token   string
	started time.Time
	mu      sync.Mutex
	values  map[key]sample
}

func New(pool *pgxpool.Pool, token string) *Metrics {
	return &Metrics{Pool: pool, Token: token, started: time.Now(), values: map[key]sample{}}
}
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		defer func() {
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			method := r.Method
			switch method {
			case "GET", "POST", "DELETE", "PATCH", "HEAD", "OPTIONS":
			default:
				method = "OTHER"
			}
			status := wrapped.Status()
			if status == 0 {
				status = 200
			}
			duration := time.Since(start).Seconds()
			m.mu.Lock()
			defer m.mu.Unlock()
			k := key{method, route, status}
			if len(m.values) >= 512 {
				if _, ok := m.values[k]; !ok {
					k = key{"OTHER", "overflow", status}
				}
			}
			v := m.values[k]
			v.count++
			v.sum += duration
			for i, b := range bounds {
				if duration <= b {
					v.buckets[i]++
				}
			}
			m.values[k] = v
		}()
		next.ServeHTTP(wrapped, r)
	})
}

type Snapshot struct {
	UptimeSeconds       int64  `json:"uptime_seconds"`
	Requests            uint64 `json:"requests"`
	Errors              uint64 `json:"errors"`
	MailPending         int    `json:"mail_pending"`
	MailFailed          int    `json:"mail_failed"`
	MediaPending        int    `json:"media_pending"`
	DatabaseConnections int32  `json:"database_connections"`
}

func (m *Metrics) Snapshot(ctx context.Context) (Snapshot, error) {
	s := Snapshot{UptimeSeconds: int64(time.Since(m.started).Seconds()), DatabaseConnections: m.Pool.Stat().AcquiredConns()}
	m.mu.Lock()
	for k, v := range m.values {
		s.Requests += v.count
		if k.status >= 500 {
			s.Errors += v.count
		}
	}
	m.mu.Unlock()
	err := m.Pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM mail_jobs WHERE attempts<5),(SELECT count(*) FROM mail_jobs WHERE attempts>=5),(SELECT count(*) FROM media WHERE deleted_at IS NOT NULL AND purged_at IS NULL)").Scan(&s.MailPending, &s.MailFailed, &s.MediaPending)
	return s, err
}
func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(m.Token) < 32 {
		http.Error(w, "metrics unavailable", 503)
		return
	}
	got := sha256.Sum256([]byte(r.Header.Get("Authorization")))
	want := sha256.Sum256([]byte("Bearer " + m.Token))
	if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
		http.Error(w, "unauthorized", 401)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	s, err := m.Snapshot(ctx)
	if err != nil {
		http.Error(w, "metrics unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintln(w, "# TYPE radura_http_requests_total counter\n# TYPE radura_http_request_duration_seconds histogram")
	m.mu.Lock()
	keys := make([]key, 0, len(m.values))
	for k := range m.values {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })
	for _, k := range keys {
		v := m.values[k]
		labels := fmt.Sprintf("method=%q,route=%q,status=%q", k.method, k.route, fmt.Sprint(k.status))
		fmt.Fprintf(w, "radura_http_requests_total{%s} %d\n", labels, v.count)
		for i, b := range bounds {
			fmt.Fprintf(w, "radura_http_request_duration_seconds_bucket{%s,le=%q} %d\n", labels, fmt.Sprint(b), v.buckets[i])
		}
		fmt.Fprintf(w, "radura_http_request_duration_seconds_bucket{%s,le=\"+Inf\"} %d\nradura_http_request_duration_seconds_sum{%s} %g\nradura_http_request_duration_seconds_count{%s} %d\n", labels, v.count, labels, v.sum, labels, v.count)
	}
	m.mu.Unlock()
	for _, g := range []struct {
		name  string
		value any
	}{{"uptime_seconds", s.UptimeSeconds}, {"mail_jobs_pending", s.MailPending}, {"mail_jobs_failed", s.MailFailed}, {"media_purge_pending", s.MediaPending}, {"database_connections_acquired", s.DatabaseConnections}, {"goroutines", runtime.NumGoroutine()}} {
		fmt.Fprintf(w, "# TYPE radura_%s gauge\nradura_%s %v\n", g.name, g.name, g.value)
	}
}
