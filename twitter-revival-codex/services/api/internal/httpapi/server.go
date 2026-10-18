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
	"time"

	"github.com/go-chi/chi/v5"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/beta"
	"radura.local/api/internal/media"
	"radura.local/api/internal/moderation"
	"radura.local/api/internal/monitoring"
	"radura.local/api/internal/notifications"
	"radura.local/api/internal/posts"
	"radura.local/api/internal/privacy"
	"radura.local/api/internal/safety"
	"radura.local/api/internal/users"
)

type Config struct {
	Origin       string
	CookieSecure bool
	SessionTTL   time.Duration
}
type Server struct {
	auth          *auth.Service
	cfg           Config
	cookieName    string
	healthy       func(context.Context) error
	limiter       *Limiter
	users         *users.Service
	accounts      *auth.AccountService
	posts         *posts.Service
	media         *media.Service
	notifications *notifications.Service
	moderation    *moderation.Service
	privacy       *privacy.Service
	guard         *safety.Guard
	beta          *beta.Service
	metrics       *monitoring.Metrics
}
type apiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type Option func(*Server)

func WithUsers(service *users.Service) Option { return func(s *Server) { s.users = service } }
func WithAccounts(service *auth.AccountService) Option {
	return func(s *Server) { s.accounts = service }
}
func WithPosts(service *posts.Service) Option { return func(s *Server) { s.posts = service } }
func WithMedia(service *media.Service) Option { return func(s *Server) { s.media = service } }
func WithNotifications(service *notifications.Service) Option {
	return func(s *Server) { s.notifications = service }
}
func WithSafety(m *moderation.Service, p *privacy.Service, g *safety.Guard) Option {
	return func(s *Server) { s.moderation = m; s.privacy = p; s.guard = g }
}

