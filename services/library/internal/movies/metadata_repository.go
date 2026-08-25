package movies

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const enrichmentConsumerName = "enrichment"

type UpdateMetadataIdempotentParams struct {
	EventID string

	ID     string
	UserID string

	ExternalID       string
	MetadataProvider string
	OriginalTitle    string
	Description      string
	ReleaseYear      int
	PosterURL        string
	RuntimeMinutes   *int
	Genres           []string
}

func (repo *Repository) UpdateMetadataIdempotent(
	ctx context.Context,
	params UpdateMetadataIdempotentParams,
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
			"begin metadata transaction: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(
			queryCtx,
		)
	}()

	isNewEvent, err :=
		claimProcessedEvent(
			queryCtx,
			tx,
			params.EventID,
			params.ID,
			"MovieCreated",
		)

	if err != nil {
		return err
	}

	// Событие уже успешно обрабатывалось раньше.
	// Повторно metadata не записываем.
	if !isNewEvent {
		if err :=
			tx.Commit(
				queryCtx,
			); err != nil {

			return fmt.Errorf(
				"commit duplicate metadata event: %w",
				err,
			)
		}

		return nil
	}

	result, err :=
		tx.Exec(
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
		return fmt.Errorf(
			"update movie metadata: %w",
			err,
		)
	}

	if result.RowsAffected() == 0 {
		return ErrMovieNotFound
	}

	_, err =
		tx.Exec(
			queryCtx,
			`
			DELETE FROM movie_genres
			WHERE movie_id = $1
			`,
			params.ID,
		)

	if err != nil {
		return fmt.Errorf(
			"delete old movie genres: %w",
			err,
		)
	}

	for _, genreName := range params.Genres {

		genreName =
			strings.TrimSpace(
				genreName,
			)

		if genreName == "" {
			continue
		}

		var genreID int

		err =
			tx.QueryRow(
				queryCtx,
				`
				INSERT INTO genres (
					name
				)
				VALUES ($1)
				ON CONFLICT (name)
				DO UPDATE SET
					name = EXCLUDED.name
				RETURNING id
				`,
				genreName,
			).Scan(
				&genreID,
			)

		if err != nil {
			return fmt.Errorf(
				"upsert genre %q: %w",
				genreName,
				err,
			)
		}

		_, err =
			tx.Exec(
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
			return fmt.Errorf(
				"link movie genre: %w",
				err,
			)
		}
	}

	if err :=
		tx.Commit(
			queryCtx,
		); err != nil {

		return fmt.Errorf(
			"commit metadata transaction: %w",
			err,
		)
	}

	return nil
}

type MarkMetadataFailedParams struct {
	EventID string

	ID     string
	UserID string

	Error string
}

func (repo *Repository) MarkMetadataFailed(
	ctx context.Context,
	params MarkMetadataFailedParams,
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
			"begin metadata failure transaction: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(
			queryCtx,
		)
	}()

	var metadataStatus string

	err =
		tx.QueryRow(
			queryCtx,
			`
			SELECT metadata_status
			FROM movies
			WHERE id = $1
			  AND user_id = $2
			  AND deleted_at IS NULL
			FOR UPDATE
			`,
			params.ID,
			params.UserID,
		).Scan(
			&metadataStatus,
		)

	if err != nil {
		if errors.Is(
			err,
			pgx.ErrNoRows,
		) {
			return ErrMovieNotFound
		}

		return fmt.Errorf(
			"get movie metadata status: %w",
			err,
		)
	}

	// Самое важное правило:
	//
	// если metadata уже была успешно сохранена,
	// более поздняя повторная ошибка не должна
	// переводить фильм из ready обратно в failed.
	if metadataStatus == "ready" {
		if err :=
			tx.Commit(
				queryCtx,
			); err != nil {

			return fmt.Errorf(
				"commit ignored metadata failure: %w",
				err,
			)
		}

		return nil
	}

	var alreadyProcessed bool

	err =
		tx.QueryRow(
			queryCtx,
			`
			SELECT EXISTS (
				SELECT 1
				FROM processed_events
				WHERE consumer = $1
				  AND event_id = $2
			)
			`,
			enrichmentConsumerName,
			params.EventID,
		).Scan(
			&alreadyProcessed,
		)

	if err != nil {
		return fmt.Errorf(
			"check processed metadata event: %w",
			err,
		)
	}

	// Защитный случай:
	// если этот event_id уже отмечен успешно обработанным,
	// никакой failed статус не записываем.
	if alreadyProcessed {
		if err :=
			tx.Commit(
				queryCtx,
			); err != nil {

			return fmt.Errorf(
				"commit duplicate metadata failure: %w",
				err,
			)
		}

		return nil
	}

	metadataError :=
		truncateMetadataError(
			params.Error,
		)

	_, err =
		tx.Exec(
			queryCtx,
			`
			UPDATE movies
			SET
				metadata_status = 'failed',
				metadata_error = $1,
				updated_at = NOW()
			WHERE id = $2
			  AND user_id = $3
			  AND deleted_at IS NULL
			  AND metadata_status <> 'ready'
			`,
			metadataError,
			params.ID,
			params.UserID,
		)

	if err != nil {
		return fmt.Errorf(
			"mark metadata failed: %w",
			err,
		)
	}

	if err :=
		tx.Commit(
			queryCtx,
		); err != nil {

		return fmt.Errorf(
			"commit metadata failure transaction: %w",
			err,
		)
	}

	return nil
}

func claimProcessedEvent(
	ctx context.Context,
	tx pgx.Tx,
	eventID string,
	aggregateID string,
	eventType string,
) (bool, error) {
	result, err :=
		tx.Exec(
			ctx,
			`
			INSERT INTO processed_events (
				consumer,
				event_id,
				aggregate_type,
				aggregate_id,
				event_type
			)
			VALUES (
				$1,
				$2,
				$3,
				$4,
				$5
			)
			ON CONFLICT (
				consumer,
				event_id
			)
			DO NOTHING
			`,
			enrichmentConsumerName,
			eventID,
			"movie",
			aggregateID,
			eventType,
		)

	if err != nil {
		return false,
			fmt.Errorf(
				"claim processed event: %w",
				err,
			)
	}

	return result.RowsAffected() == 1,
		nil
}

func truncateMetadataError(
	value string,
) string {
	value =
		strings.TrimSpace(
			value,
		)

	if value == "" {
		return "metadata enrichment failed"
	}

	const maxRunes = 2000

	runes :=
		[]rune(
			value,
		)

	if len(runes) <= maxRunes {
		return value
	}

	return string(
		runes[:maxRunes],
	)
}
