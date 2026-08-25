package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(
	db *pgxpool.Pool,
) *Repository {
	return &Repository{
		db: db,
	}
}

type InsertParams struct {
	ID string

	AggregateType string
	AggregateID   string

	EventType string

	Payload any

	OccurredAt time.Time
}

func (repo *Repository) InsertTx(
	ctx context.Context,
	tx pgx.Tx,
	params InsertParams,
) error {
	payload, err :=
		json.Marshal(
			params.Payload,
		)

	if err != nil {
		return fmt.Errorf(
			"marshal outbox payload: %w",
			err,
		)
	}

	_, err =
		tx.Exec(
			ctx,
			`
			INSERT INTO outbox_events (
				id,
				aggregate_type,
				aggregate_id,
				event_type,
				payload,
				occurred_at
			)
			VALUES (
				$1,
				$2,
				$3,
				$4,
				$5::jsonb,
				$6
			)
			`,
			params.ID,
			params.AggregateType,
			params.AggregateID,
			params.EventType,
			string(payload),
			params.OccurredAt,
		)

	if err != nil {
		return fmt.Errorf(
			"insert outbox event: %w",
			err,
		)
	}

	return nil
}

type Message struct {
	ID string

	AggregateID string

	EventType string

	Payload []byte

	OccurredAt time.Time

	Attempts int
}

func (repo *Repository) ClaimBatch(
	ctx context.Context,
	lockID string,
	limit int,
) ([]Message, error) {
	if limit <= 0 {
		return []Message{}, nil
	}

	queryCtx, cancel :=
		context.WithTimeout(
			ctx,
			3*time.Second,
		)

	defer cancel()

	tx, err :=
		repo.db.Begin(
			queryCtx,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"begin outbox claim transaction: %w",
				err,
			)
	}

	defer func() {
		_ = tx.Rollback(
			queryCtx,
		)
	}()

	rows, err :=
		tx.Query(
			queryCtx,
			`
			WITH candidates AS (
				SELECT id
				FROM outbox_events
				WHERE published_at IS NULL
				  AND next_attempt_at <= NOW()
				  AND (
				      locked_at IS NULL
				      OR locked_at <
				         NOW() - INTERVAL '1 minute'
				  )
				ORDER BY created_at
				LIMIT $1
				FOR UPDATE SKIP LOCKED
			)
			UPDATE outbox_events AS event
			SET
				locked_at = NOW(),
				lock_id = $2,
				attempts = event.attempts + 1
			FROM candidates
			WHERE event.id = candidates.id
			RETURNING
				event.id,
				event.aggregate_id,
				event.event_type,
				event.payload,
				event.occurred_at,
				event.attempts
			`,
			limit,
			lockID,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"claim outbox events: %w",
				err,
			)
	}

	defer rows.Close()

	messages :=
		make(
			[]Message,
			0,
			limit,
		)

	for rows.Next() {
		var message Message

		err =
			rows.Scan(
				&message.ID,
				&message.AggregateID,
				&message.EventType,
				&message.Payload,
				&message.OccurredAt,
				&message.Attempts,
			)

		if err != nil {
			return nil,
				fmt.Errorf(
					"scan outbox event: %w",
					err,
				)
		}

		messages =
			append(
				messages,
				message,
			)
	}

	if err := rows.Err(); err != nil {
		return nil,
			fmt.Errorf(
				"iterate outbox events: %w",
				err,
			)
	}

	if err :=
		tx.Commit(
			queryCtx,
		); err != nil {

		return nil,
			fmt.Errorf(
				"commit outbox claim transaction: %w",
				err,
			)
	}

	return messages, nil
}

func (repo *Repository) MarkPublished(
	ctx context.Context,
	eventID string,
	lockID string,
) error {
	queryCtx, cancel :=
		context.WithTimeout(
			ctx,
			3*time.Second,
		)

	defer cancel()

	result, err :=
		repo.db.Exec(
			queryCtx,
			`
			UPDATE outbox_events
			SET
				published_at = NOW(),
				locked_at = NULL,
				lock_id = NULL,
				last_error = NULL
			WHERE id = $1
			  AND lock_id = $2
			  AND published_at IS NULL
			`,
			eventID,
			lockID,
		)

	if err != nil {
		return fmt.Errorf(
			"mark outbox event published: %w",
			err,
		)
	}

	if result.RowsAffected() != 1 {
		return errors.New(
			"outbox event publish lock was lost",
		)
	}

	return nil
}

func (repo *Repository) MarkFailed(
	ctx context.Context,
	eventID string,
	lockID string,
	attempt int,
	publishErr error,
) error {
	queryCtx, cancel :=
		context.WithTimeout(
			ctx,
			3*time.Second,
		)

	defer cancel()

	delay :=
		retryDelay(
			attempt,
		)

	errorText :=
		truncateError(
			publishErr,
		)

	result, err :=
		repo.db.Exec(
			queryCtx,
			`
			UPDATE outbox_events
			SET
				locked_at = NULL,
				lock_id = NULL,
				last_error = $3,
				next_attempt_at =
					NOW() +
					($4 * INTERVAL '1 second')
			WHERE id = $1
			  AND lock_id = $2
			  AND published_at IS NULL
			`,
			eventID,
			lockID,
			errorText,
			int64(
				delay/time.Second,
			),
		)

	if err != nil {
		return fmt.Errorf(
			"mark outbox event failed: %w",
			err,
		)
	}

	if result.RowsAffected() != 1 {
		return errors.New(
			"outbox event failure lock was lost",
		)
	}

	return nil
}

func retryDelay(
	attempt int,
) time.Duration {
	if attempt <= 1 {
		return time.Second
	}

	shift :=
		attempt - 1

	if shift > 6 {
		shift = 6
	}

	delay :=
		time.Second *
			time.Duration(
				1<<shift,
			)

	if delay > time.Minute {
		return time.Minute
	}

	return delay
}

func truncateError(
	err error,
) string {
	if err == nil {
		return ""
	}

	const maxRunes = 2000

	runes :=
		[]rune(
			err.Error(),
		)

	if len(runes) <= maxRunes {
		return string(runes)
	}

	return string(
		runes[:maxRunes],
	)
}
