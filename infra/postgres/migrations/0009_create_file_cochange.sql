CREATE TABLE IF NOT EXISTS file_cochange (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    left_file_id BIGINT REFERENCES files(id) ON DELETE SET NULL,
    left_path VARCHAR(1024) NOT NULL,
    right_file_id BIGINT REFERENCES files(id) ON DELETE SET NULL,
    right_path VARCHAR(1024) NOT NULL,
    cochange_count INT NOT NULL DEFAULT 0,
    last_cochanged_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT file_cochange_pair_unique UNIQUE (repository_id, left_path, right_path)
);

CREATE INDEX IF NOT EXISTS idx_file_cochange_repository_id
    ON file_cochange (repository_id);

CREATE INDEX IF NOT EXISTS idx_file_cochange_repository_count
    ON file_cochange (repository_id, cochange_count DESC, last_cochanged_at DESC);
