CREATE TABLE movies (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    title TEXT NOT NULL,
    normalized_title TEXT NOT NULL,
    release_year INTEGER,
    status TEXT NOT NULL DEFAULT 'unwatched',
    rating INTEGER,
    review TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    watched_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT movies_status_check CHECK (status IN ('unwatched', 'watched')),
    CONSTRAINT movies_rating_check CHECK (rating IS NULL OR rating BETWEEN 1 AND 10)
);

CREATE UNIQUE INDEX movies_user_normalized_title_year_active_unique
ON movies (user_id, normalized_title, release_year) NULLS NOT DISTINCT
WHERE deleted_at IS NULL;