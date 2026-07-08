CREATE TABLE IF NOT EXISTS commit_files (
    id BIGSERIAL PRIMARY KEY,
    commit_id BIGINT NOT NULL REFERENCES commits(id) ON DELETE CASCADE,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    file_id BIGINT REFERENCES files(id) ON DELETE SET NULL,
    module_id BIGINT REFERENCES modules(id) ON DELETE SET NULL,
    path VARCHAR(1024) NOT NULL,
    previous_path VARCHAR(1024),
    change_type VARCHAR(32) NOT NULL,
    additions INT NOT NULL DEFAULT 0,
    deletions INT NOT NULL DEFAULT 0,
    changes INT NOT NULL DEFAULT 0,
    patch_text TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT commit_files_change_type_check
        CHECK (change_type IN ('added', 'modified', 'deleted', 'renamed', 'copied'))
);

CREATE INDEX IF NOT EXISTS idx_commit_files_commit_id
    ON commit_files (commit_id);

CREATE INDEX IF NOT EXISTS idx_commit_files_repository_id
    ON commit_files (repository_id);

CREATE INDEX IF NOT EXISTS idx_commit_files_file_id
    ON commit_files (file_id);

CREATE INDEX IF NOT EXISTS idx_commit_files_module_id
    ON commit_files (module_id);

CREATE INDEX IF NOT EXISTS idx_commit_files_repository_path
    ON commit_files (repository_id, path);
