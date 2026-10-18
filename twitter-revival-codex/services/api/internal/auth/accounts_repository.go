package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"radura.local/api/internal/db"
)

var ErrInvalidToken = errors.New("invalid or expired account token")
var ErrCooldown = errors.New("account token recently issued")

const VerificationPurpose = "email_verification"
const ResetPurpose = "password_reset"
const zeroUUID = "00000000-0000-0000-0000-000000000000"

func (r *PostgresRepository) deliveryAccount(ctx context.Context, id, email string) (db.DeliveryAccountRow, error) {
	if id == "" {
		id = zeroUUID
	}
	return db.New(r.Pool).DeliveryAccount(ctx, db.DeliveryAccountParams{UserID: id, Email: email})
}
func (r *PostgresRepository) issueAccountToken(ctx context.Context, id, purpose string, hash []byte, expires time.Time) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	if _, err = q.LockActiveAccount(ctx, id); err != nil {
		return err
	}
	recent, err := q.TokenRecentlyIssued(ctx, db.TokenRecentlyIssuedParams{UserID: id, Purpose: purpose})
	if err != nil {
		return err
	}
	if recent {
		return ErrCooldown
	}
	if err = q.ConsumePurposeTokens(ctx, db.ConsumePurposeTokensParams{UserID: id, Purpose: purpose}); err != nil {
		return err
	}
	if err = q.InsertAccountToken(ctx, db.InsertAccountTokenParams{UserID: id, Purpose: purpose, TokenHash: hash, ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true}}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *PostgresRepository) accountTokenUser(ctx context.Context, hash []byte, purpose string) (string, error) {
	id, err := db.New(r.Pool).FindAccountToken(ctx, db.FindAccountTokenParams{TokenHash: hash, Purpose: purpose})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrInvalidToken
	}
	return id, err
}
func (r *PostgresRepository) consumeAccountToken(ctx context.Context, hash []byte, purpose, newPasswordHash string) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	id, err := q.FindAccountToken(ctx, db.FindAccountTokenParams{TokenHash: hash, Purpose: purpose})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidToken
	}
	if err != nil {
		return err
	}
	// Always lock user before token: resend, login and reset share this order.
	if _, err = q.LockActiveAccount(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidToken
	} else if err != nil {
		return err
	}
	if _, err = q.LockAccountToken(ctx, db.LockAccountTokenParams{TokenHash: hash, Purpose: purpose}); errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidToken
	} else if err != nil {
		return err
	}
	switch purpose {
	case VerificationPurpose:
		err = q.MarkEmailVerified(ctx, id)
	case ResetPurpose:
		if err = q.UpdatePassword(ctx, db.UpdatePasswordParams{ID: id, PasswordHash: newPasswordHash}); err != nil {
			return err
		}
		err = q.RevokeUserSessions(ctx, id)
	default:
		return ErrInvalidToken
	}
	if err != nil {
		return err
	}
	if err = q.ConsumePurposeTokens(ctx, db.ConsumePurposeTokensParams{UserID: id, Purpose: purpose}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
