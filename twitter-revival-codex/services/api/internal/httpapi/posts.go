package httpapi

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"radura.local/api/internal/posts"
	"radura.local/api/internal/users"
)

func (s *Server) postsReady(w http.ResponseWriter) bool {
	if s.posts == nil {
		fail(w, 503, "unavailable", "Servizio post non disponibile.", nil)
		return false
	}
	return true
}
func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	if !s.postsReady(w) {
		return
	}
	var input posts.CreateInput
	if !decode(w, r, &input) {
		return
	}
	post, err := s.posts.Create(r.Context(), currentUser(r).ID, input)
	if err != nil {
		s.handleError(w, err)
		return
	}
	w.Header().Set("Location", "/api/v1/posts/"+post.ID)
	respond(w, 201, map[string]any{"data": map[string]any{"post": post}})
}
func (s *Server) getPost(w http.ResponseWriter, r *http.Request) {
	if !s.postsReady(w) {
		return
	}
	post, err := s.posts.Get(r.Context(), currentUser(r).ID, chi.URLParam(r, "id"))
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": map[string]any{"post": post}})
}
func (s *Server) mutatePost(w http.ResponseWriter, r *http.Request, action string) {
	if !s.postsReady(w) {
		return
	}
	var body string
	if action == "edit" {
		var input posts.EditInput
		if !decode(w, r, &input) {
			return
		}
		body = input.Body
	} else {
		var input struct{}
		if !decode(w, r, &input) {
			return
		}
	}
	post, err := s.posts.Mutate(r.Context(), currentUser(r).ID, chi.URLParam(r, "id"), action, body)
	if err != nil {
		s.handleError(w, err)
		return
	}
	if action == "delete" {
		w.WriteHeader(204)
		return
	}
	respond(w, 200, map[string]any{"data": map[string]any{"post": post}})
}
func (s *Server) postList(w http.ResponseWriter, r *http.Request, kind string) {
	if !s.postsReady(w) {
		return
	}
	p, err := users.ParsePagination(r.URL.Query().Get("cursor"), r.URL.Query().Get("limit"))
	if err != nil {
		s.handleError(w, err)
		return
	}
	page, err := s.posts.List(r.Context(), currentUser(r).ID, kind, chi.URLParam(r, "id"), p)
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": page})
}
