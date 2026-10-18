package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"radura.local/api/internal/communities"
	"radura.local/api/internal/users"
)

func (s *Server) communitiesReady(w http.ResponseWriter) bool {
	if s.communities == nil {
		fail(w, 503, "unavailable", "Servizio community non disponibile.", nil)
		return false
	}
	return true
}

func (s *Server) listCommunities(w http.ResponseWriter, r *http.Request) {
	if !s.communitiesReady(w) {
		return
	}
	mine := r.URL.Query().Get("mine")
	if mine != "" && mine != "0" && mine != "1" {
		fail(w, 422, "validation_failed", "Controlla i dati inseriti.", map[string]string{"mine": "Usa mine=1 per le tue community."})
		return
	}
	p, err := users.ParsePagination(r.URL.Query().Get("cursor"), r.URL.Query().Get("limit"))
	if err != nil {
		s.handleError(w, err)
		return
	}
	page, err := s.communities.List(r.Context(), currentUser(r).ID, mine == "1", p)
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": page})
}

func (s *Server) getCommunity(w http.ResponseWriter, r *http.Request) {
	if !s.communitiesReady(w) {
		return
	}
	community, err := s.communities.Get(r.Context(), currentUser(r).ID, chi.URLParam(r, "id"))
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": map[string]any{"community": community}})
}

func (s *Server) createCommunity(w http.ResponseWriter, r *http.Request) {
	if !s.communitiesReady(w) {
		return
	}
	var input communities.Input
	if !decode(w, r, &input) {
		return
	}
	community, err := s.communities.Create(r.Context(), currentUser(r).ID, input)
	if err != nil {
		s.handleError(w, err)
		return
	}
	w.Header().Set("Location", "/api/v1/communities/"+community.ID)
	respond(w, 201, map[string]any{"data": map[string]any{"community": community}})
}

func (s *Server) updateCommunity(w http.ResponseWriter, r *http.Request) {
	if !s.communitiesReady(w) {
		return
	}
	var input communities.UpdateInput
	if !decode(w, r, &input) {
		return
	}
	community, err := s.communities.Update(r.Context(), currentUser(r).ID, chi.URLParam(r, "id"), input)
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": map[string]any{"community": community}})
}

func (s *Server) archiveCommunity(w http.ResponseWriter, r *http.Request) {
	if !s.communitiesReady(w) {
		return
	}
	var input struct{}
	if !decode(w, r, &input) {
		return
	}
	if err := s.communities.Archive(r.Context(), currentUser(r).ID, chi.URLParam(r, "id")); err != nil {
		s.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) communityMembership(w http.ResponseWriter, r *http.Request) {
	if !s.communitiesReady(w) {
		return
	}
	var input struct{}
	if !decode(w, r, &input) {
		return
	}
	if err := s.communities.Membership(r.Context(), currentUser(r).ID, chi.URLParam(r, "id"), r.Method == http.MethodPost); err != nil {
		s.handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) communityMembers(w http.ResponseWriter, r *http.Request) {
	if !s.communitiesReady(w) {
		return
	}
	p, err := users.ParsePagination(r.URL.Query().Get("cursor"), r.URL.Query().Get("limit"))
	if err != nil {
		s.handleError(w, err)
		return
	}
	page, err := s.communities.Members(r.Context(), currentUser(r).ID, chi.URLParam(r, "id"), p)
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": page})
}
