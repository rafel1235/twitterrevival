package httpapi

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type bucket struct {
	count int
	reset time.Time
}
type Limiter struct {
	mu       sync.Mutex
	entries  map[string]bucket
	limit    int
	window   time.Duration
	capacity int
}

func NewLimiter(limit int, window time.Duration, capacity int) *Limiter {
	return &Limiter{entries: make(map[string]bucket), limit: limit, window: window, capacity: capacity}
}
func (l *Limiter) allow(key string, now time.Time) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, exists := l.entries[key]
	if !exists || !now.Before(entry.reset) {
		if len(l.entries) >= l.capacity {
			for k, v := range l.entries {
				if !now.Before(v.reset) {
					delete(l.entries, k)
				}
			}
		}
		if !exists && len(l.entries) >= l.capacity {
			return false, int(l.window.Seconds())
		}
		entry = bucket{reset: now.Add(l.window)}
	}
	if entry.count >= l.limit {
		return false, int(entry.reset.Sub(now).Seconds()) + 1
	}
	entry.count++
	l.entries[key] = entry
	return true, 0
}
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never trust client-supplied X-Forwarded-For. The Next proxy shares a conservative peer quota.
		key, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			key = r.RemoteAddr
		}
		allowed, retry := l.allow(key, time.Now())
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			fail(w, 429, "rate_limited", "Troppe richieste. Riprova tra poco.", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}
