-- Removing the audience column could publish formerly restricted posts.
-- Refuse atomically, including tombstones, rather than widen their audience.
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM posts WHERE community_id IS NOT NULL) THEN
  RAISE EXCEPTION 'cannot roll back communities while community posts exist';
 END IF;
END $$;
DROP TRIGGER posts_community_guard ON posts;
DROP FUNCTION guard_community_post();
DROP TRIGGER reposts_community_guard ON reposts;
DROP FUNCTION guard_community_repost();
DROP VIEW post_cards;
-- Restore the exact pre-community invoker view before removing dependencies.
CREATE OR REPLACE VIEW post_audience AS
SELECT viewer.id AS viewer_id,p.id AS post_id
FROM posts p JOIN users author ON author.id=p.author_id CROSS JOIN users viewer
WHERE p.deleted_at IS NULL AND p.moderation_hidden_at IS NULL AND p.visibility='public'
AND author.status='active' AND viewer.status='active'
AND NOT EXISTS(SELECT 1 FROM blocks b WHERE
 (b.blocker_id=viewer.id AND b.blocked_id=author.id) OR (b.blocker_id=author.id AND b.blocked_id=viewer.id));
ALTER TABLE posts DROP CONSTRAINT posts_community_visibility_check;
ALTER TABLE posts DROP CONSTRAINT posts_visibility_check;
ALTER TABLE posts ADD CONSTRAINT posts_visibility_check CHECK(visibility='public');
ALTER TABLE posts DROP COLUMN community_id;
DROP VIEW community_audience;
DROP TABLE community_events,community_members,communities;
CREATE VIEW post_cards AS
SELECT v.viewer_id,p.id,p.author_id,p.body,p.created_at,p.updated_at,
    COALESCE(p.reply_to_id::text,'')::text AS reply_to_id,
    COALESCE(p.quote_of_id::text,'')::text AS quote_of_id,
    u.handle,pr.display_name,pr.avatar_style,
    (SELECT count(*) FROM likes l JOIN users a ON a.id=l.user_id
     WHERE l.post_id=p.id AND a.status='active'
     AND NOT EXISTS(SELECT 1 FROM blocks b WHERE
       (b.blocker_id=a.id AND b.blocked_id IN(v.viewer_id,p.author_id)) OR
       (b.blocked_id=a.id AND b.blocker_id IN(v.viewer_id,p.author_id)))) AS likes_count,
    (SELECT count(*) FROM reposts r JOIN users a ON a.id=r.user_id
     WHERE r.post_id=p.id AND a.status='active'
     AND NOT EXISTS(SELECT 1 FROM blocks b WHERE
       (b.blocker_id=a.id AND b.blocked_id IN(v.viewer_id,p.author_id)) OR
       (b.blocked_id=a.id AND b.blocker_id IN(v.viewer_id,p.author_id)))) AS reposts_count,
    (SELECT count(*) FROM posts r JOIN post_audience rv ON rv.post_id=r.id AND rv.viewer_id=v.viewer_id
     WHERE r.reply_to_id=p.id) AS replies_count,
    (SELECT count(*) FROM posts q JOIN post_audience qv ON qv.post_id=q.id AND qv.viewer_id=v.viewer_id
     WHERE q.quote_of_id=p.id) AS quotes_count,
    EXISTS(SELECT 1 FROM likes l WHERE l.post_id=p.id AND l.user_id=v.viewer_id) AS liked,
    EXISTS(SELECT 1 FROM reposts r WHERE r.post_id=p.id AND r.user_id=v.viewer_id) AS reposted
FROM posts p JOIN post_audience v ON v.post_id=p.id
JOIN users u ON u.id=p.author_id JOIN profiles pr ON pr.user_id=u.id;
