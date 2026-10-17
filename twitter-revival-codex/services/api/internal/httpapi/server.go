package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"radura.local/api/internal/auth"
)

type Config struct {
	Origin       string
	CookieSecure bool
	SessionTTL   time.Duration
}
type Server struct {
	auth       *auth.Service
	cfg        Config
	cookieName string
	healthy    func(context.Context) error
	limiter    *Limiter
}
type apiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func New(service *auth.Service, cfg Config, healthy func(context.Context) error) (http.Handler, error) {
	origin, err := url.Parse(cfg.Origin)
	if err != nil || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" || (origin.Scheme != "http" && origin.Scheme != "https") {
		return nil, errors.New("APP_ORIGIN must be an exact http(s) origin without path")
	}
	local := origin.Hostname() == "localhost" || origin.Hostname() == "127.0.0.1" || origin.Hostname() == "::1"
	if (!local && origin.Scheme != "https") || (origin.Scheme == "https" && !cfg.CookieSecure) || (origin.Scheme == "http" && cfg.CookieSecure) {
		return nil, errors.New("use HTTPS and secure cookies outside local development")
	}
	if cfg.SessionTTL <= 0 {
		return nil, errors.New("session TTL must be positive")
	}
	name := "radura_session"
	if cfg.CookieSecure {
		name = "__Host-radura_session"
	}
	s := &Server{auth: service, cfg: cfg, cookieName: name, healthy: healthy, limiter: NewLimiter(30, time.Minute, 4096)}
	router := chi.NewRouter()
	router.Use(s.recoverJSON, middleware.Timeout(10*time.Second), s.headers)
	router.Get("/healthz", s.health)
	router.Route("/api/v1/auth", func(r chi.Router) {
		r.Get("/me", s.me)
		r.Group(func(m chi.Router) {
			m.Use(s.originCheck, s.limiter.Middleware)
			m.Post("/register", s.register)
			m.Post("/login", s.login)
			m.Post("/logout", s.logout)
		})
	})
	router.NotFound(func(w http.ResponseWriter, r *http.Request) { fail(w, 404, "not_found", "Endpoint inesistente.", nil) })
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		fail(w, 405, "method_not_allowed", "Metodo non consentito.", nil)
	})
	return router, nil
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if value != nil {
		_ = json.NewEncoder(w).Encode(value)
	}
}
func fail(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	respond(w, status, map[string]any{"error": apiError{code, message, fields}})
}
func (s *Server) handleError(w http.ResponseWriter, err error) {
	var validation *auth.ValidationError
	switch {
	case errors.As(err, &validation):
		fail(w, 422, "validation_failed", "Controlla i dati inseriti.", validation.Fields)
	case errors.Is(err, auth.ErrConflict):
		fail(w, 409, "account_exists", "Handle o email già registrati.", nil)
	case errors.Is(err, auth.ErrUnauthorized):
		fail(w, 401, "unauthorized", "Credenziali o sessione non valide.", nil)
	case errors.Is(err, auth.ErrBusy):
		w.Header().Set("Retry-After", "5")
		fail(w, 503, "auth_busy", "Servizio occupato, riprova tra poco.", nil)
	default:
		slog.Error("request failed", "error", err)
		fail(w, 500, "internal_error", "Errore del servizio. Riprova più tardi.", nil)
	}
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		fail(w, 415, "unsupported_media_type", "Usa Content-Type: application/json.", nil)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(target)
	if err == nil {
		var extra any
		if tailErr := decoder.Decode(&extra); tailErr != io.EOF {
			if tailErr == nil {
				err = errors.New("multiple JSON values")
			} else {
				err = tailErr
			}
		}
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(w, 413, "body_too_large", "Richiesta troppo grande (massimo 8 KiB).", nil)
		} else {
			fail(w, 400, "invalid_json", "JSON non valido o campi non riconosciuti.", nil)
		}
		return false
	}
	return true
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var input auth.RegisterInput
	if !decode(w, r, &input) {
		return
	}
	result, err := s.auth.Register(r.Context(), input)
	if err != nil {
		s.handleError(w, err)
		return
	}
	s.setCookie(w, result.Token, result.ExpiresAt)
	respond(w, 201, map[string]any{"data": map[string]any{"user": result.User}})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input auth.LoginInput
	if !decode(w, r, &input) {
		return
	}
	result, err := s.auth.Login(r.Context(), input)
	if err != nil {
		s.handleError(w, err)
		return
	}
	s.setCookie(w, result.Token, result.ExpiresAt)
	respond(w, 200, map[string]any{"data": map[string]any{"user": result.User}})
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, err := s.auth.Me(r.Context(), s.token(r))
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": map[string]any{"user": user}})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	var input struct{}
	if !decode(w, r, &input) {
		return
	}
	if err := s.auth.Logout(r.Context(), s.token(r)); err != nil {
		s.handleError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.cookieName, Value: "", Path: "/", HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) token(r *http.Request) string {
	cookie, err := r.Cookie(s.cookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
func (s *Server) setCookie(w http.ResponseWriter, token string, expiry time.Time) {
	http.SetCookie(w, &http.Cookie{Name: s.cookieName, Value: token, Path: "/", HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: expiry, MaxAge: int(s.cfg.SessionTTL.Seconds())})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.healthy(ctx); err != nil {
		fail(w, 503, "unavailable", "Database non disponibile o migrazione mancante.", nil)
		return
	}
	respond(w, 200, map[string]any{"data": map[string]string{"status": "ok"}})
}
func (s *Server) headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			ref, err := url.Parse(r.Header.Get("Referer"))
			if err == nil && ref.Host != "" && ref.User == nil {
				origin = ref.Scheme + "://" + ref.Host
			}
		}
		fetchSite := r.Header.Get("Sec-Fetch-Site")
		if origin != s.cfg.Origin || r.Header.Get("X-CSRF-Protection") != "1" || (fetchSite != "" && fetchSite != "same-origin" && fetchSite != "none") {
			fail(w, 403, "forbidden_origin", "Origine o protezione CSRF non valida.", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) recoverJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				slog.Error("request panic", "path", strings.TrimSpace(r.URL.Path))
				fail(w, 500, "internal_error", "Errore del servizio.", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
