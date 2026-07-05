CREATE TABLE IF NOT EXISTS repository_contributors (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    github_user_id BIGINT NOT NULL,
    username VARCHAR(255) NOT NULL,
    avatar_url TEXT,
    contributions_count INT NOT NULL DEFAULT 0,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT repository_contributors_unique_repository_user
        UNIQUE (repository_id, github_user_id)
);

CREATE INDEX IF NOT EXISTS idx_repository_contributors_repository_id
    ON repository_contributors (repository_id);

CREATE INDEX IF NOT EXISTS idx_repository_contributors_username
    ON repository_contributors (username);
