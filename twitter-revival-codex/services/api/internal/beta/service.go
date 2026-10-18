package beta

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

var ErrInvite = errors.New("invalid invitation")
var ErrClosed = errors.New("beta enrollment unavailable")
var ErrForbidden = errors.New("beta moderator required")

type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string  { return "invalid beta input" }
func invalid(field, message string) error { return &ValidationError{map[string]string{field: message}} }

type Service struct {
	Pool             *pgxpool.Pool
	RequireInvite    bool
	AllowTestCohorts bool
}
type Input struct {
	Email string `json:"email"`
	Days  int    `json:"days"`
}
type Invitation struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
type Issued struct {
	Invitation
	Token string `json:"token"`
}

func tokenHash(token string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 || len(token) != 43 {
		return nil, ErrInvite
	}
	sum := sha256.Sum256([]byte(token))
	return sum[:], nil
}
func validEmail(email string) bool {
	a, e := mail.ParseAddress(email)
	return e == nil && a.Address == email && len(email) <= 254 && !strings.ContainsAny(email, "\r\n")
}
func moderator(ctx context.Context, tx pgx.Tx, actor string) error {
	var role, status string
	if err := tx.QueryRow(ctx, "SELECT role,status FROM users WHERE id=$1 FOR SHARE", actor).Scan(&role, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrForbidden
		}
		return err
	}
	if role != "moderator" || status != "active" {
		return ErrForbidden
	}
	return nil
}
func (s *Service) Issue(ctx context.Context, actor, cohort string, in Input) (Issued, error) {
	return s.issue(ctx, actor, cohort, in, false)
}
func (s *Service) IssueLocal(ctx context.Context, cohort string, test bool, in Input) (Issued, error) {
	if test {
		if !s.AllowTestCohorts || !regexp.MustCompile(`^(e2e|load)-[a-z0-9-]{1,48}$`).MatchString(cohort) {
			return Issued{}, ErrForbidden
		}
		if _, err := s.Pool.Exec(ctx, "INSERT INTO beta_cohorts(slug,kind,capacity) VALUES($1,'test',1000) ON CONFLICT DO NOTHING", cohort); err != nil {
			return Issued{}, err
		}
	} else if cohort != "pilot" {
		return Issued{}, ErrForbidden
	}
	return s.issue(ctx, "", cohort, in, true)
}
func (s *Service) issue(ctx context.Context, actor, cohort string, in Input, operator bool) (Issued, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !validEmail(in.Email) {
		return Issued{}, invalid("email", "Inserisci un indirizzo email valido.")
	}
	if in.Days == 0 {
		in.Days = 7
	}
	if in.Days < 1 || in.Days > 14 {
		return Issued{}, invalid("days", "Usa una scadenza fra 1 e 14 giorni.")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Issued{}, err
	}
	defer tx.Rollback(ctx)
	if !operator {
		if err = moderator(ctx, tx, actor); err != nil {
			return Issued{}, err
		}
	}
	var capacity, members, reserved int
	var open bool
	var kind string
	if err = tx.QueryRow(ctx, "SELECT capacity,enrollment_open,kind FROM beta_cohorts WHERE slug=$1 FOR UPDATE", cohort).Scan(&capacity, &open, &kind); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Issued{}, ErrClosed
		}
		return Issued{}, err
	}
	if !open || (kind == "test" && !s.AllowTestCohorts) {
		return Issued{}, ErrClosed
	}
	if err = tx.QueryRow(ctx, "SELECT (SELECT count(*) FROM beta_members WHERE cohort_slug=$1),(SELECT count(*) FROM beta_invites WHERE cohort_slug=$1 AND used_at IS NULL AND revoked_at IS NULL AND expires_at>now())", cohort).Scan(&members, &reserved); err != nil {
		return Issued{}, err
	}
	if members+reserved >= capacity {
		return Issued{}, ErrClosed
	}
	var duplicate bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email=$1) OR EXISTS(SELECT 1 FROM beta_invites WHERE email=$1 AND cohort_slug=$2 AND used_at IS NULL AND revoked_at IS NULL AND expires_at>now())", in.Email, cohort).Scan(&duplicate); err != nil {
		return Issued{}, err
	}
	if duplicate {
		return Issued{}, invalid("email", "Account già presente o invito attivo per questa email.")
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return Issued{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash, _ := tokenHash(token)
	var issuer any
	if actor != "" {
		issuer = actor
	}
	out := Issued{Token: token}
	err = tx.QueryRow(ctx, "INSERT INTO beta_invites(cohort_slug,email,token_hash,issuer_id,expires_at) VALUES($1,$2,$3,$4,now()+$5::int*interval '1 day') RETURNING id,email,created_at,expires_at", cohort, in.Email, hash, issuer, in.Days).Scan(&out.ID, &out.Email, &out.CreatedAt, &out.ExpiresAt)
	if err != nil {
		return Issued{}, err
	}
	out.Status = "available"
	if _, err = tx.Exec(ctx, "INSERT INTO beta_invite_events(invite_id,actor_id,action) VALUES($1,$2,'issued')", out.ID, issuer); err != nil {
		return Issued{}, err
	}
	return out, tx.Commit(ctx)
}

// Preflight avoids spending Argon2 workers on a missing/mismatched token. The
// transaction below repeats all checks under locks and consumes it atomically.
func (s *Service) Check(ctx context.Context, token, email string) error {
	if !s.RequireInvite {
		return nil
	}
	hash, err := tokenHash(token)
	if err != nil {
		return err
	}
	var ok bool
	err = s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM beta_invites i JOIN beta_cohorts c ON c.slug=i.cohort_slug WHERE i.token_hash=$1 AND i.email=$2 AND i.used_at IS NULL AND i.revoked_at IS NULL AND i.expires_at>now() AND c.enrollment_open AND (c.kind='pilot' OR $3))`, hash, email, s.AllowTestCohorts).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvite
	}
	return nil
}
func (s *Service) Reserve(ctx context.Context, tx pgx.Tx, token, email string) (string, string, error) {
	if !s.RequireInvite {
		return "", "", nil
	}
	hash, err := tokenHash(token)
	if err != nil {
		return "", "", err
	}
	var id, cohort string
	if err = tx.QueryRow(ctx, "SELECT id,cohort_slug FROM beta_invites WHERE token_hash=$1", hash).Scan(&id, &cohort); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrInvite
		}
		return "", "", err
	}
	var open bool
	var kind string
	var capacity int
	if err = tx.QueryRow(ctx, "SELECT enrollment_open,capacity,kind FROM beta_cohorts WHERE slug=$1 FOR UPDATE", cohort).Scan(&open, &capacity, &kind); err != nil {
		return "", "", err
	}
	if !open || (kind == "test" && !s.AllowTestCohorts) {
		return "", "", ErrClosed
	}
	var valid bool
	err = tx.QueryRow(ctx, "SELECT email=$2 AND used_at IS NULL AND revoked_at IS NULL AND expires_at>now() FROM beta_invites WHERE id=$1 FOR UPDATE", id, email).Scan(&valid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrInvite
		}
		return "", "", err
	}
	if !valid {
		return "", "", ErrInvite
	}
	var members int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM beta_members WHERE cohort_slug=$1", cohort).Scan(&members); err != nil {
		return "", "", err
	}
	if members >= capacity {
		return "", "", ErrClosed
	}
	return id, cohort, nil
}
func (s *Service) Admit(ctx context.Context, tx pgx.Tx, id, cohort, user string) error {
	if id == "" {
		return nil
	}
	if _, err := tx.Exec(ctx, "UPDATE beta_invites SET used_at=now(),used_by=$2 WHERE id=$1", id, user); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO beta_members(user_id,cohort_slug) VALUES($1,$2)", user, cohort); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "INSERT INTO beta_invite_events(invite_id,action) VALUES($1,'used')", id)
	return err
}
func (s *Service) Revoke(ctx context.Context, actor, id string) error {
	var uuid pgtype.UUID
	if len(id) != 36 || uuid.Scan(id) != nil {
		return ErrInvite
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = moderator(ctx, tx, actor); err != nil {
		return err
	}
	var cohort string
	if err = tx.QueryRow(ctx, "SELECT cohort_slug FROM beta_invites WHERE id=$1", id).Scan(&cohort); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvite
		}
		return err
	}
	if _, err = tx.Exec(ctx, "SELECT slug FROM beta_cohorts WHERE slug=$1 FOR UPDATE", cohort); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, "UPDATE beta_invites SET revoked_at=now() WHERE id=$1 AND used_at IS NULL AND revoked_at IS NULL", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		if _, err = tx.Exec(ctx, "INSERT INTO beta_invite_events(invite_id,actor_id,action) VALUES($1,$2,'revoked')", id, actor); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

type Stats struct {
	Capacity         int      `json:"capacity"`
	EnrollmentOpen   bool     `json:"enrollment_open"`
	Members          int      `json:"members"`
	Reserved         int      `json:"reserved"`
	Verified         int      `json:"verified"`
	Activated        int      `json:"activated"`
	FirstFollow      int      `json:"first_follow"`
	FirstPost        int      `json:"first_post"`
	FirstInteraction int      `json:"first_interaction"`
	D7Eligible       int      `json:"d7_eligible"`
	D7Returned       int      `json:"d7_returned"`
	D7Rate           *float64 `json:"d7_rate"`
}

func (s *Service) Stats(ctx context.Context, actor string) (Stats, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return Stats{}, err
	}
	defer tx.Rollback(ctx)
	if err = moderator(ctx, tx, actor); err != nil {
		return Stats{}, err
	}
	out := Stats{}
	err = tx.QueryRow(ctx, "SELECT capacity,enrollment_open,(SELECT count(*) FROM beta_invites WHERE cohort_slug='pilot' AND used_at IS NULL AND revoked_at IS NULL AND expires_at>now()) FROM beta_cohorts WHERE slug='pilot'").Scan(&out.Capacity, &out.EnrollmentOpen, &out.Reserved)
	if err != nil {
		return out, err
	}
	err = tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE u.email_verified_at IS NOT NULL),count(*) FILTER(WHERE u.email_verified_at IS NOT NULL AND m.first_follow_at IS NOT NULL AND m.first_post_at IS NOT NULL),count(m.first_follow_at),count(m.first_post_at),count(m.first_interaction_at),count(*) FILTER(WHERE m.joined_at<=now()-interval '8 days'),count(*) FILTER(WHERE m.joined_at<=now()-interval '8 days' AND m.returned_d7_at IS NOT NULL) FROM beta_members m JOIN users u ON u.id=m.user_id WHERE m.cohort_slug='pilot' AND u.status<>'deleted'`).Scan(&out.Members, &out.Verified, &out.Activated, &out.FirstFollow, &out.FirstPost, &out.FirstInteraction, &out.D7Eligible, &out.D7Returned)
	if err != nil {
		return out, err
	}
	if out.D7Eligible > 0 {
		rate := float64(out.D7Returned) / float64(out.D7Eligible)
		out.D7Rate = &rate
	}
	return out, tx.Commit(ctx)
}
func (s *Service) Configure(ctx context.Context, actor string, capacity int, open bool) error {
	if capacity < 30 || capacity > 50 {
		return invalid("capacity", "La beta ammette fra 30 e 50 piloti.")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = moderator(ctx, tx, actor); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "SELECT slug FROM beta_cohorts WHERE slug='pilot' FOR UPDATE"); err != nil {
		return err
	}
	var occupied int
	if err = tx.QueryRow(ctx, "SELECT (SELECT count(*) FROM beta_members WHERE cohort_slug='pilot')+(SELECT count(*) FROM beta_invites WHERE cohort_slug='pilot' AND used_at IS NULL AND revoked_at IS NULL AND expires_at>now())").Scan(&occupied); err != nil {
		return err
	}
	if occupied > capacity {
		return ErrClosed
	}
	if _, err = tx.Exec(ctx, "UPDATE beta_cohorts SET capacity=$1,enrollment_open=$2 WHERE slug='pilot'", capacity, open); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Service) List(ctx context.Context, actor string) ([]Invitation, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = moderator(ctx, tx, actor); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id,email,CASE WHEN used_at IS NOT NULL THEN 'used' WHEN revoked_at IS NOT NULL THEN 'revoked' WHEN expires_at<=now() THEN 'expired' ELSE 'available' END,created_at,expires_at FROM beta_invites WHERE cohort_slug='pilot' ORDER BY created_at DESC,id DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	items := []Invitation{}
	for rows.Next() {
		var i Invitation
		if err = rows.Scan(&i.ID, &i.Email, &i.Status, &i.CreatedAt, &i.ExpiresAt); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return items, tx.Commit(ctx)
}
func (s *Service) Public(ctx context.Context) (bool, error) {
	var open bool
	err := s.Pool.QueryRow(ctx, "SELECT enrollment_open FROM beta_cohorts WHERE slug='pilot'").Scan(&open)
	return open, err
}
func (s *Service) Activity(ctx context.Context, user string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE beta_members m SET last_seen_at=now(),returned_d7_at=CASE WHEN joined_at<=now()-interval '7 days' AND joined_at>now()-interval '8 days' THEN COALESCE(returned_d7_at,now()) ELSE returned_d7_at END WHERE user_id=$1 AND (last_seen_at IS NULL OR last_seen_at<now()-interval '1 minute') AND EXISTS(SELECT 1 FROM users WHERE id=$1 AND status='active')`, user)
	return err
}
