DROP TABLE IF EXISTS movie_genres;
DROP TABLE IF EXISTS genres;

ALTER TABLE movies
    DROP CONSTRAINT IF EXISTS movies_metadata_status_check,
    DROP COLUMN IF EXISTS metadata_error,
    DROP COLUMN IF EXISTS metadata_status,
    DROP COLUMN IF EXISTS runtime_minutes,
    DROP COLUMN IF EXISTS poster_url,
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS original_title,
    DROP COLUMN IF EXISTS metadata_provider,
    DROP COLUMN IF EXISTS external_id;