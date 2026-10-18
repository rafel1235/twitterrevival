CREATE TABLE media (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL UNIQUE,
    mime_type TEXT NOT NULL CHECK (mime_type IN ('image/jpeg','image/png')),
    bytes BIGINT NOT NULL CHECK (bytes BETWEEN 1 AND 8388608),
    width INTEGER NOT NULL CHECK (width BETWEEN 1 AND 4096),
    height INTEGER NOT NULL CHECK (height BETWEEN 1 AND 4096),
    alt_text TEXT NOT NULL CHECK (char_length(alt_text) BETWEEN 1 AND 200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CHECK (width::bigint*height <= 8000000)
);
CREATE INDEX media_owner_idx ON media(owner_id) WHERE deleted_at IS NULL;
CREATE TABLE post_media (
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    media_id UUID NOT NULL UNIQUE REFERENCES media(id),
    position INTEGER NOT NULL CHECK (position BETWEEN 0 AND 3),
    PRIMARY KEY(post_id,media_id),
    UNIQUE(post_id,position)
);

ALTER TABLE posts ADD COLUMN search_document TSVECTOR GENERATED ALWAYS AS (to_tsvector('simple',body)) STORED;
CREATE INDEX posts_search_idx ON posts USING GIN(search_document) WHERE deleted_at IS NULL;
CREATE INDEX users_handle_prefix_idx ON users(lower(handle::text) text_pattern_ops) WHERE status='active';

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipient_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('follow','like','repost','reply','quote')),
    subject_id UUID NOT NULL,
    post_id UUID REFERENCES posts(id) ON DELETE CASCADE,
    target_post_id UUID REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    read_at TIMESTAMPTZ,
    UNIQUE(recipient_id,actor_id,type,subject_id),
    CHECK(recipient_id<>actor_id),
    CHECK((type='follow' AND post_id IS NULL AND target_post_id IS NULL AND subject_id=actor_id)
       OR (type<>'follow' AND post_id IS NOT NULL AND target_post_id IS NOT NULL)),
    CHECK(read_at IS NULL OR read_at>=created_at)
);
CREATE INDEX notifications_recipient_cursor_idx ON notifications(recipient_id,created_at DESC,id DESC);
CREATE INDEX notifications_unread_idx ON notifications(recipient_id) WHERE read_at IS NULL;
CREATE INDEX notifications_post_idx ON notifications(post_id);
CREATE INDEX notifications_target_idx ON notifications(target_post_id);

CREATE FUNCTION notify_follow() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM notifications WHERE recipient_id=OLD.followed_id AND actor_id=OLD.follower_id AND type='follow';
        RETURN OLD;
    END IF;
    INSERT INTO notifications(recipient_id,actor_id,type,subject_id)
    SELECT NEW.followed_id,NEW.follower_id,'follow',NEW.follower_id
    WHERE NEW.followed_id<>NEW.follower_id
    AND (SELECT count(*)=2 FROM users WHERE id IN(NEW.followed_id,NEW.follower_id) AND status='active')
    AND NOT EXISTS(SELECT 1 FROM blocks WHERE
        (blocker_id=NEW.followed_id AND blocked_id=NEW.follower_id) OR
        (blocker_id=NEW.follower_id AND blocked_id=NEW.followed_id))
    ON CONFLICT DO NOTHING;
    RETURN NEW;
END $$;
CREATE TRIGGER follows_notify AFTER INSERT OR DELETE ON follows FOR EACH ROW EXECUTE FUNCTION notify_follow();

CREATE FUNCTION notify_interaction() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE recipient UUID;
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM notifications WHERE actor_id=OLD.user_id AND type=TG_ARGV[0] AND subject_id=OLD.post_id;
        RETURN OLD;
    END IF;
    SELECT author_id INTO recipient FROM posts WHERE id=NEW.post_id AND deleted_at IS NULL;
    INSERT INTO notifications(recipient_id,actor_id,type,subject_id,post_id,target_post_id)
    SELECT recipient,NEW.user_id,TG_ARGV[0],NEW.post_id,NEW.post_id,NEW.post_id
    WHERE recipient IS NOT NULL AND recipient<>NEW.user_id
    AND (SELECT count(*)=2 FROM users WHERE id IN(recipient,NEW.user_id) AND status='active')
    AND NOT EXISTS(SELECT 1 FROM blocks WHERE
        (blocker_id=recipient AND blocked_id=NEW.user_id) OR (blocker_id=NEW.user_id AND blocked_id=recipient))
    ON CONFLICT DO NOTHING;
    RETURN NEW;
