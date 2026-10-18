package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"hash"
	"radura.local/api/internal/media"
	"sort"
)

type Table struct {
	Name   string `json:"name"`
	Rows   int    `json:"rows"`
	SHA256 string `json:"sha256"`
}
type Object struct {
	KeySHA256 string `json:"key_sha256"`
	SHA256    string `json:"sha256"`
	Bytes     int    `json:"bytes"`
}
type Manifest struct {
	Tables  []Table  `json:"tables"`
	Objects []Object `json:"objects"`
}

func digest(h hash.Hash) string { return hex.EncodeToString(h.Sum(nil)) }

// Verification is read-only. Only counts and SHA-256 digests leave the process;
// raw database rows, credentials and image contents are never printed.
func Build(ctx context.Context, pool *pgxpool.Pool, store *media.S3Store) (Manifest, error) {
	result := Manifest{Tables: []Table{}, Objects: []Object{}}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema=current_schema() AND table_type='BASE TABLE' ORDER BY table_name")
	if err != nil {
		return result, errors.New("cannot enumerate backup tables")
	}
	names := []string{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return result, err
		}
		names = append(names, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	for _, name := range names {
		h := sha256.New()
		table := Table{Name: name}
		rows, err = tx.Query(ctx, "SELECT row_to_json(t)::text FROM "+pgx.Identifier{name}.Sanitize()+" t ORDER BY row_to_json(t)::text COLLATE \"C\"")
		if err != nil {
			return result, errors.New("cannot hash backup table")
		}
		for rows.Next() {
			var value string
			if err = rows.Scan(&value); err != nil {
				rows.Close()
				return result, err
			}
			_, _ = h.Write([]byte(value))
			_, _ = h.Write([]byte{'\n'})
			table.Rows++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return result, err
		}
		table.SHA256 = digest(h)
		result.Tables = append(result.Tables, table)
	}
	err = store.Objects(ctx, func(o media.StoredObject) error {
		data, e := store.Get(ctx, o.Key)
		if e != nil {
			return errors.New("cannot verify backup image")
		}
		h := sha256.Sum256(data)
		key := sha256.Sum256([]byte(o.Key))
		result.Objects = append(result.Objects, Object{hex.EncodeToString(key[:]), hex.EncodeToString(h[:]), len(data)})
		return nil
	})
	if err != nil {
		return result, err
	}
	sort.Slice(result.Objects, func(i, j int) bool { return result.Objects[i].KeySHA256 < result.Objects[j].KeySHA256 })
	return result, tx.Commit(ctx)
}
