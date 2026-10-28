-- Keep a non-secret hash -> session association after token rotation so logout
-- with an older cookie still revokes the same session during concurrent refresh.
CREATE TABLE refresh_session_tokens (
    token_hash BYTEA PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES refresh_sessions(id) ON DELETE CASCADE
);

CREATE INDEX refresh_session_tokens_session_id_idx
ON refresh_session_tokens (session_id);

INSERT INTO refresh_session_tokens (token_hash, session_id)
SELECT refresh_token_hash, id FROM refresh_sessions;
