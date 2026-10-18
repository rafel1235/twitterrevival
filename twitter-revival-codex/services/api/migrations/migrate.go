package migrations

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.sql
var files embed.FS

const CurrentVersion = 3

type migration struct {
	version int
	name    string
}

func catalog() ([]migration, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var result []migration
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		version, err := strconv.Atoi(strings.SplitN(entry.Name(), "_", 2)[0])
		if err != nil {
			return nil, fmt.Errorf("invalid migration name: %s", entry.Name())
		}
		name := strings.TrimSuffix(entry.Name(), ".up.sql")
		if _, err = files.ReadFile(name + ".down.sql"); err != nil {
			return nil, err
		}
		result = append(result, migration{version, name})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].version < result[j].version })
	if len(result) != CurrentVersion {
		return nil, fmt.Errorf("migration catalog/version mismatch")
	}
	for i, m := range result {
		if m.version != i+1 {
			return nil, fmt.Errorf("migration versions must be contiguous")
		}
	}
	return result, nil
}

// Run applies pending migrations or rolls back exactly the last applied version.
// The entire operation is atomic and serialized across processes.
func Run(ctx context.Context, pool *pgxpool.Pool, direction string) error {
	if direction != "up" && direction != "down" {
		return fmt.Errorf("migration direction must be up or down")
	}
	available, err := catalog()
	if err != nil {
		return err
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
	rows, err := tx.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return err
	}
	applied := 0
	for rows.Next() {
		var version int
		if err = rows.Scan(&version); err != nil {
			rows.Close()
			return err
		}
		applied++
		if version != applied || version > len(available) {
			rows.Close()
			return fmt.Errorf("unknown or noncontiguous database migration version")
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	pending := available[applied:]
	if direction == "down" {
		pending = nil
		if applied > 0 {
			pending = available[applied-1 : applied]
		}
	}
	for _, m := range pending {
		sql, err := files.ReadFile(m.name + "." + direction + ".sql")
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("migration %d %s: %w", m.version, direction, err)
		}
		if direction == "up" {
			_, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", m.version)
		} else {
			_, err = tx.Exec(ctx, "DELETE FROM schema_migrations WHERE version=$1", m.version)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
