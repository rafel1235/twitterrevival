package httpapi

import (
	"errors"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/media"
	"strings"
)

func (s *Server) mediaReady(w http.ResponseWriter) bool {
	if s.media == nil {
		fail(w, 503, "unavailable", "Storage non disponibile.", nil)
		return false
	}
	return true
}
func (s *Server) draftMedia(w http.ResponseWriter, r *http.Request) {
	if !s.mediaReady(w) {
		return
	}
	items, err := s.media.Drafts(r.Context(), currentUser(r).ID)
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": map[string]any{"media": items}})
}
func (s *Server) uploadMedia(w http.ResponseWriter, r *http.Request) {
	if !s.mediaReady(w) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, media.MaxUploadBytes+16384)
	reader, err := r.MultipartReader()
	if err != nil {
		fail(w, 415, "unsupported_media_type", "Usa multipart/form-data con file e alt_text.", nil)
		return
	}
	var data []byte
	var alt, claimed string
	fileSeen, altSeen := false, false
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.uploadReadError(w, err)
			return
		}
		switch part.FormName() {
		case "file":
			if fileSeen {
				fail(w, 400, "invalid_upload", "Invia una sola immagine per richiesta.", nil)
				return
			}
			fileSeen = true
			claimed = part.Header.Get("Content-Type")
			data, err = io.ReadAll(io.LimitReader(part, media.MaxUploadBytes+1))
			if err != nil {
				s.uploadReadError(w, err)
				return
			}
			if len(data) > media.MaxUploadBytes {
				s.handleError(w, media.ErrTooLarge)
				return
			}
		case "alt_text":
			if altSeen {
				fail(w, 400, "invalid_upload", "Descrizione duplicata.", nil)
				return
			}
			altSeen = true
			value, err := io.ReadAll(io.LimitReader(part, 1001))
			if err != nil {
				s.uploadReadError(w, err)
				return
			}
			if len(value) > 1000 {
				s.handleError(w, &auth.ValidationError{Fields: map[string]string{"alt_text": "Descrizione troppo lunga."}})
				return
			}
			alt = string(value)
		default:
			fail(w, 400, "invalid_upload", "Campi ammessi: file e alt_text.", nil)
			return
		}
		_ = part.Close()
	}
	if !fileSeen {
		s.handleError(w, &auth.ValidationError{Fields: map[string]string{"image": "Seleziona un'immagine."}})
		return
	}
	item, err := s.media.Upload(r.Context(), currentUser(r).ID, claimed, alt, data)
	if err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 201, map[string]any{"data": map[string]any{"media": item}})
}
func (s *Server) uploadReadError(w http.ResponseWriter, err error) {
	var large *http.MaxBytesError
	if errors.As(err, &large) {
		s.handleError(w, media.ErrTooLarge)
	} else {
		fail(w, 400, "invalid_upload", "Upload incompleto o non valido.", nil)
	}
}
func (s *Server) getMedia(w http.ResponseWriter, r *http.Request) {
	if !s.mediaReady(w) {
		return
	}
	content := strings.HasSuffix(r.URL.Path, "/content")
	item, data, err := s.media.Read(r.Context(), currentUser(r).ID, chi.URLParam(r, "id"), content)
	if err != nil {
		s.handleError(w, err)
		return
	}
	if !content {
		respond(w, 200, map[string]any{"data": map[string]any{"media": item}})
		return
	}
	w.Header().Set("Content-Type", item.MimeType)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Content-Disposition", "inline; filename=\"image"+map[string]string{"image/png": ".png", "image/jpeg": ".jpg"}[item.MimeType]+"\"")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
func (s *Server) deleteMedia(w http.ResponseWriter, r *http.Request) {
	if !s.mediaReady(w) {
		return
	}
	var input struct{}
	if !decode(w, r, &input) {
		return
	}
	if err := s.media.Delete(r.Context(), currentUser(r).ID, chi.URLParam(r, "id")); err != nil {
		s.handleError(w, err)
		return
	}
	w.WriteHeader(204)
}
