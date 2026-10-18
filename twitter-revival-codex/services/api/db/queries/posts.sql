-- name: InsertPost :one
INSERT INTO posts(author_id,body,reply_to_id,quote_of_id,community_id,visibility)
VALUES(sqlc.arg(author_id)::uuid,sqlc.arg(body)::text,NULLIF(sqlc.arg(reply_id)::text,'')::uuid,NULLIF(sqlc.arg(quote_id)::text,'')::uuid,
NULLIF(sqlc.arg(community_id)::text,'')::uuid,CASE WHEN sqlc.arg(community_id)::text='' THEN 'public' ELSE 'community' END)
RETURNING id;

-- name: FindPostContext :one
SELECT p.author_id,COALESCE(p.community_id::text,'')::text AS community_id,COALESCE(c.owner_id::text,'')::text AS community_owner_id
FROM posts p LEFT JOIN communities c ON c.id=p.community_id WHERE p.id=$1;

-- name: FindCommunityOwner :one
SELECT COALESCE(owner_id::text,'')::text AS owner_id FROM communities WHERE id=$1;

-- name: LockPostCommunity :one
SELECT id FROM communities WHERE id=$1 FOR UPDATE;

-- name: CommunityPostAccess :one
SELECT EXISTS(SELECT 1 FROM community_audience WHERE community_id=sqlc.arg(community_id)::uuid AND viewer_id=sqlc.arg(viewer_id)::uuid) AS visible,
EXISTS(SELECT 1 FROM community_members WHERE community_id=sqlc.arg(community_id)::uuid AND user_id=sqlc.arg(viewer_id)::uuid) AS member;

-- name: LockPost :one
SELECT id,author_id,deleted_at,COALESCE(community_id::text,'')::text AS community_id FROM posts WHERE id=$1 FOR UPDATE;

-- name: GetPostCards :many
SELECT * FROM post_cards WHERE viewer_id=sqlc.arg(viewer_id)::uuid AND id=ANY(sqlc.arg(post_ids)::uuid[]);

-- name: EditOwnPost :exec
UPDATE posts SET body=sqlc.arg(body)::text,updated_at=clock_timestamp()
WHERE id=sqlc.arg(post_id)::uuid AND author_id=sqlc.arg(author_id)::uuid AND deleted_at IS NULL;

-- name: DeleteOwnPost :exec
UPDATE posts SET deleted_at=clock_timestamp(),updated_at=clock_timestamp()
WHERE id=sqlc.arg(post_id)::uuid AND author_id=sqlc.arg(author_id)::uuid AND deleted_at IS NULL;

-- name: LikePost :exec
INSERT INTO likes(user_id,post_id) VALUES($1,$2) ON CONFLICT DO NOTHING;

-- name: UnlikePost :exec
DELETE FROM likes WHERE user_id=$1 AND post_id=$2;

-- name: RepostPost :exec
INSERT INTO reposts(user_id,post_id) VALUES($1,$2) ON CONFLICT DO NOTHING;

-- name: UndoRepost :exec
DELETE FROM reposts WHERE user_id=$1 AND post_id=$2;

-- name: ListReplyIDs :many
SELECT p.id,p.created_at FROM posts p JOIN post_audience v ON v.post_id=p.id AND v.viewer_id=sqlc.arg(viewer_id)::uuid
JOIN post_audience parent ON parent.post_id=sqlc.arg(parent_id)::uuid AND parent.viewer_id=v.viewer_id
WHERE p.reply_to_id=sqlc.arg(parent_id)::uuid
AND (p.created_at,p.id)<(sqlc.arg(cursor_time)::timestamptz,sqlc.arg(cursor_id)::uuid)
ORDER BY p.created_at DESC,p.id DESC LIMIT sqlc.arg(page_limit);

-- name: ListPostEvents :many
WITH events AS (
  SELECT p.id AS event_id,p.id AS post_id,p.author_id AS actor_id,p.created_at,false AS is_repost
  FROM posts p WHERE p.deleted_at IS NULL
  UNION ALL
  SELECT r.id,r.post_id,r.user_id,r.created_at,true FROM reposts r
)
SELECT e.event_id,e.post_id,e.created_at,e.is_repost,u.id AS actor_id,u.handle,pr.display_name,pr.avatar_style
FROM events e JOIN post_audience v ON v.post_id=e.post_id AND v.viewer_id=sqlc.arg(viewer_id)::uuid
JOIN posts p ON p.id=e.post_id JOIN users u ON u.id=e.actor_id JOIN profiles pr ON pr.user_id=u.id
WHERE u.status='active' AND p.community_id IS NULL
AND ((sqlc.arg(profile_id)::text='' AND
     (e.actor_id=sqlc.arg(viewer_id)::uuid OR EXISTS(SELECT 1 FROM follows f WHERE f.follower_id=sqlc.arg(viewer_id)::uuid AND f.followed_id=e.actor_id)))
  OR (sqlc.arg(profile_id)::text<>'' AND e.actor_id=NULLIF(sqlc.arg(profile_id)::text,'')::uuid))
AND NOT EXISTS(SELECT 1 FROM blocks b WHERE
    (b.blocker_id=u.id AND b.blocked_id IN(v.viewer_id,p.author_id)) OR
    (b.blocked_id=u.id AND b.blocker_id IN(v.viewer_id,p.author_id)))
AND (e.created_at,e.event_id)<(sqlc.arg(cursor_time)::timestamptz,sqlc.arg(cursor_id)::uuid)
ORDER BY e.created_at DESC,e.event_id DESC LIMIT sqlc.arg(page_limit);

-- name: ListCommunityPostIDs :many
SELECT p.id,p.created_at FROM posts p JOIN post_audience v ON v.post_id=p.id AND v.viewer_id=sqlc.arg(viewer_id)::uuid
WHERE p.community_id=sqlc.arg(community_id)::uuid AND p.reply_to_id IS NULL
AND (p.created_at,p.id)<(sqlc.arg(cursor_time)::timestamptz,sqlc.arg(cursor_id)::uuid)
ORDER BY p.created_at DESC,p.id DESC LIMIT sqlc.arg(page_limit);
