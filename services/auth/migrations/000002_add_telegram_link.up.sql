ALTER TABLE users
ADD COLUMN telegram_user_id BIGINT;

CREATE UNIQUE INDEX users_telegram_user_id_unique_idx
ON users (telegram_user_id)
WHERE telegram_user_id IS NOT NULL;


CREATE TABLE telegram_link_codes (
    user_id UUID PRIMARY KEY
        REFERENCES users(id)
        ON DELETE CASCADE,

    code_hash BYTEA NOT NULL UNIQUE,

    expires_at TIMESTAMPTZ NOT NULL,

    created_at TIMESTAMPTZ NOT NULL
        DEFAULT NOW()
);

CREATE INDEX telegram_link_codes_expires_at_idx
ON telegram_link_codes (expires_at);