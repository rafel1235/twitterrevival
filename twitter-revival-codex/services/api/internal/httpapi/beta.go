package httpapi

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"radura.local/api/internal/beta"
	"radura.local/api/internal/monitoring"
)

func WithBeta(b *beta.Service, m *monitoring.Metrics) Option {
	return func(s *Server) { s.beta = b; s.metrics = m }
}
func (s *Server) betaPublic(w http.ResponseWriter, r *http.Request) {
	open := true
	required := false
	var err error
	if s.beta != nil {
		required = s.beta.RequireInvite
		open, err = s.beta.Public(r.Context())
		if err != nil {
			s.handleError(w, err)
			return
		}
	}
	respond(w, 200, map[string]any{"data": map[string]any{"invite_required": required, "enrollment_open": open}})
}
func (s *Server) betaDashboard(w http.ResponseWriter, r *http.Request) {
	if s.beta == nil {
		fail(w, 503, "unavailable", "Beta non configurata.", nil)
		return
	}
	stats, err := s.beta.Stats(r.Context(), currentUser(r).ID)
	if err != nil {
		s.handleError(w, err)
		return
	}
	var ops any
	if s.metrics != nil {
		snapshot, e := s.metrics.Snapshot(r.Context())
		if e != nil {
			s.handleError(w, e)
			return
		}
		ops = snapshot
	}
	respond(w, 200, map[string]any{"data": map[string]any{"beta": stats, "operations": ops}})
}
func (s *Server) betaConfigure(w http.ResponseWriter, r *http.Request) {
	if s.beta == nil {
		fail(w, 503, "unavailable", "Beta non configurata.", nil)
		return
	}
	var in struct {
		Capacity       int  `json:"capacity"`
		EnrollmentOpen bool `json:"enrollment_open"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.beta.Configure(r.Context(), currentUser(r).ID, in.Capacity, in.EnrollmentOpen); err != nil {
		s.handleError(w, err)
		return
	}
	w.WriteHeader(204)
}
func (s *Server) betaInvites(w http.ResponseWriter, r *http.Request) {
	if s.beta == nil {
		fail(w, 503, "unavailable", "Inviti non configurati.", nil)
		return
	}
	actor := currentUser(r).ID
	if r.Method == "POST" {
		var in beta.Input
		if !decode(w, r, &in) {
			return
		}
		out, err := s.beta.Issue(r.Context(), actor, "pilot", in)
		if err != nil {
			s.handleError(w, err)
			return
		}
		respond(w, 201, map[string]any{"data": out})
		return
	}
	if r.Method == "DELETE" {
		var input struct{}
		if !decode(w, r, &input) {
			return
		}
		if err := s.beta.Revoke(r.Context(), actor, chi.URLParam(r, "id")); err != nil {
			s.handleError(w, err)
			return
		}
		w.WriteHeader(204)
		return
	}
	items, err := s.beta.List(r.Context(), actor)
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": map[string]any{"items": items}})
}
