package httpapi

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/users"
)

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	p, err := users.ParsePagination(r.URL.Query().Get("cursor"), r.URL.Query().Get("limit"))
	if err != nil {
		s.handleError(w, err)
		return
	}
	kind := r.URL.Query().Get("type")
	term := r.URL.Query().Get("q")
	switch kind {
	case "", "posts":
		if !s.postsReady(w) {
			return
		}
		page, err := s.posts.Search(r.Context(), currentUser(r).ID, term, p)
		if err != nil {
			s.handleError(w, err)
			return
		}
		respond(w, 200, map[string]any{"data": page})
	case "users":
		if !s.usersReady(w) {
			return
		}
		page, err := s.users.Search(r.Context(), currentUser(r).ID, term, p)
		if err != nil {
			s.handleError(w, err)
			return
		}
		respond(w, 200, map[string]any{"data": page})
	default:
		s.handleError(w, &auth.ValidationError{Fields: map[string]string{"type": "Scegli posts oppure users."}})
	}
}
func (s *Server) notificationsReady(w http.ResponseWriter) bool {
	if s.notifications == nil {
		fail(w, 503, "unavailable", "Notifiche non disponibili.", nil)
		return false
	}
	return true
}
func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	if !s.notificationsReady(w) {
		return
	}
	p, err := users.ParsePagination(r.URL.Query().Get("cursor"), r.URL.Query().Get("limit"))
	if err != nil {
		s.handleError(w, err)
		return
	}
	page, err := s.notifications.List(r.Context(), currentUser(r).ID, p)
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": page})
}
func (s *Server) unreadNotifications(w http.ResponseWriter, r *http.Request) {
	if !s.notificationsReady(w) {
		return
	}
	count, err := s.notifications.Unread(r.Context(), currentUser(r).ID)
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": map[string]any{"unread_count": count}})
}
func (s *Server) readNotifications(w http.ResponseWriter, r *http.Request) {
	if !s.notificationsReady(w) {
		return
	}
	var input struct{}
	if !decode(w, r, &input) {
		return
	}
	if err := s.notifications.Read(r.Context(), currentUser(r).ID, chi.URLParam(r, "id")); err != nil {
		s.handleError(w, err)
		return
	}
	w.WriteHeader(204)
}
