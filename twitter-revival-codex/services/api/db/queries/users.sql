-- name: GetProfileByHandle :one
SELECT u.id,u.handle,p.display_name,p.bio,p.avatar_style,p.location_text,
       COALESCE(p.website_url,'')::text AS website_url,u.created_at,
       (u.id=sqlc.arg(viewer_id)::uuid) AS is_self,
       EXISTS(SELECT 1 FROM follows f WHERE f.follower_id=sqlc.arg(viewer_id)::uuid AND f.followed_id=u.id) AS following,
       EXISTS(SELECT 1 FROM follows f WHERE f.followed_id=sqlc.arg(viewer_id)::uuid AND f.follower_id=u.id) AS followed_by,
       (SELECT count(*) FROM follows f JOIN users v ON v.id=f.follower_id WHERE f.followed_id=u.id AND v.status='active'
        AND NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=sqlc.arg(viewer_id)::uuid AND b.blocked_id=v.id) OR (b.blocked_id=sqlc.arg(viewer_id)::uuid AND b.blocker_id=v.id))) AS followers_count,
       (SELECT count(*) FROM follows f JOIN users v ON v.id=f.followed_id WHERE f.follower_id=u.id AND v.status='active'
        AND NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=sqlc.arg(viewer_id)::uuid AND b.blocked_id=v.id) OR (b.blocked_id=sqlc.arg(viewer_id)::uuid AND b.blocker_id=v.id))) AS following_count
FROM users u JOIN profiles p ON p.user_id=u.id
WHERE u.handle=sqlc.arg(handle)::citext AND u.status='active'
AND NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=sqlc.arg(viewer_id)::uuid AND b.blocked_id=u.id) OR (b.blocked_id=sqlc.arg(viewer_id)::uuid AND b.blocker_id=u.id));

-- name: UpdateOwnProfile :exec
UPDATE profiles SET display_name=sqlc.arg(display_name),bio=sqlc.arg(bio),avatar_style=sqlc.arg(avatar_style),location_text=sqlc.arg(location_text),website_url=NULLIF(sqlc.arg(website_url)::text,'') WHERE user_id=sqlc.arg(user_id);

-- name: TouchUser :exec
UPDATE users SET updated_at=now() WHERE id=$1;

-- name: LockUserPair :exec
SELECT pg_advisory_xact_lock(hashtextextended(LEAST(sqlc.arg(actor_id)::text,sqlc.arg(target_id)::text)||':'||GREATEST(sqlc.arg(actor_id)::text,sqlc.arg(target_id)::text),0));

-- name: ActiveUserPair :one
SELECT count(*)=2 AS active FROM users WHERE status='active' AND (id=sqlc.arg(actor_id)::uuid OR id=sqlc.arg(target_id)::uuid);

-- name: PairBlocked :one
SELECT EXISTS(SELECT 1 FROM blocks WHERE (blocker_id=sqlc.arg(actor_id)::uuid AND blocked_id=sqlc.arg(target_id)::uuid) OR (blocker_id=sqlc.arg(target_id)::uuid AND blocked_id=sqlc.arg(actor_id)::uuid));

-- name: InsertFollow :exec
INSERT INTO follows(follower_id,followed_id) VALUES($1,$2) ON CONFLICT DO NOTHING;

-- name: DeleteFollow :exec
DELETE FROM follows WHERE follower_id=$1 AND followed_id=$2;

-- name: InsertBlock :exec
INSERT INTO blocks(blocker_id,blocked_id) VALUES($1,$2) ON CONFLICT DO NOTHING;

-- name: DeleteBlock :exec
DELETE FROM blocks WHERE blocker_id=$1 AND blocked_id=$2;

-- name: RemovePairFollows :exec
DELETE FROM follows WHERE (follower_id=sqlc.arg(actor_id)::uuid AND followed_id=sqlc.arg(target_id)::uuid) OR (follower_id=sqlc.arg(target_id)::uuid AND followed_id=sqlc.arg(actor_id)::uuid);

-- name: VisibleUserID :one
SELECT u.id FROM users u WHERE u.id=sqlc.arg(target_id)::uuid AND u.status='active'
AND NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=sqlc.arg(viewer_id)::uuid AND b.blocked_id=u.id) OR (b.blocked_id=sqlc.arg(viewer_id)::uuid AND b.blocker_id=u.id));

-- name: ListFollowers :many
SELECT u.id,u.handle,p.display_name,p.avatar_style,f.created_at AS connection_created_at
FROM follows f JOIN users u ON u.id=f.follower_id JOIN profiles p ON p.user_id=u.id
WHERE f.followed_id=sqlc.arg(target_id)::uuid AND u.status='active'
AND NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=sqlc.arg(viewer_id)::uuid AND b.blocked_id=sqlc.arg(target_id)::uuid) OR (b.blocked_id=sqlc.arg(viewer_id)::uuid AND b.blocker_id=sqlc.arg(target_id)::uuid))
AND (f.created_at,u.id)<(sqlc.arg(cursor_time)::timestamptz,sqlc.arg(cursor_id)::uuid)
AND NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=sqlc.arg(viewer_id)::uuid AND b.blocked_id=u.id) OR (b.blocked_id=sqlc.arg(viewer_id)::uuid AND b.blocker_id=u.id))
ORDER BY f.created_at DESC,u.id DESC LIMIT sqlc.arg(page_limit);

-- name: ListFollowing :many
SELECT u.id,u.handle,p.display_name,p.avatar_style,f.created_at AS connection_created_at
FROM follows f JOIN users u ON u.id=f.followed_id JOIN profiles p ON p.user_id=u.id
WHERE f.follower_id=sqlc.arg(target_id)::uuid AND u.status='active'
AND NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=sqlc.arg(viewer_id)::uuid AND b.blocked_id=sqlc.arg(target_id)::uuid) OR (b.blocked_id=sqlc.arg(viewer_id)::uuid AND b.blocker_id=sqlc.arg(target_id)::uuid))
AND (f.created_at,u.id)<(sqlc.arg(cursor_time)::timestamptz,sqlc.arg(cursor_id)::uuid)
AND NOT EXISTS(SELECT 1 FROM blocks b WHERE (b.blocker_id=sqlc.arg(viewer_id)::uuid AND b.blocked_id=u.id) OR (b.blocked_id=sqlc.arg(viewer_id)::uuid AND b.blocker_id=u.id))
ORDER BY f.created_at DESC,u.id DESC LIMIT sqlc.arg(page_limit);

-- name: ListBlocked :many
SELECT u.id,u.handle,p.display_name,p.avatar_style,b.created_at AS connection_created_at
FROM blocks b JOIN users u ON u.id=b.blocked_id JOIN profiles p ON p.user_id=u.id
WHERE b.blocker_id=sqlc.arg(viewer_id)::uuid AND u.status='active'
AND (b.created_at,u.id)<(sqlc.arg(cursor_time)::timestamptz,sqlc.arg(cursor_id)::uuid)
ORDER BY b.created_at DESC,u.id DESC LIMIT sqlc.arg(page_limit);
