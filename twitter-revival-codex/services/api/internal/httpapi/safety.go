package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgconn"
	"io"
	"net"
	"net/http"
	"radura.local/api/internal/moderation"
	"radura.local/api/internal/users"
	"strings"
	"time"
)

func requestTimeout(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		duration := 10 * time.Second
		if r.Method == "POST" && r.URL.Path == "/api/v1/account/export" {
			duration = 60 * time.Second
		}
		middleware.Timeout(duration)(next).ServeHTTP(w, r)
	})
}
func safeErrorClass(err error) string {
	var pg *pgconn.PgError
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.As(err, &pg):
		return "database"
	default:
		return "dependency"
	}
}
func (s *Server) limitAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.guard != nil {
			peer, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				peer = r.RemoteAddr
			}
			sum := sha256.Sum256([]byte(peer))
			if err = s.guard.Allow(r.Context(), "peer:"+hex.EncodeToString(sum[:]), "auth", 30); err != nil {
				s.handleError(w, err)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) limitAccount(w http.ResponseWriter, r *http.Request) bool {
	if s.guard == nil || r.Method == "GET" || r.Method == "HEAD" {
		return true
	}
	bucket, limit := "writes", 120
	path := strings.TrimRight(r.URL.Path, "/")
	switch {
	case strings.HasPrefix(path, "/api/v1/account"):
		bucket, limit = "erasure", 5
		if r.Method == "POST" {
			bucket, limit = "export", 3
		}
	case strings.HasPrefix(path, "/api/v1/reports"):
		bucket, limit = "reports", 10
	case strings.HasPrefix(path, "/api/v1/media"):
		bucket, limit = "media", 20
	case path == "/api/v1/posts" && r.Method == "POST":
		bucket, limit = "publish", 30
	}
	if err := s.guard.Allow(r.Context(), currentUser(r).ID, bucket, limit); err != nil {
		s.handleError(w, err)
		return false
	}
	return true
}
func (s *Server) safetyReady(w http.ResponseWriter) bool {
	if s.moderation == nil {
		fail(w, 503, "unavailable", "Moderazione non disponibile.", nil)
		return false
	}
	return true
}
func (s *Server) createReport(w http.ResponseWriter, r *http.Request) {
	if !s.safetyReady(w) {
		return
	}
	var input moderation.Input
	if !decode(w, r, &input) {
		return
	}
	id, err := s.moderation.Create(r.Context(), currentUser(r).ID, input)
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 201, map[string]any{"data": map[string]string{"id": id}})
}
func (s *Server) listReports(w http.ResponseWriter, r *http.Request) {
	if !s.safetyReady(w) {
		return
	}
	p, err := users.ParsePagination(r.URL.Query().Get("cursor"), r.URL.Query().Get("limit"))
	if err != nil {
		s.handleError(w, err)
		return
	}
	page, err := s.moderation.List(r.Context(), currentUser(r).ID, strings.HasPrefix(r.URL.Path, "/api/v1/moderation/"), r.URL.Query().Get("status"), p)
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": page})
}
func (s *Server) moderate(w http.ResponseWriter, r *http.Request) {
	if !s.safetyReady(w) {
		return
	}
	var input moderation.Decision
	if !decode(w, r, &input) {
		return
	}
	if err := s.moderation.Decide(r.Context(), currentUser(r).ID, chi.URLParam(r, "id"), input); err != nil {
		s.handleError(w, err)
		return
	}
	w.WriteHeader(204)
}
func (s *Server) moderationAudit(w http.ResponseWriter, r *http.Request) {
	if !s.safetyReady(w) {
		return
	}
	p, err := users.ParsePagination(r.URL.Query().Get("cursor"), r.URL.Query().Get("limit"))
	if err != nil {
		s.handleError(w, err)
		return
	}
	page, err := s.moderation.Audit(r.Context(), currentUser(r).ID, p)
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": page})
}
func (s *Server) exportAccount(w http.ResponseWriter, r *http.Request) {
	if s.privacy == nil {
		fail(w, 503, "unavailable", "Esportazione non disponibile.", nil)
		return
	}
	var input struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	archive, err := s.privacy.Export(r.Context(), currentUser(r), input.Password)
	if err != nil {
		s.handleError(w, err)
		return
	}
	defer archive.Close()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="radura-export.zip"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(200)
	_, _ = io.Copy(w, archive.File)
}
func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	if s.privacy == nil {
		fail(w, 503, "unavailable", "Cancellazione non disponibile.", nil)
		return
	}
	var input struct {
		Password     string `json:"password"`
		Confirmation string `json:"confirmation"`
	}
	if !decode(w, r, &input) {
		return
	}
	if err := s.privacy.Delete(r.Context(), currentUser(r), input.Password, input.Confirmation); err != nil {
		s.handleError(w, err)
		return
	}
	s.clearCookie(w)
	w.WriteHeader(204)
}
