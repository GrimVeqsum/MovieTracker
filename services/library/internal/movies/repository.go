package movies

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

// Общий интерфейс для QueryRow и Rows.
// Оба умеют Scan.
type movieScanner interface {
	Scan(dest ...any) error
}

// ВАЖНО:
// порядок полей здесь должен полностью совпадать
// с порядком колонок во всех SELECT / RETURNING ниже.
func scanMovie(
	scanner movieScanner,
	movie *Movie,
) error {
	return scanner.Scan(
		&movie.ID,
		&movie.UserID,
		&movie.Title,
		&movie.NormalizedTitle,
		&movie.ReleaseYear,

		&movie.ExternalID,
		&movie.MetadataProvider,
		&movie.OriginalTitle,
		&movie.Description,
		&movie.PosterURL,
		&movie.RuntimeMinutes,
		&movie.MetadataStatus,
		&movie.MetadataError,

		&movie.Status,
		&movie.Rating,
		&movie.Review,
		&movie.CreatedAt,
		&movie.UpdatedAt,
		&movie.WatchedAt,
		&movie.DeletedAt,
	)
}

// Загружает жанры одного фильма.
func (repo *Repository) loadGenres(
	ctx context.Context,
	movieID string,
) ([]Genre, error) {
	query := `
		SELECT
			g.id,
			g.name
		FROM genres g
		JOIN movie_genres mg
			ON mg.genre_id = g.id
		WHERE mg.movie_id = $1
		ORDER BY g.name
	`

	rows, err := repo.db.Query(
		ctx,
		query,
		movieID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	genres := make([]Genre, 0)

	for rows.Next() {
		var genre Genre

		if err := rows.Scan(
			&genre.ID,
			&genre.Name,
		); err != nil {
			return nil, err
		}

		genres = append(
			genres,
			genre,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return genres, nil
}

// Create movie

type CreateMovieParams struct {
	UserID          string
	Title           string
	NormalizedTitle string
	ReleaseYear     *int
}

func (repo *Repository) Create(
	ctx context.Context,
	params CreateMovieParams,
) (*Movie, error) {
	query := `
		INSERT INTO movies (
			id,
			user_id,
			title,
			normalized_title,
			release_year
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING
			id,
			user_id,
			title,
			normalized_title,
			release_year,

			external_id,
			metadata_provider,
			original_title,
			description,
			poster_url,
			runtime_minutes,
			metadata_status,
			metadata_error,

			status,
			rating,
			review,
			created_at,
			updated_at,
			watched_at,
			deleted_at
	`

	id := uuid.NewString()

	queryCtx, cancel := context.WithTimeout(
		ctx,
		3*time.Second,
	)
	defer cancel()

	var movie Movie

	row := repo.db.QueryRow(
		queryCtx,
		query,
		id,
		params.UserID,
		params.Title,
		params.NormalizedTitle,
		params.ReleaseYear,
	)

	err := scanMovie(
		row,
		&movie,
	)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" {
				return nil, ErrMovieAlreadyExists
			}
		}

		return nil, err
	}

	movie.Genres = make([]Genre, 0)

	return &movie, nil
}

// List movies

type ListMovieParams struct {
	UserID string
}

