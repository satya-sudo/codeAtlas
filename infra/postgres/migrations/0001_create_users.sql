CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    github_id BIGINT NOT NULL UNIQUE,
    username TEXT NOT NULL,
    avatar_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users (username);
