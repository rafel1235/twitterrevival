package migrations

import (
	"context"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.sql
var files embed.FS

// Run applies the initial migration atomically under a transaction advisory lock.
func Run(ctx context.Context, pool *pgxpool.Pool, direction string) error {
	if direction != "up" && direction != "down" {
		return fmt.Errorf("migration direction must be up or down")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(81716001)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=1)").Scan(&exists); err != nil {
		return err
	}
	if (direction == "up" && !exists) || (direction == "down" && exists) {
		sql, readErr := files.ReadFile("001_auth." + direction + ".sql")
		if readErr != nil {
			return readErr
		}
		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("migration 001 %s: %w", direction, err)
		}
		if direction == "up" {
			_, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES(1)")
		} else {
			_, err = tx.Exec(ctx, "DELETE FROM schema_migrations WHERE version=1")
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
