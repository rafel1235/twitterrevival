-- name: InsertUser :one
INSERT INTO users(handle,email,password_hash)
VALUES($1,$2,$3)
RETURNING id,handle,email,created_at;

-- name: InsertProfile :exec
INSERT INTO profiles(user_id,display_name) VALUES($1,$2);

-- name: InsertSession :exec
INSERT INTO sessions(user_id,token_hash,expires_at) VALUES($1,$2,$3);

-- name: FindAccount :one
SELECT u.id,u.handle,u.email,p.display_name,p.avatar_style,u.created_at,u.password_hash,u.status,(u.email_verified_at IS NOT NULL)::boolean AS email_verified
FROM users u JOIN profiles p ON p.user_id=u.id WHERE u.email=$1;

-- name: InsertActiveSession :execrows
INSERT INTO sessions(user_id,token_hash,expires_at)
SELECT id,sqlc.arg(token_hash)::bytea,sqlc.arg(expires_at)::timestamptz
FROM users WHERE id=sqlc.arg(user_id)::uuid AND status='active' AND password_hash=sqlc.arg(expected_password_hash);

-- name: FindSessionUser :one
SELECT u.id,u.handle,u.email,p.display_name,p.avatar_style,u.created_at,(u.email_verified_at IS NOT NULL)::boolean AS email_verified
FROM sessions s JOIN users u ON u.id=s.user_id JOIN profiles p ON p.user_id=u.id
WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND u.status='active';

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL;
