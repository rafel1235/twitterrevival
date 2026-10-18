package auth

import (
	"context"
)

func (r *PostgresRepository) enqueueMail(ctx context.Context, job mailJob) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Serialize only short enqueue transactions; no network/Argon2 under this lock.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(81716006)"); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM mail_jobs WHERE attempts<5 AND created_at>now()-interval '24 hours'").Scan(&count); err != nil {
		return err
	}
	if count >= 32 {
		return ErrBusy
	}
	var user any
	if job.UserID != "" {
		user = job.UserID
	}
	if _, err = tx.Exec(ctx, "UPDATE mail_jobs SET attempts=0,created_at=now(),available_at=now(),leased_until=NULL WHERE purpose=$3 AND (user_id=$1 OR ($1::uuid IS NULL AND email=$2)) AND (attempts>=5 OR created_at<now()-interval '24 hours')", user, job.Email, job.Purpose); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO mail_jobs(user_id,email,purpose) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, user, job.Email, job.Purpose)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *PostgresRepository) claimMail(ctx context.Context) (mailJob, string, int, error) {
	var job mailJob
	var id string
	var attempt int
	err := r.Pool.QueryRow(ctx, `UPDATE mail_jobs SET attempts=attempts+1,leased_until=now()+interval '30 seconds'
 WHERE id=(SELECT id FROM mail_jobs WHERE attempts<5 AND created_at>now()-interval '24 hours' AND available_at<=now() AND (leased_until IS NULL OR leased_until<now()) ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1)
 RETURNING id,COALESCE(user_id::text,''),email,purpose,attempts`).Scan(&id, &job.UserID, &job.Email, &job.Purpose, &attempt)
	return job, id, attempt, err
}
