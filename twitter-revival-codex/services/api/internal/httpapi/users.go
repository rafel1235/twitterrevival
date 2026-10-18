package httpapi

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/users"
)

type userContextKey struct{}

func (s *Server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := s.auth.Me(r.Context(), s.token(r))
		if err != nil {
			s.handleError(w, err)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), userContextKey{}, user))
		if s.beta != nil {
			if err = s.beta.Activity(r.Context(), user.ID); err != nil {
				s.handleError(w, err)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func currentUser(r *http.Request) auth.User {
	user, _ := r.Context().Value(userContextKey{}).(auth.User)
	return user
}
func (s *Server) usersReady(w http.ResponseWriter) bool {
	if s.users == nil {
		fail(w, 503, "unavailable", "Servizio profili non disponibile.", nil)
		return false
	}
	return true
}
func (s *Server) profile(w http.ResponseWriter, r *http.Request) {
	if !s.usersReady(w) {
		return
	}
	profile, err := s.users.Profile(r.Context(), currentUser(r).ID, chi.URLParam(r, "handle"))
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": map[string]any{"user": profile}})
}
func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) {
	if !s.usersReady(w) {
		return
	}
	var input users.UpdateInput
	if !decode(w, r, &input) {
		return
	}
	actor := currentUser(r)
	profile, err := s.users.Update(r.Context(), actor.ID, actor.Handle, input)
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": map[string]any{"user": profile}})
}
func (s *Server) relationship(w http.ResponseWriter, r *http.Request, action string) {
	if !s.usersReady(w) {
		return
	}
	var input struct{}
	if !decode(w, r, &input) {
		return
	}
	if err := s.users.Mutate(r.Context(), currentUser(r).ID, chi.URLParam(r, "id"), action); err != nil {
		s.handleError(w, err)
		return
	}
	w.WriteHeader(204)
}
func (s *Server) connections(w http.ResponseWriter, r *http.Request, kind string) {
	if !s.usersReady(w) {
		return
	}
	p, err := users.ParsePagination(r.URL.Query().Get("cursor"), r.URL.Query().Get("limit"))
	if err != nil {
		s.handleError(w, err)
		return
	}
	page, err := s.users.Connections(r.Context(), currentUser(r).ID, chi.URLParam(r, "id"), kind, p)
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": page})
}
func (s *Server) blockedUsers(w http.ResponseWriter, r *http.Request) { s.connections(w, r, "blocks") }
