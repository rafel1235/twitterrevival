package safety

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"radura.local/api/internal/auth"
	"radura.local/api/internal/db"
)

var ErrLimited = errors.New("persistent rate limit exceeded")
var ErrSpam = errors.New("recent duplicate post")

// All account-writing transactions take user rows in UUID order before taking
// pair/post/media locks. Erasure and suspension use the same ordering.
func LockActive(ctx context.Context, q *db.Queries, ids ...string) error {
	rows, err := q.LockAccounts(ctx, ids)
	if err != nil {
		return err
	}
	unique := map[string]bool{}
	for _, id := range ids {
		unique[id] = true
	}
	if len(rows) != len(unique) {
		return auth.ErrUnauthorized
	}
	for _, row := range rows {
		if row.Status != "active" {
			return auth.ErrUnauthorized
		}
	}
	return nil
}

type Guard struct{ Pool *pgxpool.Pool }

func (g *Guard) Allow(ctx context.Context, subject, bucket string, limit int) error {
	n, err := db.New(g.Pool).ConsumeRateLimit(ctx, db.ConsumeRateLimitParams{Subject: subject, Bucket: bucket})
	if err != nil {
		return err
	}
	if n > int32(limit) {
		return ErrLimited
	}
	return nil
}
