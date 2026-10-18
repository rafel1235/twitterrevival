CREATE OR REPLACE VIEW post_audience AS
SELECT viewer.id AS viewer_id,p.id AS post_id
FROM posts p JOIN users author ON author.id=p.author_id CROSS JOIN users viewer
WHERE p.deleted_at IS NULL AND p.visibility='public'
AND author.status='active' AND viewer.status='active'
AND NOT EXISTS(SELECT 1 FROM blocks b WHERE
 (b.blocker_id=viewer.id AND b.blocked_id=author.id) OR (b.blocker_id=author.id AND b.blocked_id=viewer.id));
DROP TABLE rate_limits,moderation_actions,reports;
DROP INDEX posts_spam_idx;
ALTER TABLE media DROP COLUMN purged_at;
ALTER TABLE posts DROP COLUMN moderation_hidden_at;
ALTER TABLE users DROP COLUMN role;
