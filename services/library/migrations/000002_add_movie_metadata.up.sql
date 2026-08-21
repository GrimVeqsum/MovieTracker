ALTER TABLE movies
    ADD COLUMN external_id TEXT,
    ADD COLUMN metadata_provider TEXT,
    ADD COLUMN original_title TEXT,
    ADD COLUMN description TEXT,
    ADD COLUMN poster_url TEXT,
    ADD COLUMN runtime_minutes INTEGER,
    ADD COLUMN metadata_status TEXT NOT NULL DEFAULT 'pending',
    ADD COLUMN metadata_error TEXT;

ALTER TABLE movies
    ADD CONSTRAINT movies_metadata_status_check
    CHECK (metadata_status IN ('pending', 'processing', 'ready', 'failed'));

CREATE TABLE genres (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE movie_genres (
    movie_id UUID NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    genre_id INTEGER NOT NULL REFERENCES genres(id) ON DELETE CASCADE,

    PRIMARY KEY (movie_id, genre_id)
);

CREATE INDEX movie_genres_movie_id_idx
    ON movie_genres(movie_id);

CREATE INDEX movie_genres_genre_id_idx
    ON movie_genres(genre_id);