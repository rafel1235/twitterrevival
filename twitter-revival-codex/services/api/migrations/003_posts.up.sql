CREATE TABLE posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 280 AND body !~ '^[[:space:]]*$'),
    reply_to_id UUID REFERENCES posts(id),
    quote_of_id UUID REFERENCES posts(id),
    visibility TEXT NOT NULL DEFAULT 'public' CHECK (visibility = 'public'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CHECK (reply_to_id IS NULL OR quote_of_id IS NULL),
    CHECK (reply_to_id IS NULL OR reply_to_id <> id),
    CHECK (quote_of_id IS NULL OR quote_of_id <> id),
    CHECK (updated_at >= created_at),
    CHECK (deleted_at IS NULL OR deleted_at >= created_at)
);
CREATE INDEX posts_author_cursor_idx ON posts(author_id,created_at DESC,id DESC) WHERE deleted_at IS NULL;
CREATE INDEX posts_reply_cursor_idx ON posts(reply_to_id,created_at DESC,id DESC) WHERE deleted_at IS NULL;
CREATE INDEX posts_quote_idx ON posts(quote_of_id) WHERE deleted_at IS NULL;

CREATE TABLE likes (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id,post_id)
);
CREATE INDEX likes_post_idx ON likes(post_id);

CREATE TABLE reposts (
    id UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id,post_id)
);
CREATE INDEX reposts_user_cursor_idx ON reposts(user_id,created_at DESC,id DESC);
CREATE INDEX reposts_post_idx ON reposts(post_id);

-- Each API query supplies one viewer_id. PostgreSQL pushes that predicate into
-- these ordinary invoker views; no cross-user result is exposed by the API.
CREATE VIEW post_audience AS
SELECT viewer.id AS viewer_id,p.id AS post_id
FROM posts p JOIN users author ON author.id=p.author_id
CROSS JOIN users viewer
WHERE p.deleted_at IS NULL AND p.visibility='public'
AND author.status='active' AND viewer.status='active'
AND NOT EXISTS(SELECT 1 FROM blocks b WHERE
    (b.blocker_id=viewer.id AND b.blocked_id=author.id) OR
    (b.blocker_id=author.id AND b.blocked_id=viewer.id));

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
