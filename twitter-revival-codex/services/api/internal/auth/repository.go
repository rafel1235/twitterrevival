package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"radura.local/api/internal/beta"
	"radura.local/api/internal/db"
)

var ErrConflict = errors.New("handle or email already registered")
var ErrUnauthorized = errors.New("unauthorized")
var ErrBusy = errors.New("password workers busy")

type User struct {
	ID            string    `json:"id"`
	Handle        string    `json:"handle"`
	Email         string    `json:"email"`
	DisplayName   string    `json:"display_name"`
	CreatedAt     time.Time `json:"created_at"`
	EmailVerified bool      `json:"email_verified"`
	AvatarStyle   string    `json:"avatar_style"`
	Role          string    `json:"role"`
}

type Account struct {
	User         User
	PasswordHash string
	Status       string
}
type NewUser struct{ Handle, Email, DisplayName, PasswordHash, InviteToken string }
type NewSession struct {
	Hash      []byte
	ExpiresAt time.Time
}

type Repository interface {
	Register(context.Context, NewUser, NewSession) (User, error)
	FindAccount(context.Context, string) (Account, error)
	CreateSession(context.Context, string, NewSession, string) error
	SessionUser(context.Context, []byte) (User, error)
	RevokeSession(context.Context, []byte) error
}

type PostgresRepository struct {
	Pool *pgxpool.Pool
	Beta *beta.Service
}

func (r *PostgresRepository) Register(ctx context.Context, input NewUser, session NewSession) (User, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)
	var inviteID, cohort string
	if r.Beta != nil {
		inviteID, cohort, err = r.Beta.Reserve(ctx, tx, input.InviteToken, input.Email)
		if err != nil {
			return User{}, err
		}
	}
	row, err := queries.InsertUser(ctx, db.InsertUserParams{Handle: input.Handle, Email: input.Email, PasswordHash: input.PasswordHash})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, ErrConflict
		}
		return User{}, err
	}
	user := User{ID: row.ID, Handle: row.Handle, Email: row.Email, DisplayName: input.DisplayName, CreatedAt: row.CreatedAt.Time.UTC(), AvatarStyle: "sage", Role: "member"}
	if err = queries.InsertProfile(ctx, db.InsertProfileParams{UserID: user.ID, DisplayName: input.DisplayName}); err != nil {
		return User{}, err
	}
	if err = queries.InsertSession(ctx, db.InsertSessionParams{UserID: user.ID, TokenHash: session.Hash, ExpiresAt: pgtype.Timestamptz{Time: session.ExpiresAt, Valid: true}}); err != nil {
		return User{}, err
	}
	if r.Beta != nil {
		if err = r.Beta.Admit(ctx, tx, inviteID, cohort, user.ID); err != nil {
			return User{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return user, nil
}

func (r *PostgresRepository) FindAccount(ctx context.Context, email string) (Account, error) {
	row, err := db.New(r.Pool).FindAccount(ctx, email)
	a := Account{User: User{ID: row.ID, Handle: row.Handle, Email: row.Email, DisplayName: row.DisplayName, CreatedAt: row.CreatedAt.Time.UTC(), EmailVerified: row.EmailVerified, AvatarStyle: row.AvatarStyle, Role: row.Role}, PasswordHash: row.PasswordHash, Status: row.Status}
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrUnauthorized
	}
	return a, err
}

func (r *PostgresRepository) CreateSession(ctx context.Context, userID string, session NewSession, expectedHash string) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)
	if _, err = queries.LockActiveAccount(ctx, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUnauthorized
		}
		return err
	}
	rows, err := queries.InsertActiveSession(ctx, db.InsertActiveSessionParams{UserID: userID, TokenHash: session.Hash, ExpiresAt: pgtype.Timestamptz{Time: session.ExpiresAt, Valid: true}, ExpectedPasswordHash: expectedHash})
	if err == nil && rows == 0 {
		return ErrUnauthorized
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) SessionUser(ctx context.Context, hash []byte) (User, error) {
	row, err := db.New(r.Pool).FindSessionUser(ctx, hash)
	u := User{ID: row.ID, Handle: row.Handle, Email: row.Email, DisplayName: row.DisplayName, CreatedAt: row.CreatedAt.Time.UTC(), EmailVerified: row.EmailVerified, AvatarStyle: row.AvatarStyle, Role: row.Role}
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrUnauthorized
	}
	return u, err
}

func (r *PostgresRepository) RevokeSession(ctx context.Context, hash []byte) error {
	return db.New(r.Pool).RevokeSession(ctx, hash)
}
