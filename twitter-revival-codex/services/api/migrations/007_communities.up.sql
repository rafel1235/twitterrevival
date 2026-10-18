CREATE TABLE communities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 3 AND 60 AND name !~ '^[[:space:]]*$'),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    rules TEXT NOT NULL DEFAULT '' CHECK (char_length(rules) <= 1000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at TIMESTAMPTZ,
    CHECK (updated_at >= created_at),
    CHECK (archived_at IS NULL OR archived_at >= created_at)
);
CREATE INDEX communities_cursor_idx ON communities(created_at DESC,id DESC) WHERE archived_at IS NULL;
CREATE INDEX communities_owner_idx ON communities(owner_id) WHERE archived_at IS NULL;
CREATE TABLE community_members (
    community_id UUID NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (community_id,user_id)
);
CREATE INDEX community_members_user_idx ON community_members(user_id,community_id);
CREATE INDEX community_members_cursor_idx ON community_members(community_id,created_at DESC,user_id DESC);
CREATE TABLE community_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    community_id UUID REFERENCES communities(id) ON DELETE SET NULL,
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL CHECK (action IN ('create','join','leave','update','archive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX community_events_retention_idx ON community_events(created_at);

-- Public discovery is within the authenticated beta. Metadata can be read
-- without joining; content always requires a current membership below.
CREATE VIEW community_audience AS
SELECT viewer.id AS viewer_id,c.id AS community_id
FROM communities c JOIN users owner ON owner.id=c.owner_id CROSS JOIN users viewer
WHERE c.archived_at IS NULL AND owner.status='active' AND viewer.status='active'
AND NOT EXISTS(SELECT 1 FROM blocks b WHERE
 (b.blocker_id=viewer.id AND b.blocked_id=owner.id) OR
 (b.blocker_id=owner.id AND b.blocked_id=viewer.id));

ALTER TABLE posts ADD COLUMN community_id UUID REFERENCES communities(id);
ALTER TABLE posts DROP CONSTRAINT posts_visibility_check;
ALTER TABLE posts ADD CONSTRAINT posts_visibility_check CHECK (visibility IN ('public','community'));
ALTER TABLE posts ADD CONSTRAINT posts_community_visibility_check CHECK ((community_id IS NULL)=(visibility='public'));
CREATE INDEX posts_community_cursor_idx ON posts(community_id,created_at DESC,id DESC) WHERE deleted_at IS NULL;

CREATE OR REPLACE VIEW post_audience AS
SELECT viewer.id AS viewer_id,p.id AS post_id
FROM posts p JOIN users author ON author.id=p.author_id CROSS JOIN users viewer
WHERE p.deleted_at IS NULL AND p.moderation_hidden_at IS NULL
AND author.status='active' AND viewer.status='active'
AND (p.community_id IS NULL OR EXISTS(
 SELECT 1 FROM community_members m JOIN community_audience cv ON cv.community_id=m.community_id AND cv.viewer_id=m.user_id
 WHERE m.community_id=p.community_id AND m.user_id=viewer.id))
AND NOT EXISTS(SELECT 1 FROM blocks b WHERE
 (b.blocker_id=viewer.id AND b.blocked_id=author.id) OR
 (b.blocker_id=author.id AND b.blocked_id=viewer.id));

CREATE OR REPLACE VIEW post_cards AS
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
    EXISTS(SELECT 1 FROM reposts r WHERE r.post_id=p.id AND r.user_id=v.viewer_id) AS reposted,
    COALESCE(p.community_id::text,'')::text AS community_id,
    COALESCE(c.name,'')::text AS community_name
FROM posts p JOIN post_audience v ON v.post_id=p.id
JOIN users u ON u.id=p.author_id JOIN profiles pr ON pr.user_id=u.id
LEFT JOIN communities c ON c.id=p.community_id;

-- An additional DB boundary prevents accidental cross-audience references or
-- reclassifying a post. The service also checks membership under row locks.
CREATE FUNCTION guard_community_post() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE target_community UUID;
BEGIN
 IF TG_OP='UPDATE' THEN
  IF NEW.community_id IS DISTINCT FROM OLD.community_id OR NEW.visibility<>OLD.visibility
     OR NEW.reply_to_id IS DISTINCT FROM OLD.reply_to_id OR NEW.quote_of_id IS DISTINCT FROM OLD.quote_of_id THEN
   RAISE EXCEPTION 'post audience is immutable' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
 END IF;
 IF NEW.community_id IS NOT NULL AND NOT EXISTS(
  SELECT 1 FROM community_members m JOIN community_audience v ON v.community_id=m.community_id AND v.viewer_id=m.user_id
  WHERE m.community_id=NEW.community_id AND m.user_id=NEW.author_id) THEN
  RAISE EXCEPTION 'community membership required' USING ERRCODE='23514';
 END IF;
 IF NEW.reply_to_id IS NOT NULL THEN
  SELECT community_id INTO target_community FROM posts WHERE id=NEW.reply_to_id;
  IF NEW.community_id IS DISTINCT FROM target_community THEN
   RAISE EXCEPTION 'reply audience mismatch' USING ERRCODE='23514';
  END IF;
 END IF;
 IF NEW.quote_of_id IS NOT NULL THEN
  SELECT community_id INTO target_community FROM posts WHERE id=NEW.quote_of_id;
  IF NEW.community_id IS NOT NULL OR target_community IS NOT NULL THEN
   RAISE EXCEPTION 'community quotes are not allowed' USING ERRCODE='23514';
  END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER posts_community_guard BEFORE INSERT OR UPDATE OF community_id,visibility,reply_to_id,quote_of_id ON posts FOR EACH ROW EXECUTE FUNCTION guard_community_post();
CREATE FUNCTION guard_community_repost() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM posts WHERE id=NEW.post_id AND community_id IS NOT NULL) THEN
  RAISE EXCEPTION 'community reposts are not allowed' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER reposts_community_guard BEFORE INSERT ON reposts FOR EACH ROW EXECUTE FUNCTION guard_community_repost();