func (repo *Repository) List(
	ctx context.Context,
	params ListMovieParams,
) ([]Movie, error) {
	query := `
		SELECT
			id,
			user_id,
			title,
			normalized_title,
			release_year,

			external_id,
			metadata_provider,
			original_title,
			description,
			poster_url,
			runtime_minutes,
			metadata_status,
			metadata_error,

			status,
			rating,
			review,
			created_at,
			updated_at,
			watched_at,
			deleted_at
		FROM movies
		WHERE user_id = $1
		  AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	queryCtx, cancel := context.WithTimeout(
		ctx,
		3*time.Second,
	)
	defer cancel()

	rows, err := repo.db.Query(
		queryCtx,
		query,
		params.UserID,
	)
	if err != nil {
		return nil, err
	}

	movieList := make([]Movie, 0)

	for rows.Next() {
		var movie Movie

		if err := scanMovie(
			rows,
			&movie,
		); err != nil {
			rows.Close()
			return nil, err
		}

		movieList = append(
			movieList,
			movie,
		)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}

	// Закрываем rows до дополнительных запросов жанров.
	rows.Close()

	for i := range movieList {
		genres, err := repo.loadGenres(
			queryCtx,
			movieList[i].ID,
		)
		if err != nil {
			return nil, err
		}

		movieList[i].Genres = genres
	}

	return movieList, nil
}

// Delete movie

type DeleteMovieParams struct {
	ID     string
	UserID string
}

func (repo *Repository) Delete(
	ctx context.Context,
	params DeleteMovieParams,
) error {
	query := `
		UPDATE movies
		SET
			deleted_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
		  AND user_id = $2
		  AND deleted_at IS NULL
	`

	queryCtx, cancel := context.WithTimeout(
		ctx,
		3*time.Second,
	)
	defer cancel()

	result, err := repo.db.Exec(
		queryCtx,
		query,
		params.ID,
		params.UserID,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrMovieNotFound
	}

	return nil
}

// Get movie by id

type GetOneParams struct {
	UserID string
	ID     string
}

func (repo *Repository) GetOne(
	ctx context.Context,
	params GetOneParams,
) (*Movie, error) {
	query := `
		SELECT
			id,
			user_id,
			title,
			normalized_title,
			release_year,

			external_id,
			metadata_provider,
			original_title,
			description,
			poster_url,
			runtime_minutes,
			metadata_status,
			metadata_error,

			status,
			rating,
			review,
			created_at,
			updated_at,
			watched_at,
			deleted_at
		FROM movies
		WHERE id = $1
		  AND user_id = $2
		  AND deleted_at IS NULL
	`

	queryCtx, cancel := context.WithTimeout(
		ctx,
		3*time.Second,
	)
	defer cancel()

	var movie Movie

	row := repo.db.QueryRow(
		queryCtx,
		query,
		params.ID,
		params.UserID,
	)

	err := scanMovie(
		row,
		&movie,
	)
	if err != nil {
		if errors.Is(
			err,
			pgx.ErrNoRows,
		) {
			return nil, ErrMovieNotFound
		}

		return nil, err
	}

	genres, err := repo.loadGenres(
		queryCtx,
		movie.ID,
	)
	if err != nil {
		return nil, err
	}

	movie.Genres = genres

	return &movie, nil
}

// Update movie status

type UpdateMovieStatusParams struct {
	ID     string
	UserID string
	Status string
	Rating *int
	Review *string
}

func (repo *Repository) UpdateStatus(
	ctx context.Context,
	params UpdateMovieStatusParams,
) (*Movie, error) {
	query := `
		UPDATE movies
		SET
			status = $1,
			rating = $2,
			review = $3,
			updated_at = NOW(),
			watched_at = CASE
				WHEN $1 = 'watched' THEN NOW()
				ELSE NULL
			END
		WHERE id = $4
		  AND user_id = $5
		  AND deleted_at IS NULL
		RETURNING
			id,
			user_id,
			title,
			normalized_title,
			release_year,

			external_id,
			metadata_provider,
			original_title,
			description,
			poster_url,
			runtime_minutes,
			metadata_status,
			metadata_error,

			status,
			rating,
			review,
			created_at,
			updated_at,
			watched_at,
			deleted_at
	`

	queryCtx, cancel := context.WithTimeout(
		ctx,
		3*time.Second,
	)
	defer cancel()

	var movie Movie

	row := repo.db.QueryRow(
		queryCtx,
		query,
		params.Status,
		params.Rating,
		params.Review,
		params.ID,
		params.UserID,
	)

	err := scanMovie(
		row,
		&movie,
	)
	if err != nil {
		if errors.Is(
			err,
			pgx.ErrNoRows,
		) {
			return nil, ErrMovieNotFound
		}

		return nil, err
	}

	genres, err := repo.loadGenres(
		queryCtx,
		movie.ID,
	)
	if err != nil {
		return nil, err
	}

	movie.Genres = genres

	return &movie, nil
}

// Get random movie

type GetRandomMovieParams struct {
	UserID string
}

func (repo *Repository) GetRandom(
	ctx context.Context,
	params GetRandomMovieParams,
) (*Movie, error) {
	query := `
		SELECT
			id,
			user_id,
			title,
			normalized_title,
			release_year,

			external_id,
			metadata_provider,
			original_title,
			description,
			poster_url,
			runtime_minutes,
			metadata_status,
			metadata_error,

			status,
			rating,
			review,
			created_at,
			updated_at,
			watched_at,
			deleted_at
		FROM movies
		WHERE user_id = $1
		  AND deleted_at IS NULL
		ORDER BY random()
		LIMIT 1
	`

	queryCtx, cancel := context.WithTimeout(
		ctx,
		3*time.Second,
	)
	defer cancel()

	var movie Movie

	row := repo.db.QueryRow(
		queryCtx,
		query,
		params.UserID,
	)

	err := scanMovie(
		row,
		&movie,
	)
	if err != nil {
		if errors.Is(
			err,
			pgx.ErrNoRows,
		) {
			return nil, ErrMovieNotFound
		}

		return nil, err
	}

	genres, err := repo.loadGenres(
		queryCtx,
		movie.ID,
	)
	if err != nil {
		return nil, err
	}

	movie.Genres = genres

	return &movie, nil
}

// Update metadata

type UpdateMetadataParams struct {
	ID               string
	UserID           string
	ExternalID       string
	MetadataProvider string
	OriginalTitle    string
	Description      string
	ReleaseYear      int
	PosterURL        string
	RuntimeMinutes   *int
	Genres           []string
}

func (repo *Repository) UpdateMetadata(
	ctx context.Context,
	params UpdateMetadataParams,
) error {
	queryCtx, cancel := context.WithTimeout(
		ctx,
		5*time.Second,
	)
	defer cancel()

	tx, err := repo.db.Begin(queryCtx)
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback(queryCtx)
	}()

	result, err := tx.Exec(
		queryCtx,
		`
		UPDATE movies
		SET
			external_id = $1,
			metadata_provider = $2,
			original_title = $3,
			description = $4,
			release_year = $5,
			poster_url = $6,
			runtime_minutes = $7,
			metadata_status = 'ready',
			metadata_error = NULL,
			updated_at = NOW()
		WHERE id = $8
		  AND user_id = $9
		  AND deleted_at IS NULL
		`,
		params.ExternalID,
		params.MetadataProvider,
		params.OriginalTitle,
		params.Description,
		params.ReleaseYear,
		params.PosterURL,
		params.RuntimeMinutes,
		params.ID,
		params.UserID,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrMovieNotFound
	}

	_, err = tx.Exec(
		queryCtx,
		`
		DELETE FROM movie_genres
		WHERE movie_id = $1
		`,
		params.ID,
	)
	if err != nil {
		return err
	}

	for _, genreName := range params.Genres {
		genreName = strings.TrimSpace(
			genreName,
		)

		if genreName == "" {
			continue
		}

		var genreID int

		err = tx.QueryRow(
			queryCtx,
			`
			INSERT INTO genres (name)
			VALUES ($1)
			ON CONFLICT (name)
			DO UPDATE SET name = EXCLUDED.name
			RETURNING id
			`,
			genreName,
		).Scan(
			&genreID,
		)
		if err != nil {
			return err
		}

		_, err = tx.Exec(
			queryCtx,
			`
			INSERT INTO movie_genres (
				movie_id,
				genre_id
			)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
			`,
			params.ID,
			genreID,
		)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(
		queryCtx,
	); err != nil {
		return err
	}

	return nil
}
