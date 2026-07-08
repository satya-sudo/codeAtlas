CREATE TABLE IF NOT EXISTS modules (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    path_prefix VARCHAR(512) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT modules_unique_repository_name
        UNIQUE (repository_id, name)
);

CREATE INDEX IF NOT EXISTS idx_modules_repository_id
    ON modules (repository_id);

CREATE TABLE IF NOT EXISTS files (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    module_id BIGINT REFERENCES modules(id) ON DELETE SET NULL,
    path VARCHAR(1024) NOT NULL,
    extension VARCHAR(32),
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    first_seen_commit_sha VARCHAR(64),
    last_seen_commit_sha VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT files_unique_repository_path
        UNIQUE (repository_id, path)
);

CREATE INDEX IF NOT EXISTS idx_files_repository_id
    ON files (repository_id);

CREATE INDEX IF NOT EXISTS idx_files_module_id
    ON files (module_id);

CREATE TABLE IF NOT EXISTS commits (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    github_commit_sha VARCHAR(64) NOT NULL,
    author_github_user_id BIGINT,
    author_username VARCHAR(255),
    author_name VARCHAR(255),
    author_email VARCHAR(512),
    committed_at TIMESTAMPTZ NOT NULL,
    message TEXT,
    parent_count INT NOT NULL DEFAULT 0,
    additions INT NOT NULL DEFAULT 0,
    deletions INT NOT NULL DEFAULT 0,
    total_changes INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT commits_unique_repository_sha
        UNIQUE (repository_id, github_commit_sha)
);

CREATE INDEX IF NOT EXISTS idx_commits_repository_id
    ON commits (repository_id);

CREATE INDEX IF NOT EXISTS idx_commits_repository_committed_at
    ON commits (repository_id, committed_at DESC);

CREATE INDEX IF NOT EXISTS idx_commits_author_github_user_id
    ON commits (author_github_user_id);
