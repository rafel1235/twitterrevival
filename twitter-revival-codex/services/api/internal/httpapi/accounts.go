package httpapi

import (
	"net/http"
	"time"
)

func (s *Server) accountsReady(w http.ResponseWriter) bool {
	if s.accounts == nil {
		fail(w, 503, "unavailable", "Servizio email non disponibile.", nil)
		return false
	}
	return true
}
func (s *Server) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: s.cookieName, Value: "", Path: "/", HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}
func (s *Server) requestVerification(w http.ResponseWriter, r *http.Request) {
	if !s.accountsReady(w) {
		return
	}
	var input struct{}
	if !decode(w, r, &input) {
		return
	}
	if err := s.accounts.RequestVerification(currentUser(r).ID); err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 202, map[string]any{"data": map[string]string{"message": "Richiesta ricevuta. Se necessario, riceverai un link per verificare l'email. Controlla anche la posta indesiderata."}})
}
func (s *Server) confirmVerification(w http.ResponseWriter, r *http.Request) {
	if !s.accountsReady(w) {
		return
	}
	var input struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &input) {
		return
	}
	if err := s.accounts.VerifyEmail(r.Context(), input.Token); err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 200, map[string]any{"data": map[string]string{"message": "Email verificata."}})
}
func (s *Server) forgotPassword(w http.ResponseWriter, r *http.Request) {
	if !s.accountsReady(w) {
		return
	}
	var input struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &input) {
		return
	}
	if err := s.accounts.RequestReset(input.Email); err != nil {
		s.handleError(w, err)
		return
	}
	respond(w, 202, map[string]any{"data": map[string]string{"message": "Se l'indirizzo è associato a un account attivo, riceverai un link per reimpostare la password. Controlla anche la posta indesiderata."}})
}
func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	if !s.accountsReady(w) {
		return
	}
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	if err := s.accounts.ResetPassword(r.Context(), input.Token, input.Password); err != nil {
		s.handleError(w, err)
		return
	}
	s.clearCookie(w)
	respond(w, 200, map[string]any{"data": map[string]string{"message": "Password aggiornata. Accedi nuovamente con la nuova password."}})
}
