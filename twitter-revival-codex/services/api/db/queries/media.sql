-- name: InsertMedia :one
INSERT INTO media(owner_id,storage_key,mime_type,bytes,width,height,alt_text) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING *;

-- name: ListDraftMedia :many
SELECT m.* FROM media m WHERE m.owner_id=$1 AND m.deleted_at IS NULL
AND NOT EXISTS(SELECT 1 FROM post_media pm WHERE pm.media_id=m.id)
ORDER BY m.created_at DESC,m.id DESC LIMIT 12;

-- name: MediaUsage :one
SELECT COALESCE(sum(bytes),0)::bigint AS bytes,
 (SELECT count(*) FROM media m WHERE m.owner_id=sqlc.arg(owner_id)::uuid AND m.deleted_at IS NULL
  AND NOT EXISTS(SELECT 1 FROM post_media pm WHERE pm.media_id=m.id)) AS drafts
FROM media WHERE owner_id=sqlc.arg(owner_id)::uuid AND deleted_at IS NULL;

-- name: LockMedia :many
SELECT * FROM media WHERE id=ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR UPDATE;

-- name: MediaAttached :one
SELECT EXISTS(SELECT 1 FROM post_media WHERE media_id=$1);

-- name: AttachMedia :exec
INSERT INTO post_media(post_id,media_id,position) VALUES($1,$2,$3);

-- name: DeleteDraftMedia :exec
UPDATE media SET deleted_at=COALESCE(deleted_at,clock_timestamp()) WHERE id=$1;

-- name: GetReadableMedia :one
SELECT m.* FROM media m JOIN users owner ON owner.id=m.owner_id
WHERE m.id=sqlc.arg(media_id)::uuid AND m.deleted_at IS NULL AND owner.status='active'
AND EXISTS(SELECT 1 FROM users WHERE id=sqlc.arg(viewer_id)::uuid AND status='active')
AND ((m.owner_id=sqlc.arg(viewer_id)::uuid AND NOT EXISTS(SELECT 1 FROM post_media pm WHERE pm.media_id=m.id))
 OR EXISTS(SELECT 1 FROM post_media pm JOIN post_audience v ON v.post_id=pm.post_id AND v.viewer_id=sqlc.arg(viewer_id)::uuid WHERE pm.media_id=m.id));

-- name: ListPostMedia :many
SELECT pm.post_id,m.id,m.mime_type,m.width,m.height,m.alt_text,m.bytes
FROM post_media pm JOIN media m ON m.id=pm.media_id JOIN post_audience v ON v.post_id=pm.post_id AND v.viewer_id=sqlc.arg(viewer_id)::uuid
WHERE pm.post_id=ANY(sqlc.arg(post_ids)::uuid[]) AND m.deleted_at IS NULL
ORDER BY pm.post_id,pm.position;
