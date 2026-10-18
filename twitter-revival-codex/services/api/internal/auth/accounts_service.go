package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"radura.local/api/internal/mailer"
)

type mailJob struct{ UserID, Email, Purpose string }
type AccountService struct {
	repo      *PostgresRepository
	passwords *Service
	sender    mailer.Sender
	origin    string
	jobs      chan mailJob
	started   atomic.Bool
	running   atomic.Bool
	workers   sync.WaitGroup
}

func NewAccountService(repo *PostgresRepository, passwords *Service, sender mailer.Sender, origin string) *AccountService {
	return &AccountService{repo: repo, passwords: passwords, sender: sender, origin: origin, jobs: make(chan mailJob, 32)}
}
func (s *AccountService) Start(ctx context.Context) {
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	s.running.Store(true)
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		defer s.running.Store(false)
		for {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case job := <-s.jobs:
				delivery, cancel := context.WithTimeout(ctx, 15*time.Second)
				err := s.deliver(delivery, job)
				cancel()
				if err != nil && !errors.Is(err, ErrCooldown) && !errors.Is(err, pgx.ErrNoRows) && !errors.Is(err, context.Canceled) {
					slog.Error("account email delivery failed", "purpose", job.Purpose)
				}
			}
		}
	}()
}
func (s *AccountService) Wait() { s.workers.Wait() }
func (s *AccountService) enqueue(job mailJob) error {
	if !s.running.Load() {
		return ErrBusy
	}
	select {
	case s.jobs <- job:
		return nil
	default:
		return ErrBusy
	}
}
func (s *AccountService) RequestVerification(userID string) error {
	return s.enqueue(mailJob{UserID: userID, Purpose: VerificationPurpose})
}
func (s *AccountService) RequestReset(email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) {
		return &ValidationError{Fields: map[string]string{"email": "Inserisci un indirizzo email valido."}}
	}
	// No account lookup on the request path: unknown/known addresses get the same response and work.
	return s.enqueue(mailJob{Email: email, Purpose: ResetPurpose})
}
func (s *AccountService) deliver(ctx context.Context, job mailJob) error {
	account, err := s.repo.deliveryAccount(ctx, job.UserID, job.Email)
	if err != nil {
		return err
	}
	if job.Purpose == VerificationPurpose && account.EmailVerified {
		return nil
	}
	token, hash, err := NewToken()
	if err != nil {
		return err
	}
	duration := 24 * time.Hour
	path := "/verify-email"
	subject := "Conferma la tua email - Radura"
	intro := "Conferma il tuo indirizzo email aprendo questo link. Il link scade tra 24 ore."
	if job.Purpose == ResetPurpose {
		duration = time.Hour
		path = "/reset-password"
		subject = "Recupero password - Radura"
		intro = "Hai richiesto una nuova password. Apri questo link entro un'ora. Se non sei stato tu, ignora questo messaggio."
	}
	if err = s.repo.issueAccountToken(ctx, account.ID, job.Purpose, hash, time.Now().UTC().Add(duration)); err != nil {
		return err
	}
	link := s.origin + path + "?token=" + url.QueryEscape(token)
	return s.sender.Send(ctx, mailer.Message{To: account.Email, Subject: subject, Body: intro + "\n\n" + link + "\n\nIl link puo' essere usato una sola volta. Radura e' un progetto indipendente."})
}
func (s *AccountService) VerifyEmail(ctx context.Context, token string) error {
	if !ValidToken(token) {
		return ErrInvalidToken
	}
	return s.repo.consumeAccountToken(ctx, TokenHash(token), VerificationPurpose, "")
}
func (s *AccountService) ResetPassword(ctx context.Context, token, password string) error {
	if !ValidToken(token) {
		return ErrInvalidToken
	}
	if !validPassword(password) {
		return &ValidationError{Fields: map[string]string{"password": "Usa almeno 12 caratteri e al massimo 128 byte."}}
	}
	hash := TokenHash(token)
	if _, err := s.repo.accountTokenUser(ctx, hash, ResetPurpose); err != nil {
		return err
	}
	if !s.passwords.acquire() {
		return ErrBusy
	}
	defer s.passwords.release()
	encoded, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.repo.consumeAccountToken(ctx, hash, ResetPurpose, encoded)
}