END $$;
CREATE TRIGGER likes_notify AFTER INSERT OR DELETE ON likes FOR EACH ROW EXECUTE FUNCTION notify_interaction('like');
CREATE TRIGGER reposts_notify AFTER INSERT OR DELETE ON reposts FOR EACH ROW EXECUTE FUNCTION notify_interaction('repost');

CREATE FUNCTION notify_post() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE recipient UUID; target UUID; kind TEXT;
BEGIN
    IF NEW.deleted_at IS NOT NULL THEN
        DELETE FROM notifications WHERE post_id=NEW.id OR target_post_id=NEW.id;
        RETURN NEW;
    END IF;
    IF TG_OP<>'INSERT' THEN RETURN NEW; END IF;
    target:=COALESCE(NEW.reply_to_id,NEW.quote_of_id);
    IF target IS NULL THEN RETURN NEW; END IF;
    kind:=CASE WHEN NEW.reply_to_id IS NOT NULL THEN 'reply' ELSE 'quote' END;
    SELECT author_id INTO recipient FROM posts WHERE id=target AND deleted_at IS NULL;
    INSERT INTO notifications(recipient_id,actor_id,type,subject_id,post_id,target_post_id)
    SELECT recipient,NEW.author_id,kind,NEW.id,NEW.id,target
    WHERE recipient IS NOT NULL AND recipient<>NEW.author_id
    AND (SELECT count(*)=2 FROM users WHERE id IN(recipient,NEW.author_id) AND status='active')
    AND NOT EXISTS(SELECT 1 FROM blocks WHERE
        (blocker_id=recipient AND blocked_id=NEW.author_id) OR (blocker_id=NEW.author_id AND blocked_id=recipient))
    ON CONFLICT DO NOTHING;
    RETURN NEW;
END $$;
CREATE TRIGGER posts_notify AFTER INSERT OR UPDATE OF deleted_at ON posts FOR EACH ROW EXECUTE FUNCTION notify_post();
CREATE FUNCTION clear_blocked_notifications() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM notifications WHERE
        (recipient_id=NEW.blocker_id AND actor_id=NEW.blocked_id) OR
        (recipient_id=NEW.blocked_id AND actor_id=NEW.blocker_id);
    RETURN NEW;
END $$;
CREATE TRIGGER blocks_clear_notifications AFTER INSERT ON blocks FOR EACH ROW EXECUTE FUNCTION clear_blocked_notifications();

CREATE VIEW visible_notifications AS
SELECT n.*,a.handle,p.display_name,p.avatar_style
FROM notifications n JOIN users a ON a.id=n.actor_id JOIN profiles p ON p.user_id=a.id
JOIN users recipient ON recipient.id=n.recipient_id
WHERE a.status='active' AND recipient.status='active'
AND NOT EXISTS(SELECT 1 FROM blocks b WHERE
    (b.blocker_id=n.recipient_id AND b.blocked_id=n.actor_id) OR (b.blocker_id=n.actor_id AND b.blocked_id=n.recipient_id))
AND ((n.type='follow' AND EXISTS(SELECT 1 FROM follows f WHERE f.follower_id=n.actor_id AND f.followed_id=n.recipient_id))
 OR (n.type<>'follow'
     AND EXISTS(SELECT 1 FROM post_audience v WHERE v.viewer_id=n.recipient_id AND v.post_id=n.post_id)
     AND EXISTS(SELECT 1 FROM post_audience v WHERE v.viewer_id=n.recipient_id AND v.post_id=n.target_post_id)));