func New(service *auth.Service, cfg Config, healthy func(context.Context) error, options ...Option) (http.Handler, error) {
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
	for _, option := range options {
		option(s)
	}
	router := chi.NewRouter()
	if s.metrics != nil {
		router.Use(s.metrics.Middleware)
	}
	router.Use(s.recoverJSON, requestTimeout, s.headers)
	router.Get("/healthz", s.health)
	if s.metrics != nil {
		router.Handle("/metrics", s.metrics)
	}
	router.Get("/api/v1/beta", s.betaPublic)
	router.Route("/api/v1/moderation/beta", func(r chi.Router) {
		r.Use(s.requireUser)
		r.Get("/", s.betaDashboard)
		r.Get("/invites", s.betaInvites)
		r.With(s.originCheck).Patch("/", s.betaConfigure)
		r.With(s.originCheck).Post("/invites", s.betaInvites)
		r.With(s.originCheck).Delete("/invites/{id}", s.betaInvites)
	})
	router.Route("/api/v1/auth", func(r chi.Router) {
		r.Get("/me", s.me)
		r.Group(func(m chi.Router) {
			m.Use(s.originCheck, s.limiter.Middleware, s.limitAuth)
			m.Post("/register", s.register)
			m.Post("/login", s.login)
			m.Post("/logout", s.logout)
		})
		r.Group(func(m chi.Router) {
			m.Use(s.originCheck, NewLimiter(10, time.Minute, 4096).Middleware, s.limitAuth)
			m.With(s.requireUser).Post("/verification/request", s.requestVerification)
			m.Post("/verification/confirm", s.confirmVerification)
			m.Post("/password/forgot", s.forgotPassword)
			m.Post("/password/reset", s.resetPassword)
		})
	})
	router.Route("/api/v1/users", func(r chi.Router) {
		r.Use(s.requireUser)
		r.Get("/me/blocks", s.blockedUsers)
		r.Get("/{id}/posts", func(w http.ResponseWriter, r *http.Request) { s.postList(w, r, "profile") })
		r.Get("/{handle}", s.profile)
		r.Get("/{id}/followers", func(w http.ResponseWriter, r *http.Request) { s.connections(w, r, "followers") })
		r.Get("/{id}/following", func(w http.ResponseWriter, r *http.Request) { s.connections(w, r, "following") })
		r.Group(func(m chi.Router) {
			m.Use(s.originCheck, NewLimiter(60, time.Minute, 4096).Middleware)
			m.Patch("/me", s.updateProfile)
			m.Post("/{id}/follow", func(w http.ResponseWriter, r *http.Request) { s.relationship(w, r, "follow") })
			m.Delete("/{id}/follow", func(w http.ResponseWriter, r *http.Request) { s.relationship(w, r, "unfollow") })
			m.Post("/{id}/block", func(w http.ResponseWriter, r *http.Request) { s.relationship(w, r, "block") })
			m.Delete("/{id}/block", func(w http.ResponseWriter, r *http.Request) { s.relationship(w, r, "unblock") })
		})
	})
	router.Route("/api/v1/posts", func(r chi.Router) {
		r.Use(s.requireUser)
		r.Get("/{id}", s.getPost)
		r.Get("/{id}/replies", func(w http.ResponseWriter, r *http.Request) { s.postList(w, r, "replies") })
		r.Group(func(m chi.Router) {
			m.Use(s.originCheck, NewLimiter(90, time.Minute, 4096).Middleware)
			m.Post("/", s.createPost)
			m.Patch("/{id}", func(w http.ResponseWriter, r *http.Request) { s.mutatePost(w, r, "edit") })
			m.Delete("/{id}", func(w http.ResponseWriter, r *http.Request) { s.mutatePost(w, r, "delete") })
			m.Post("/{id}/likes", func(w http.ResponseWriter, r *http.Request) { s.mutatePost(w, r, "like") })
			m.Delete("/{id}/likes", func(w http.ResponseWriter, r *http.Request) { s.mutatePost(w, r, "unlike") })
			m.Post("/{id}/reposts", func(w http.ResponseWriter, r *http.Request) { s.mutatePost(w, r, "repost") })
			m.Delete("/{id}/reposts", func(w http.ResponseWriter, r *http.Request) { s.mutatePost(w, r, "unrepost") })
		})
	})
	router.With(s.requireUser).Get("/api/v1/feed/home", func(w http.ResponseWriter, r *http.Request) { s.postList(w, r, "home") })
	router.With(s.requireUser, NewLimiter(120, time.Minute, 4096).Middleware).Get("/api/v1/search", s.search)
	router.Route("/api/v1/media", func(r chi.Router) {
		r.Use(s.requireUser)
		r.Get("/drafts", s.draftMedia)
		r.Get("/{id}", s.getMedia)
		r.Get("/{id}/content", s.getMedia)
		r.Group(func(m chi.Router) {
			m.Use(s.originCheck, NewLimiter(20, time.Minute, 4096).Middleware)
			m.Post("/", s.uploadMedia)
			m.Delete("/{id}", s.deleteMedia)
		})
	})
	router.Route("/api/v1/notifications", func(r chi.Router) {
		r.Use(s.requireUser)
		r.Get("/", s.listNotifications)
		r.Get("/unread-count", s.unreadNotifications)
		r.Group(func(m chi.Router) {
			m.Use(s.originCheck, NewLimiter(60, time.Minute, 4096).Middleware)
			m.Post("/read", s.readNotifications)
			m.Post("/{id}/read", s.readNotifications)
		})
	})
	router.Route("/api/v1/reports", func(r chi.Router) {
		r.Use(s.requireUser)
		r.Get("/", s.listReports)
		r.With(s.originCheck).Post("/", s.createReport)
	})
	router.Route("/api/v1/moderation", func(r chi.Router) {
		r.Use(s.requireUser)
		r.Get("/reports", s.listReports)
		r.Get("/audit", s.moderationAudit)
		r.With(s.originCheck).Post("/reports/{id}/decisions", s.moderate)
	})
	router.Route("/api/v1/account", func(r chi.Router) {
		r.Use(s.requireUser, s.originCheck)
		r.Post("/export", s.exportAccount)
		r.Delete("/", s.deleteAccount)
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
	var betaValidation *beta.ValidationError
	switch {
	case errors.Is(err, context.Canceled):
		// Browser navigation can cancel a DB query. Keep disconnected clients
		// separate from server faults in operational metrics.
		w.WriteHeader(499)
	case errors.Is(err, context.DeadlineExceeded):
		fail(w, 504, "request_timeout", "Richiesta scaduta. Riprova.", nil)
	case errors.As(err, &betaValidation):
		fail(w, 422, "validation_failed", "Controlla i dati inseriti.", betaValidation.Fields)
	case errors.Is(err, beta.ErrInvite):
		fail(w, 403, "invalid_invite", "Serve un invito valido per questa email. Il codice potrebbe essere scaduto, revocato o già usato.", nil)
	case errors.Is(err, beta.ErrClosed):
		fail(w, 409, "beta_closed", "Ingresso alla beta sospeso o posti esauriti.", nil)
	case errors.Is(err, beta.ErrForbidden):
		fail(w, 403, "moderator_required", "Accesso riservato ai moderatori.", nil)
	case errors.Is(err, safety.ErrLimited):
		w.Header().Set("Retry-After", "60")
		fail(w, 429, "rate_limited", "Troppe richieste. Attendi un minuto.", nil)
	case errors.Is(err, safety.ErrSpam):
		fail(w, 409, "duplicate_post", "Hai già pubblicato questo testo negli ultimi 10 minuti.", nil)
	case errors.Is(err, moderation.ErrForbidden):
		fail(w, 403, "moderator_required", "Accesso riservato ai moderatori.", nil)
	case errors.Is(err, moderation.ErrNotFound):
		fail(w, 404, "not_found", "Segnalazione o contenuto non disponibile.", nil)
	case errors.Is(err, moderation.ErrConflict):
		fail(w, 409, "moderation_conflict", "Stato già cambiato o azione non consentita. Aggiorna la coda.", nil)
	case errors.Is(err, privacy.ErrPassword):
		fail(w, 403, "password_required", "La password attuale non è corretta.", nil)
	case errors.Is(err, privacy.ErrTooLarge):
		fail(w, 413, "export_too_large", "Archivio oltre il limite automatico. Richiedi l’esportazione completa al gestore.", nil)
	case errors.As(err, &validation):
		fail(w, 422, "validation_failed", "Controlla i dati inseriti.", validation.Fields)
	case errors.Is(err, auth.ErrConflict):
		fail(w, 409, "account_exists", "Handle o email già registrati.", nil)
	case errors.Is(err, auth.ErrUnauthorized):
		fail(w, 401, "unauthorized", "Credenziali o sessione non valide.", nil)
	case errors.Is(err, users.ErrNotFound):
		fail(w, 404, "profile_not_found", "Profilo non disponibile.", nil)
	case errors.Is(err, posts.ErrNotFound):
		fail(w, 404, "post_not_found", "Post non disponibile.", nil)
	case errors.Is(err, posts.ErrForbidden):
		fail(w, 403, "post_forbidden", "Puoi modificare o eliminare solo i tuoi post.", nil)
	case errors.Is(err, users.ErrBlocked):
		fail(w, 403, "relationship_blocked", "Questa connessione non è consentita.", nil)
	case errors.Is(err, media.ErrNotFound), errors.Is(err, notifications.ErrNotFound):
		fail(w, 404, "not_found", "Risorsa non disponibile.", nil)
	case errors.Is(err, media.ErrUnsupported):
		fail(w, 415, "unsupported_image", "Sono ammesse soltanto immagini JPEG o PNG valide, con MIME corrispondente.", nil)
	case errors.Is(err, media.ErrTooLarge):
		fail(w, 413, "image_too_large", "Immagine troppo grande (massimo 5 MiB in upload e 8 MiB elaborata).", nil)
	case errors.Is(err, media.ErrAttached):
		fail(w, 409, "image_attached", "L'immagine appartiene già a un post. Puoi eliminare il post.", nil)
	case errors.Is(err, media.ErrStorage):
		fail(w, 503, "storage_unavailable", "Storage non disponibile. Riprova tra poco.", nil)
	case errors.Is(err, auth.ErrInvalidToken):
		fail(w, 400, "invalid_token", "Link non valido, scaduto o già utilizzato. Richiedine uno nuovo.", nil)
	case errors.Is(err, auth.ErrBusy):
		w.Header().Set("Retry-After", "5")
		fail(w, 503, "auth_busy", "Servizio occupato, riprova tra poco.", nil)
	default:
		// Driver errors can contain SQL values, addresses or storage keys.
		slog.Error("request failed", "class", safeErrorClass(err))
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
	if s.accounts != nil {
		if err = s.accounts.RequestVerification(result.User.ID); err != nil {
			slog.Error("verification email could not be queued")
		}
	}
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
	if s.beta != nil {
		if err = s.beta.Activity(r.Context(), user.ID); err != nil {
			s.handleError(w, err)
			return
		}
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
	s.clearCookie(w)
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
		fail(w, 503, "unavailable", "Dipendenze del servizio non disponibili o schema non aggiornato.", nil)
		return
	}
	respond(w, 200, map[string]any{"data": map[string]string{"status": "ok"}})
}
func (s *Server) headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
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
		if currentUser(r).ID != "" && !s.limitAccount(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) recoverJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				slog.Error("request panic")
				fail(w, 500, "internal_error", "Errore del servizio.", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
