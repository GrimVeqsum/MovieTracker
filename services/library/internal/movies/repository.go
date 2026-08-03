package movies

import (
	"context"
	"errors"
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

// create movie

type CreateMovieParams struct {
	UserID          string
	Title           string
	NormalizedTitle string
	ReleaseYear     *int
}

func (repo *Repository) Create(ctx context.Context, params CreateMovieParams) (*Movie, error) {

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
			status,
			rating,
			review,
			created_at,
			updated_at,
			watched_at,
			deleted_at
	`
	id := uuid.NewString()
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var movie Movie

	err := repo.db.QueryRow(
		queryCtx,
		query,
		id,
		params.UserID,
		params.Title,
		params.NormalizedTitle,
		params.ReleaseYear,
	).Scan(
		&movie.ID,
		&movie.UserID,
		&movie.Title,
		&movie.NormalizedTitle,
		&movie.ReleaseYear,
		&movie.Status,
		&movie.Rating,
		&movie.Review,
		&movie.CreatedAt,
		&movie.UpdatedAt,
		&movie.WatchedAt,
		&movie.DeletedAt,
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

	return &movie, nil
}

// get movies
type ListMovieParams struct {
	UserID string
}

func (repo *Repository) List(ctx context.Context, params ListMovieParams) ([]Movie, error) {
	query := `SELECT
	id,
	user_id,
	title,
	normalized_title,
	release_year,
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
ORDER BY created_at DESC`

	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := repo.db.Query(queryCtx, query, params.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	movies := make([]Movie, 0)

	for rows.Next() {
		var movie Movie
		err := rows.Scan(&movie.ID,
			&movie.UserID,
			&movie.Title,
			&movie.NormalizedTitle,
			&movie.ReleaseYear,
			&movie.Status,
			&movie.Rating,
			&movie.Review,
			&movie.CreatedAt,
			&movie.UpdatedAt,
			&movie.WatchedAt,
			&movie.DeletedAt)
		if err != nil {
			return nil, err
		}

		movies = append(movies, movie)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return movies, nil
}

//delete movie

type DeleteMovieParams struct {
	ID     string
	UserID string
}

func (repo *Repository) Delete(ctx context.Context, params DeleteMovieParams) error {
	query := `UPDATE movies
	SET
	deleted_at = NOW(),
	updated_at = NOW()
	WHERE id = $1
  AND user_id = $2
  AND deleted_at IS NULL
	`
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result, err := repo.db.Exec(queryCtx, query, params.ID, params.UserID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrMovieNotFound
	}

	return nil
}

// get movie by id
type GetOneParams struct {
	UserID string
	ID     string
}

func (repo *Repository) GetOne(ctx context.Context, params GetOneParams) (*Movie, error) {
	query := `SELECT 
	id,
	user_id,
	title,
	normalized_title,
	release_year,
	status,
	rating,
	review,
	created_at,
	updated_at,
	watched_at,
	deleted_at
	FROM movies
	WHERE id=$1 
	AND user_id=$2
	AND deleted_at IS NULL
	`
	var movie Movie

	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	err := repo.db.QueryRow(
		queryCtx,
		query,
		params.ID,
		params.UserID,
	).Scan(
		&movie.ID,
		&movie.UserID,
		&movie.Title,
		&movie.NormalizedTitle,
		&movie.ReleaseYear,
		&movie.Status,
		&movie.Rating,
		&movie.Review,
		&movie.CreatedAt,
		&movie.UpdatedAt,
		&movie.WatchedAt,
		&movie.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMovieNotFound
		}

		return nil, err
	}

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

func (repo *Repository) UpdateStatus(ctx context.Context, params UpdateMovieStatusParams) (*Movie, error) {

	query := `UPDATE movies
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
	RETURNING id,
	user_id,
	title,
	normalized_title,
	release_year,
	status,
	rating,
	review,
	created_at,
	updated_at,
	watched_at,
	deleted_at
	`
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var movie Movie

	err := repo.db.QueryRow(
		queryCtx,
		query,
		params.Status,
		params.Rating,
		params.Review,
		params.ID,
		params.UserID,
	).Scan(
		&movie.ID,
		&movie.UserID,
		&movie.Title,
		&movie.NormalizedTitle,
		&movie.ReleaseYear,
		&movie.Status,
		&movie.Rating,
		&movie.Review,
		&movie.CreatedAt,
		&movie.UpdatedAt,
		&movie.WatchedAt,
		&movie.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMovieNotFound
		}

		return nil, err
	}

	return &movie, nil
}

// Get random movie
type GetRandomMovieParams struct {
	UserID string
}

func (repo *Repository) GetRandom(ctx context.Context, params GetRandomMovieParams) (*Movie, error) {
	query := `
		SELECT
			id,
			user_id,
			title,
			normalized_title,
			release_year,
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

	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var movie Movie

	err := repo.db.QueryRow(
		queryCtx,
		query,
		params.UserID,
	).Scan(
		&movie.ID,
		&movie.UserID,
		&movie.Title,
		&movie.NormalizedTitle,
		&movie.ReleaseYear,
		&movie.Status,
		&movie.Rating,
		&movie.Review,
		&movie.CreatedAt,
		&movie.UpdatedAt,
		&movie.WatchedAt,
		&movie.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMovieNotFound
		}

		return nil, err
	}

	return &movie, nil
}
