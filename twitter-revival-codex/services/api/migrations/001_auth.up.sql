CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    handle CITEXT NOT NULL CONSTRAINT users_handle_unique UNIQUE,
    email CITEXT NOT NULL CONSTRAINT users_email_unique UNIQUE,
    password_hash TEXT NOT NULL,
    email_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    status TEXT NOT NULL DEFAULT 'active',
    CONSTRAINT users_handle_format CHECK (handle::text COLLATE "C" ~ '^[A-Za-z0-9_]{3,20}$'),
    CONSTRAINT users_email_length CHECK (length(email::text) BETWEEN 3 AND 254),
    CONSTRAINT users_password_hash_format CHECK (password_hash LIKE '$argon2id$%'),
    CONSTRAINT users_status_valid CHECK (status IN ('active', 'suspended', 'deleted')),
    CONSTRAINT users_timestamps_valid CHECK (updated_at >= created_at)
);

CREATE TABLE profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 50),
    bio TEXT NOT NULL DEFAULT '' CHECK (char_length(bio) <= 160),
    avatar_key TEXT,
    banner_key TEXT,
    location_text TEXT NOT NULL DEFAULT '' CHECK (char_length(location_text) <= 100),
    website_url TEXT CHECK (char_length(website_url) <= 2048)
);

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT sessions_expiry_valid CHECK (expires_at > created_at),
    CONSTRAINT sessions_revocation_valid CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);
CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);
