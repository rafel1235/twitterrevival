-- name: LockAccounts :many
SELECT id,status,role FROM users WHERE id=ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR UPDATE;

-- name: ConsumeRateLimit :one
INSERT INTO rate_limits(subject,bucket,window_start,requests)
VALUES(sqlc.arg(subject),sqlc.arg(bucket),date_trunc('minute',clock_timestamp()),1)
ON CONFLICT(subject,bucket,window_start) DO UPDATE SET requests=rate_limits.requests+1
RETURNING requests;

-- name: RecentDuplicatePost :one
SELECT EXISTS(SELECT 1 FROM posts WHERE author_id=sqlc.arg(author_id)::uuid
 AND created_at>now()-interval '10 minutes' AND lower(body)=lower(sqlc.arg(body)::text)
 AND COALESCE(reply_to_id::text,'')=sqlc.arg(reply_id)::text
 AND COALESCE(quote_of_id::text,'')=sqlc.arg(quote_id)::text
 AND COALESCE(community_id::text,'')=sqlc.arg(community_id)::text)::boolean;
