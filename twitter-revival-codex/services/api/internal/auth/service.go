package auth

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var handlePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,20}$`)

type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "invalid input" }

type RegisterInput struct {
	Handle      string `json:"handle"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type Result struct {
	User      User
	Token     string
	ExpiresAt time.Time
}

type Service struct {
	repo      Repository
	ttl       time.Duration
	dummyHash string
	workers   chan struct{}
}

func NewService(repo Repository, ttl time.Duration) (*Service, error) {
	// Equal-cost verification for unknown and suspended accounts.
	token, _, err := NewToken()
	if err != nil {
		return nil, err
	}
	dummy, err := HashPassword(token)
	if err != nil {
		return nil, err
	}
	return &Service{repo: repo, ttl: ttl, dummyHash: dummy, workers: make(chan struct{}, 4)}, nil
}

func validEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && len(email) <= 254 && !strings.ContainsAny(email, "\r\n")
}
func validPassword(password string) bool {
	n := utf8.RuneCountInString(password)
	return utf8.ValidString(password) && n >= 12 && len(password) <= 128
}

func (s *Service) acquire() bool {
	select {
	case s.workers <- struct{}{}:
		return true
	default:
		return false
	}
}
func (s *Service) release() { <-s.workers }

func (s *Service) Register(ctx context.Context, input RegisterInput) (Result, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Handle = strings.ToLower(strings.TrimSpace(input.Handle))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	fields := map[string]string{}
	if !handlePattern.MatchString(input.Handle) {
		fields["handle"] = "Usa 3–20 lettere ASCII, cifre o underscore."
	}
	if !validEmail(input.Email) {
		fields["email"] = "Inserisci un indirizzo email valido (massimo 254 byte)."
	}
	if !validPassword(input.Password) {
		fields["password"] = "Usa almeno 12 caratteri e al massimo 128 byte."
	}
	if !utf8.ValidString(input.DisplayName) || utf8.RuneCountInString(input.DisplayName) < 1 || utf8.RuneCountInString(input.DisplayName) > 50 || strings.ContainsFunc(input.DisplayName, unicode.IsControl) {
		fields["display_name"] = "Usa un nome da 1 a 50 caratteri, senza caratteri di controllo."
	}
	if len(fields) > 0 {
		return Result{}, &ValidationError{fields}
	}
	if !s.acquire() {
		return Result{}, ErrBusy
	}
	defer s.release()
	hash, err := HashPassword(input.Password)
	if err != nil {
		return Result{}, err
	}
	result, session, err := s.newSession()
	if err != nil {
		return Result{}, err
	}
	result.User, err = s.repo.Register(ctx, NewUser{input.Handle, input.Email, input.DisplayName, hash}, session)
	return result, err
}

func (s *Service) Login(ctx context.Context, input LoginInput) (Result, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !validEmail(input.Email) || !validPassword(input.Password) {
		return Result{}, &ValidationError{map[string]string{"credentials": "Inserisci email e password valide (12 caratteri minimi, 128 byte massimi)."}}
	}
	if !s.acquire() {
		return Result{}, ErrBusy
	}
	defer s.release()
	account, err := s.repo.FindAccount(ctx, input.Email)
	if err != nil && !errors.Is(err, ErrUnauthorized) {
		return Result{}, err
	}
	hash := s.dummyHash
	if err == nil && account.Status == "active" {
		hash = account.PasswordHash
	}
	match := VerifyPassword(hash, input.Password)
	if !match || err != nil || account.Status != "active" {
		return Result{}, ErrUnauthorized
	}
	result, session, err := s.newSession()
	if err != nil {
		return Result{}, err
	}
	if err = s.repo.CreateSession(ctx, account.User.ID, session, account.PasswordHash); err != nil {
		return Result{}, err
	}
	result.User = account.User
	return result, nil
}

func (s *Service) newSession() (Result, NewSession, error) {
	token, hash, err := NewToken()
	expiry := time.Now().UTC().Add(s.ttl)
	return Result{Token: token, ExpiresAt: expiry}, NewSession{hash, expiry}, err
}

func (s *Service) Me(ctx context.Context, token string) (User, error) {
	if !ValidToken(token) {
		return User{}, ErrUnauthorized
	}
	return s.repo.SessionUser(ctx, TokenHash(token))
}
func (s *Service) Logout(ctx context.Context, token string) error {
	if !ValidToken(token) {
		return nil
	}
	return s.repo.RevokeSession(ctx, TokenHash(token))
}

// ConfirmPassword returns an expected hash for a subsequent transaction to
// compare under its account lock, fencing concurrent password reset/erasure.
func (s *Service) ConfirmPassword(ctx context.Context, user User, password string) (string, error) {
	if !validPassword(password) {
		return "", ErrUnauthorized
	}
	if !s.acquire() {
		return "", ErrBusy
	}
	defer s.release()
	account, err := s.repo.FindAccount(ctx, user.Email)
	if err != nil {
		return "", err
	}
	if !VerifyPassword(account.PasswordHash, password) || account.Status != "active" || account.User.ID != user.ID {
		return "", ErrUnauthorized
	}
	return account.PasswordHash, nil
}
