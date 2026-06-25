CREATE TABLE IF NOT EXISTS github_app_installations (
    id BIGSERIAL PRIMARY KEY,
    installation_id BIGINT NOT NULL UNIQUE,
    installed_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    account_login VARCHAR(255),
    account_type VARCHAR(32),
    setup_action VARCHAR(32),
    status VARCHAR(32) NOT NULL DEFAULT 'pending_verification',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT github_app_installations_status_check
        CHECK (status IN ('pending_verification', 'active', 'suspended', 'removed')),
    CONSTRAINT github_app_installations_setup_action_check
        CHECK (setup_action IS NULL OR setup_action IN ('install', 'update', 'request'))
);

CREATE INDEX IF NOT EXISTS idx_github_app_installations_installed_by_user_id
    ON github_app_installations (installed_by_user_id);

CREATE INDEX IF NOT EXISTS idx_github_app_installations_status
    ON github_app_installations (status);
