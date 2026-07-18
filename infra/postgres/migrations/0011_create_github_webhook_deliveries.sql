CREATE TABLE IF NOT EXISTS github_webhook_deliveries (
    id BIGSERIAL PRIMARY KEY,
    delivery_id VARCHAR(255) NOT NULL UNIQUE,
    event VARCHAR(64) NOT NULL,
    action VARCHAR(64),
    repository_id BIGINT,
    installation_id BIGINT,
    status VARCHAR(32) NOT NULL DEFAULT 'received',
    error_message TEXT,
    payload_json JSONB,
    received_at TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT github_webhook_deliveries_status_check
        CHECK (status IN ('received', 'published', 'ignored', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_github_webhook_deliveries_repository_id
    ON github_webhook_deliveries (repository_id);

CREATE INDEX IF NOT EXISTS idx_github_webhook_deliveries_installation_id
    ON github_webhook_deliveries (installation_id);

CREATE INDEX IF NOT EXISTS idx_github_webhook_deliveries_status
    ON github_webhook_deliveries (status);

CREATE INDEX IF NOT EXISTS idx_github_webhook_deliveries_received_at
    ON github_webhook_deliveries (received_at DESC);
