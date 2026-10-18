DROP TABLE mail_jobs;
DROP TRIGGER beta_follow ON follows;
DROP TRIGGER beta_post ON posts;
DROP TRIGGER beta_like ON likes;
DROP TRIGGER beta_repost ON reposts;
DROP FUNCTION beta_milestone();
DROP TABLE beta_members,beta_invite_events,beta_invites,beta_cohorts;
