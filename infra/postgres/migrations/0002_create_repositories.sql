CREATE TABLE IF NOT EXISTS repositories (
    id BIGSERIAL PRIMARY KEY,
    github_repo_id BIGINT NOT NULL UNIQUE,
    owner VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    full_name VARCHAR(512) NOT NULL UNIQUE,
    default_branch VARCHAR(255) NOT NULL,
    is_private BOOLEAN NOT NULL DEFAULT FALSE,
    installation_id BIGINT,
    webhook_id BIGINT,
    sync_status VARCHAR(32) NOT NULL DEFAULT 'pending',
    last_synced_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT repositories_sync_status_check
        CHECK (sync_status IN ('pending', 'importing', 'ready', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_repositories_owner ON repositories (owner);
CREATE INDEX IF NOT EXISTS idx_repositories_sync_status ON repositories (sync_status);

CREATE TABLE IF NOT EXISTS user_repositories (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    role VARCHAR(32) NOT NULL DEFAULT 'owner',
    connected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, repository_id),
    CONSTRAINT user_repositories_role_check
        CHECK (role IN ('owner', 'viewer'))
);

CREATE INDEX IF NOT EXISTS idx_user_repositories_repository_id
    ON user_repositories (repository_id);

CREATE TABLE IF NOT EXISTS repository_sync_runs (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    sync_type VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'queued',
    error_message TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT repository_sync_runs_sync_type_check
        CHECK (sync_type IN ('initial', 'incremental', 'manual')),
    CONSTRAINT repository_sync_runs_status_check
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_repository_sync_runs_repository_id
    ON repository_sync_runs (repository_id);

CREATE INDEX IF NOT EXISTS idx_repository_sync_runs_status
    ON repository_sync_runs (status);
