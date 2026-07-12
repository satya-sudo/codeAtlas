CREATE TABLE IF NOT EXISTS module_cochange (
    id BIGSERIAL PRIMARY KEY,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    left_module_id BIGINT REFERENCES modules(id) ON DELETE SET NULL,
    left_module_name VARCHAR(255) NOT NULL,
    left_path_prefix VARCHAR(1024) NOT NULL,
    right_module_id BIGINT REFERENCES modules(id) ON DELETE SET NULL,
    right_module_name VARCHAR(255) NOT NULL,
    right_path_prefix VARCHAR(1024) NOT NULL,
    cochange_count INT NOT NULL DEFAULT 0,
    last_cochanged_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT module_cochange_pair_unique UNIQUE (repository_id, left_path_prefix, right_path_prefix)
);

CREATE INDEX IF NOT EXISTS idx_module_cochange_repository_id
    ON module_cochange (repository_id);

CREATE INDEX IF NOT EXISTS idx_module_cochange_repository_count
    ON module_cochange (repository_id, cochange_count DESC, last_cochanged_at DESC);
