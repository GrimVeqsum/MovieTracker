CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,

    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,

    event_type TEXT NOT NULL,

    payload JSONB NOT NULL,

    occurred_at TIMESTAMPTZ NOT NULL,

    created_at TIMESTAMPTZ NOT NULL
        DEFAULT NOW(),

    published_at TIMESTAMPTZ,

    attempts INTEGER NOT NULL
        DEFAULT 0,

    next_attempt_at TIMESTAMPTZ NOT NULL
        DEFAULT NOW(),

    last_error TEXT,

    locked_at TIMESTAMPTZ,

    lock_id UUID,

    CONSTRAINT outbox_events_attempts_check
        CHECK (attempts >= 0)
);

CREATE INDEX outbox_events_pending_idx
ON outbox_events (
    next_attempt_at,
    created_at
)
WHERE published_at IS NULL;

CREATE INDEX outbox_events_aggregate_idx
ON outbox_events (
    aggregate_type,
    aggregate_id
);