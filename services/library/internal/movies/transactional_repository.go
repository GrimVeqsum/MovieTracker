package movies

import (
	"context"
	"errors"
	"fmt"
	"time"

	"movie-platform/library/internal/outbox"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionalRepository struct {
	db *pgxpool.Pool

	outbox *outbox.Repository
}

func NewTransactionalRepository(
	db *pgxpool.Pool,
	outboxRepository *outbox.Repository,
) *TransactionalRepository {
	return &TransactionalRepository{
		db: db,

		outbox: outboxRepository,
	}
}

type CreateMovieWithEventParams struct {
	ID string

	UserID string

	Title string

	NormalizedTitle string

	ReleaseYear *int
}

func (repo *TransactionalRepository) Create(
	ctx context.Context,
	params CreateMovieWithEventParams,
	event Event,
) (*Movie, error) {
	queryCtx, cancel :=
		context.WithTimeout(
			ctx,
			5*time.Second,
		)

	defer cancel()

	tx, err :=
		repo.db.Begin(
			queryCtx,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"begin create movie transaction: %w",
				err,
			)
	}

	defer func() {
		_ = tx.Rollback(
			queryCtx,
		)
	}()

	var movie Movie

	row :=
		tx.QueryRow(
			queryCtx,
			`
			INSERT INTO movies (
				id,
				user_id,
				title,
				normalized_title,
				release_year
			)
			VALUES (
				$1,
				$2,
				$3,
				$4,
				$5
			)
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
			`,
			params.ID,
			params.UserID,
			params.Title,
			params.NormalizedTitle,
			params.ReleaseYear,
		)

	err =
		scanMovie(
			row,
			&movie,
		)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(
			err,
			&pgErr,
		) &&
			pgErr.Code == "23505" {

			return nil,
				ErrMovieAlreadyExists
		}

		return nil, err
	}

	movie.Genres =
		make(
			[]Genre,
			0,
		)

	err =
		repo.insertMovieEvent(
			queryCtx,
			tx,
			event,
		)

	if err != nil {
		return nil, err
	}

	err =
		tx.Commit(
			queryCtx,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"commit create movie transaction: %w",
				err,
			)
	}

	return &movie, nil
}

func (repo *TransactionalRepository) Delete(
	ctx context.Context,
	params DeleteMovieParams,
	event Event,
) error {
	queryCtx, cancel :=
		context.WithTimeout(
			ctx,
			5*time.Second,
		)

	defer cancel()

	tx, err :=
		repo.db.Begin(
			queryCtx,
		)

	if err != nil {
		return fmt.Errorf(
			"begin delete movie transaction: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(
			queryCtx,
		)
	}()

	result, err :=
		tx.Exec(
			queryCtx,
			`
			UPDATE movies
			SET
				deleted_at = NOW(),
				updated_at = NOW()
			WHERE id = $1
			  AND user_id = $2
			  AND deleted_at IS NULL
			`,
			params.ID,
			params.UserID,
		)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrMovieNotFound
	}

	err =
		repo.insertMovieEvent(
			queryCtx,
			tx,
			event,
		)

	if err != nil {
		return err
	}

	err =
		tx.Commit(
			queryCtx,
		)

	if err != nil {
		return fmt.Errorf(
			"commit delete movie transaction: %w",
			err,
		)
	}

	return nil
}

func (repo *TransactionalRepository) UpdateStatus(
	ctx context.Context,
	params UpdateMovieStatusParams,
	event Event,
) (*Movie, error) {
	queryCtx, cancel :=
		context.WithTimeout(
			ctx,
			5*time.Second,
		)

	defer cancel()

	tx, err :=
		repo.db.Begin(
			queryCtx,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"begin update movie status transaction: %w",
				err,
			)
	}

	defer func() {
		_ = tx.Rollback(
			queryCtx,
		)
	}()

	var movie Movie

	row :=
		tx.QueryRow(
			queryCtx,
			`
			UPDATE movies
			SET
				status = $1,
				rating = $2,
				review = $3,
				updated_at = NOW(),
				watched_at = CASE
					WHEN $1 = 'watched'
						THEN NOW()
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
			`,
			params.Status,
			params.Rating,
			params.Review,
			params.ID,
			params.UserID,
		)

	err =
		scanMovie(
			row,
			&movie,
		)

	if err != nil {
		if errors.Is(
			err,
			pgx.ErrNoRows,
		) {
			return nil,
				ErrMovieNotFound
		}

		return nil, err
	}

	genres, err :=
		loadGenresTx(
			queryCtx,
			tx,
			movie.ID,
		)

	if err != nil {
		return nil, err
	}

	movie.Genres =
		genres

	err =
		repo.insertMovieEvent(
			queryCtx,
			tx,
			event,
		)

	if err != nil {
		return nil, err
	}

	err =
		tx.Commit(
			queryCtx,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"commit update movie status transaction: %w",
				err,
			)
	}

	return &movie, nil
}

func (repo *TransactionalRepository) insertMovieEvent(
	ctx context.Context,
	tx pgx.Tx,
	event Event,
) error {
	return repo.outbox.InsertTx(
		ctx,
		tx,
		outbox.InsertParams{
			ID: event.EventID,

			AggregateType: "movie",

			AggregateID: event.MovieID,

			EventType: event.Type,

			Payload: event,

			OccurredAt: event.OccurredAt,
		},
	)
}

func loadGenresTx(
	ctx context.Context,
	tx pgx.Tx,
	movieID string,
) ([]Genre, error) {
	rows, err :=
		tx.Query(
			ctx,
			`
			SELECT
				g.id,
				g.name
			FROM genres g
			JOIN movie_genres mg
				ON mg.genre_id = g.id
			WHERE mg.movie_id = $1
			ORDER BY g.name
			`,
			movieID,
		)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	genres :=
		make(
			[]Genre,
			0,
		)

	for rows.Next() {
		var genre Genre

		err :=
			rows.Scan(
				&genre.ID,
				&genre.Name,
			)

		if err != nil {
			return nil, err
		}

		genres =
			append(
				genres,
				genre,
			)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return genres, nil
}
