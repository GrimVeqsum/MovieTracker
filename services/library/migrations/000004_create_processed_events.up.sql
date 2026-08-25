CREATE TABLE processed_events (
    consumer TEXT NOT NULL,

    event_id UUID NOT NULL,

    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,

    event_type TEXT NOT NULL,

    processed_at TIMESTAMPTZ NOT NULL
        DEFAULT NOW(),

    PRIMARY KEY (
        consumer,
        event_id
    )
);

CREATE INDEX processed_events_aggregate_idx
ON processed_events (
    aggregate_type,
    aggregate_id
);

CREATE INDEX processed_events_processed_at_idx
ON processed_events (
    processed_at
);