CREATE TABLE refresh_sessions (
    id UUID PRIMARY KEY,

    user_id UUID NOT NULL
        REFERENCES users(id)
        ON DELETE CASCADE,

    refresh_token_hash BYTEA NOT NULL UNIQUE,

    expires_at TIMESTAMPTZ NOT NULL,

    created_at TIMESTAMPTZ NOT NULL
        DEFAULT NOW(),

    last_used_at TIMESTAMPTZ,

    revoked_at TIMESTAMPTZ
);

CREATE INDEX refresh_sessions_user_active_idx
ON refresh_sessions (
    user_id,
    expires_at
)
WHERE revoked_at IS NULL;

CREATE INDEX refresh_sessions_expires_at_idx
ON refresh_sessions (
    expires_at
);