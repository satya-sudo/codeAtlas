ALTER TABLE repository_sync_runs
    ADD COLUMN IF NOT EXISTS contributors_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS commits_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS commit_files_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS modules_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS files_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS duration_ms BIGINT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_repository_sync_runs_one_active_per_repository
    ON repository_sync_runs (repository_id)
    WHERE status IN ('queued', 'running');
