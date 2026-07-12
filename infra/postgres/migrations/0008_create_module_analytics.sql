CREATE TABLE IF NOT EXISTS module_metrics (
    module_id BIGINT PRIMARY KEY REFERENCES modules(id) ON DELETE CASCADE,
    bus_factor INTEGER NOT NULL DEFAULT 0,
    active_contributors INTEGER NOT NULL DEFAULT 0,
    top_owner_percent NUMERIC(5,2) NOT NULL DEFAULT 0,
    risk VARCHAR(32) NOT NULL DEFAULT 'unknown',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT module_metrics_risk_check
        CHECK (risk IN ('low', 'medium', 'high', 'unknown'))
);

CREATE TABLE IF NOT EXISTS module_ownership (
    module_id BIGINT NOT NULL REFERENCES modules(id) ON DELETE CASCADE,
    github_user_id BIGINT,
    username VARCHAR(255) NOT NULL,
    ownership_percent NUMERIC(5,2) NOT NULL DEFAULT 0,
    commit_count INTEGER NOT NULL DEFAULT 0,
    changes_count INTEGER NOT NULL DEFAULT 0,
    files_touched_count INTEGER NOT NULL DEFAULT 0,
    rank INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT module_ownership_primary_key
        PRIMARY KEY (module_id, rank)
);

CREATE INDEX IF NOT EXISTS idx_module_ownership_module_id
    ON module_ownership (module_id);

CREATE TABLE IF NOT EXISTS module_expertise (
    module_id BIGINT NOT NULL REFERENCES modules(id) ON DELETE CASCADE,
    github_user_id BIGINT,
    username VARCHAR(255) NOT NULL,
    score INTEGER NOT NULL DEFAULT 0,
    raw_score INTEGER NOT NULL DEFAULT 0,
    commit_count INTEGER NOT NULL DEFAULT 0,
    files_touched_count INTEGER NOT NULL DEFAULT 0,
    recent_commit_count INTEGER NOT NULL DEFAULT 0,
    last_commit_at TIMESTAMPTZ,
    rank INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT module_expertise_primary_key
        PRIMARY KEY (module_id, rank)
);

CREATE INDEX IF NOT EXISTS idx_module_expertise_module_id
    ON module_expertise (module_id);
