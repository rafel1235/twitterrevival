package privacy

import (
	"context"
	"log/slog"
	"radura.local/api/internal/media"
	"time"
)

type SweepResult struct {
	Purged  int `json:"purged"`
	Pending int `json:"pending"`
}

// Tombstones form a durable deletion queue. S3 deletion is idempotent; a crash
// before the database acknowledgement leaves the work available for retry.
func (s *Service) Sweep(ctx context.Context) (SweepResult, error) {
	result := SweepResult{}
	_, err := s.Pool.Exec(ctx, `UPDATE media m SET deleted_at=now() WHERE deleted_at IS NULL AND
 ((created_at<now()-interval '24 hours' AND NOT EXISTS(SELECT 1 FROM post_media pm WHERE pm.media_id=m.id))
 OR EXISTS(SELECT 1 FROM post_media pm JOIN posts p ON p.id=pm.post_id WHERE pm.media_id=m.id AND p.deleted_at<now()-interval '30 days'))`)
	if err != nil {
		return result, err
	}
	rows, err := s.Pool.Query(ctx, "SELECT id,storage_key FROM media WHERE deleted_at IS NOT NULL AND purged_at IS NULL ORDER BY deleted_at,id LIMIT 200")
	if err != nil {
		return result, err
	}
	type item struct{ id, key string }
	items := []item{}
	for rows.Next() {
		var i item
		if err = rows.Scan(&i.id, &i.key); err != nil {
			rows.Close()
			return result, err
		}
		items = append(items, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	for _, i := range items {
		if err = s.Store.Delete(ctx, i.key); err != nil {
			result.Pending++
			continue
		}
		if _, err = s.Pool.Exec(ctx, "UPDATE media SET purged_at=now(),alt_text='Immagine rimossa' WHERE id=$1", i.id); err != nil {
			return result, err
		}
		result.Purged++
	}
	statements := []string{
		"DELETE FROM mail_jobs WHERE created_at<now()-interval '24 hours'",
		"DELETE FROM beta_invites WHERE expires_at<now()-interval '30 days' OR revoked_at<now()-interval '30 days'",
		"UPDATE posts SET body='[contenuto eliminato]' WHERE deleted_at<now()-interval '30 days' AND body<>'[contenuto eliminato]'",
		"DELETE FROM sessions WHERE expires_at<now() OR revoked_at<now()-interval '1 day'",
		"DELETE FROM auth_tokens WHERE expires_at<now() OR consumed_at<now()-interval '1 day'",
		"DELETE FROM rate_limits WHERE window_start<now()-interval '10 minutes'",
		"DELETE FROM reports WHERE reviewed_at<now()-interval '180 days'",
		"DELETE FROM moderation_actions WHERE created_at<now()-interval '180 days'",
	}
	for _, statement := range statements {
		if _, err = s.Pool.Exec(ctx, statement); err != nil {
			return result, err
		}
	}
	return result, nil
}
func (s *Service) RunMaintenance(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		run, cancel := context.WithTimeout(ctx, 45*time.Second)
		result, err := s.Sweep(run)
		cancel()
		if err != nil || result.Pending > 0 {
			slog.Warn("privacy maintenance incomplete")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Only objects older than 24 hours without any database record are orphans.
// This grace period protects in-flight uploads (S3 put precedes DB commit).
func (s *Service) Reconcile(ctx context.Context) (int, error) {
	store, ok := s.Store.(media.ObjectLister)
	if !ok {
		return 0, nil
	}
	count := 0
	err := store.Objects(ctx, func(o media.StoredObject) error {
		if o.Modified.IsZero() || o.Modified.After(time.Now().Add(-24*time.Hour)) {
			return nil
		}
		var exists bool
		if err := s.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM media WHERE storage_key=$1)", o.Key).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return nil
		}
		if err := s.Store.Delete(ctx, o.Key); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}
