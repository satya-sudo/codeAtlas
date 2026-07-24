ALTER TABLE repository_sync_runs
    ADD COLUMN IF NOT EXISTS trigger_source VARCHAR(32) NOT NULL DEFAULT 'manual',
    ADD COLUMN IF NOT EXISTS trigger_delivery_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS trigger_ref VARCHAR(255),
    ADD COLUMN IF NOT EXISTS before_sha VARCHAR(64),
    ADD COLUMN IF NOT EXISTS after_sha VARCHAR(64);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'repository_sync_runs_trigger_source_check'
    ) THEN
        ALTER TABLE repository_sync_runs
            ADD CONSTRAINT repository_sync_runs_trigger_source_check
            CHECK (trigger_source IN ('manual', 'webhook'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_repository_sync_runs_trigger_source
    ON repository_sync_runs (trigger_source);

CREATE INDEX IF NOT EXISTS idx_repository_sync_runs_trigger_delivery_id
    ON repository_sync_runs (trigger_delivery_id);
