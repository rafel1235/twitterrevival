ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'member' CHECK(role IN ('member','moderator'));
ALTER TABLE posts ADD COLUMN moderation_hidden_at TIMESTAMPTZ;
ALTER TABLE media ADD COLUMN purged_at TIMESTAMPTZ;

CREATE OR REPLACE VIEW post_audience AS
SELECT viewer.id AS viewer_id,p.id AS post_id
FROM posts p JOIN users author ON author.id=p.author_id CROSS JOIN users viewer
WHERE p.deleted_at IS NULL AND p.moderation_hidden_at IS NULL AND p.visibility='public'
AND author.status='active' AND viewer.status='active'
AND NOT EXISTS(SELECT 1 FROM blocks b WHERE
 (b.blocker_id=viewer.id AND b.blocked_id=author.id) OR (b.blocker_id=author.id AND b.blocked_id=viewer.id));

CREATE TABLE reports (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 reporter_id UUID REFERENCES users(id) ON DELETE SET NULL,
 target_type TEXT NOT NULL CHECK(target_type IN ('post','user')),
 target_user_id UUID NOT NULL REFERENCES users(id),
 target_post_id UUID REFERENCES posts(id),
 reason TEXT NOT NULL CHECK(reason IN ('spam','harassment','illegal','other')),
 details TEXT NOT NULL CHECK(char_length(details) BETWEEN 10 AND 2000),
 status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','resolved','dismissed')),
 decision_reason TEXT NOT NULL DEFAULT '' CHECK(char_length(decision_reason)<=2000),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), reviewed_at TIMESTAMPTZ,
 CHECK((target_type='post')=(target_post_id IS NOT NULL)),
 CHECK(reporter_id IS NULL OR reporter_id<>target_user_id),
 CHECK((status='open')=(reviewed_at IS NULL))
);
CREATE UNIQUE INDEX reports_open_user_idx ON reports(reporter_id,target_user_id) WHERE status='open' AND target_type='user';
CREATE UNIQUE INDEX reports_open_post_idx ON reports(reporter_id,target_post_id) WHERE status='open' AND target_type='post';
CREATE INDEX reports_queue_idx ON reports(status,created_at DESC,id DESC);
CREATE INDEX reports_reporter_idx ON reports(reporter_id,created_at DESC,id DESC);
CREATE TABLE moderation_actions (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 moderator_id UUID REFERENCES users(id) ON DELETE SET NULL,
 report_id UUID REFERENCES reports(id) ON DELETE SET NULL,
 target_type TEXT NOT NULL CHECK(target_type IN ('post','user')),
 target_id UUID NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('dismiss','hide_post','restore_post','suspend_user','restore_user','grant_moderator','revoke_moderator')),
 reason TEXT NOT NULL CHECK(char_length(reason) BETWEEN 10 AND 2000),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX moderation_actions_cursor_idx ON moderation_actions(created_at DESC,id DESC);
-- The application only appends audit records. Dedicated retention maintenance
-- may remove records older than the documented review window.
CREATE TABLE rate_limits (
 subject TEXT NOT NULL, bucket TEXT NOT NULL, window_start TIMESTAMPTZ NOT NULL,
 requests INTEGER NOT NULL CHECK(requests>0), PRIMARY KEY(subject,bucket,window_start)
);
CREATE INDEX rate_limits_expiry_idx ON rate_limits(window_start);
CREATE INDEX posts_spam_idx ON posts(author_id,created_at DESC);
