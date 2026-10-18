-- name: DeliveryAccount :one
SELECT id,email,(email_verified_at IS NOT NULL)::boolean AS email_verified FROM users
WHERE status='active' AND (id=sqlc.arg(user_id)::uuid OR email=sqlc.arg(email)::citext);

-- name: LockActiveAccount :one
SELECT id FROM users WHERE id=$1 AND status='active' FOR UPDATE;

-- name: TokenRecentlyIssued :one
SELECT EXISTS(SELECT 1 FROM auth_tokens WHERE user_id=$1 AND purpose=$2 AND created_at>now()-interval '1 minute');

-- name: ConsumePurposeTokens :exec
UPDATE auth_tokens SET consumed_at=now() WHERE user_id=$1 AND purpose=$2 AND consumed_at IS NULL;

-- name: InsertAccountToken :exec
INSERT INTO auth_tokens(user_id,purpose,token_hash,expires_at) VALUES($1,$2,$3,$4);

-- name: FindAccountToken :one
SELECT t.user_id FROM auth_tokens t JOIN users u ON u.id=t.user_id
WHERE t.token_hash=$1 AND t.purpose=$2 AND t.consumed_at IS NULL AND t.expires_at>now() AND u.status='active';

-- name: LockAccountToken :one
SELECT t.user_id FROM auth_tokens t JOIN users u ON u.id=t.user_id
WHERE t.token_hash=$1 AND t.purpose=$2 AND t.consumed_at IS NULL AND t.expires_at>now() AND u.status='active'
FOR UPDATE OF t;

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified_at=COALESCE(email_verified_at,now()),updated_at=now() WHERE id=$1;

-- name: UpdatePassword :exec
UPDATE users SET password_hash=$2,updated_at=now() WHERE id=$1;

-- name: RevokeUserSessions :exec
UPDATE sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL;
