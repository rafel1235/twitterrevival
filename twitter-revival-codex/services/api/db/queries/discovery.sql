-- name: SearchPostIDs :many
SELECT p.id,p.created_at FROM posts p JOIN post_audience v ON v.post_id=p.id AND v.viewer_id=sqlc.arg(viewer_id)::uuid
WHERE p.community_id IS NULL AND p.search_document @@ websearch_to_tsquery('simple',sqlc.arg(term)::text)
AND (p.created_at,p.id)<(sqlc.arg(cursor_time)::timestamptz,sqlc.arg(cursor_id)::uuid)
ORDER BY p.created_at DESC,p.id DESC LIMIT sqlc.arg(page_limit);

-- name: SearchUsers :many
SELECT u.id,u.handle,p.display_name,p.avatar_style,u.created_at
FROM users u JOIN profiles p ON p.user_id=u.id
WHERE u.status='active' AND
(lower(u.handle::text) LIKE lower(sqlc.arg(prefix)::text) OR p.display_name ILIKE sqlc.arg(contains)::text)
AND NOT EXISTS(SELECT 1 FROM blocks b WHERE
 (b.blocker_id=sqlc.arg(viewer_id)::uuid AND b.blocked_id=u.id) OR (b.blocker_id=u.id AND b.blocked_id=sqlc.arg(viewer_id)::uuid))
AND (u.created_at,u.id)<(sqlc.arg(cursor_time)::timestamptz,sqlc.arg(cursor_id)::uuid)
ORDER BY u.created_at DESC,u.id DESC LIMIT sqlc.arg(page_limit);

-- name: ListNotifications :many
SELECT id,actor_id,type,COALESCE(post_id::text,'')::text AS post_id,created_at,read_at,handle,display_name,avatar_style
FROM visible_notifications WHERE recipient_id=sqlc.arg(recipient_id)::uuid
AND (created_at,id)<(sqlc.arg(cursor_time)::timestamptz,sqlc.arg(cursor_id)::uuid)
ORDER BY created_at DESC,id DESC LIMIT sqlc.arg(page_limit);

-- name: UnreadNotifications :one
SELECT count(*) FROM visible_notifications WHERE recipient_id=$1 AND read_at IS NULL;

-- name: ReadNotification :execrows
UPDATE notifications n SET read_at=COALESCE(n.read_at,clock_timestamp())
WHERE n.recipient_id=sqlc.arg(recipient_id)::uuid AND n.id=sqlc.arg(notification_id)::uuid
AND n.id IN(SELECT v.id FROM visible_notifications v WHERE v.recipient_id=sqlc.arg(recipient_id)::uuid);

-- name: ReadAllNotifications :exec
UPDATE notifications n SET read_at=clock_timestamp()
WHERE n.recipient_id=$1 AND n.read_at IS NULL AND n.id IN(SELECT v.id FROM visible_notifications v WHERE v.recipient_id=$1);
