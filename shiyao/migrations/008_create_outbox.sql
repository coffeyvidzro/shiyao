CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    payload JSONB NOT NULL,
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0,
    publish_failures INTEGER NOT NULL DEFAULT 0,
    redrive_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    quarantined_at TIMESTAMPTZ,
    quarantine_code TEXT,
    quarantine_reason TEXT,
    locked_at TIMESTAMPTZ,
    locked_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_outbox_subject CHECK (length(trim(subject)) > 0 AND subject !~ '[[:space:]]'),
    CONSTRAINT chk_outbox_aggregate_type CHECK (length(trim(aggregate_type)) > 0),
    CONSTRAINT chk_outbox_headers_object CHECK (jsonb_typeof(headers) = 'object'),
    CONSTRAINT chk_outbox_attempts_non_negative CHECK (attempts >= 0),
    CONSTRAINT chk_outbox_publish_failures_non_negative CHECK (publish_failures >= 0),
    CONSTRAINT chk_outbox_redrive_count_non_negative CHECK (redrive_count >= 0),
    CONSTRAINT chk_outbox_quarantine_state CHECK (
        (quarantined_at IS NULL AND quarantine_code IS NULL AND quarantine_reason IS NULL)
        OR (
            quarantined_at IS NOT NULL
            AND quarantine_code IS NOT NULL
            AND length(trim(quarantine_code)) > 0
            AND quarantine_reason IS NOT NULL
            AND length(trim(quarantine_reason)) > 0
        )
    ),
    CONSTRAINT chk_outbox_terminal_state CHECK (
        NOT (published_at IS NOT NULL AND quarantined_at IS NOT NULL)
    ),
    CONSTRAINT chk_outbox_lock_pair CHECK (
        (locked_at IS NULL AND locked_by IS NULL)
        OR (locked_at IS NOT NULL AND locked_by IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_pending
    ON outbox_events (available_at, created_at)
    WHERE published_at IS NULL AND quarantined_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_outbox_events_locked
    ON outbox_events (locked_at)
    WHERE published_at IS NULL AND quarantined_at IS NULL AND locked_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_outbox_events_aggregate
    ON outbox_events (aggregate_type, aggregate_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_outbox_events_quarantined
    ON outbox_events (quarantined_at DESC)
    WHERE quarantined_at IS NOT NULL;
