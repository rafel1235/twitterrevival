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
	wake      chan struct{}
	started   atomic.Bool
	running   atomic.Bool
	workers   sync.WaitGroup
}

func NewAccountService(repo *PostgresRepository, passwords *Service, sender mailer.Sender, origin string) *AccountService {
	return &AccountService{repo: repo, passwords: passwords, sender: sender, origin: origin, wake: make(chan struct{}, 1)}
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
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			if s.processOne(ctx) {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case <-s.wake:
			case <-ticker.C:
			}
		}
	}()
}
func (s *AccountService) Wait() { s.workers.Wait() }
func (s *AccountService) enqueue(job mailJob) error {
	if !s.running.Load() {
		return ErrBusy
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.repo.enqueueMail(ctx, job); err != nil {
		return err
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return nil
}

func (s *AccountService) processOne(ctx context.Context) bool {
	claim, cancel := context.WithTimeout(ctx, 3*time.Second)
	job, id, attempt, err := s.repo.claimMail(claim)
	cancel()
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("mail queue claim failed")
		}
		return false
	}
	delivery, stop := context.WithTimeout(ctx, 15*time.Second)
	err = s.deliver(delivery, job)
	stop()
	ack, end := context.WithTimeout(context.Background(), 3*time.Second)
	defer end()
	if err == nil || (errors.Is(err, ErrCooldown) && attempt == 1) || errors.Is(err, pgx.ErrNoRows) {
		_, err = s.repo.Pool.Exec(ack, "DELETE FROM mail_jobs WHERE id=$1 AND attempts=$2", id, attempt)
	} else {
		slog.Warn("account email delivery deferred", "purpose", job.Purpose)
		delay := time.Minute * time.Duration(1<<uint(attempt-1))
		_, err = s.repo.Pool.Exec(ack, "UPDATE mail_jobs SET leased_until=NULL,available_at=now()+$2::int*interval '1 second' WHERE id=$1 AND attempts=$3", id, int(delay.Seconds()), attempt)
	}
	if err != nil {
		slog.Warn("mail queue acknowledgement failed")
	}
	return true
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
