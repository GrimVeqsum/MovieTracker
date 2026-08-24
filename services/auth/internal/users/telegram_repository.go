package users

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (repo *Repository) CreateTelegramLinkCode(
	ctx context.Context,
	userID string,
	codeHash []byte,
	expiresAt time.Time,
) error {
	queryCtx, cancel :=
		context.WithTimeout(
			ctx,
			3*time.Second,
		)
	defer cancel()

	_, err := repo.db.Exec(
		queryCtx,
		`
			INSERT INTO telegram_link_codes (
				user_id,
				code_hash,
				expires_at
			)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id)
			DO UPDATE SET
				code_hash = EXCLUDED.code_hash,
				expires_at = EXCLUDED.expires_at,
				created_at = NOW()
		`,
		userID,
		codeHash,
		expiresAt,
	)

	return err
}

func (repo *Repository) LinkTelegram(
	ctx context.Context,
	codeHash []byte,
	telegramUserID int64,
) error {
	queryCtx, cancel :=
		context.WithTimeout(
			ctx,
			3*time.Second,
		)
	defer cancel()

	tx, err := repo.db.Begin(queryCtx)
	if err != nil {
		return err
	}

	defer tx.Rollback(queryCtx)

	var userID string

	err = tx.QueryRow(
		queryCtx,
		`
			SELECT user_id
			FROM telegram_link_codes
			WHERE code_hash = $1
			  AND expires_at > NOW()
			FOR UPDATE
		`,
		codeHash,
	).Scan(
		&userID,
	)

	if err != nil {
		if errors.Is(
			err,
			pgx.ErrNoRows,
		) {
			return ErrTelegramLinkCodeNotFound
		}

		return err
	}

	result, err := tx.Exec(
		queryCtx,
		`
			UPDATE users
			SET
				telegram_user_id = $1,
				updated_at = NOW()
			WHERE id = $2
			  AND (
				telegram_user_id IS NULL
				OR telegram_user_id = $1
			  )
		`,
		telegramUserID,
		userID,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(
			err,
			&pgErr,
		) &&
			pgErr.Code == "23505" {

			return ErrTelegramAccountAlreadyLinked
		}

		return err
	}

	if result.RowsAffected() == 0 {
		return ErrMovieTrackerAccountAlreadyLinked
	}

	_, err = tx.Exec(
		queryCtx,
		`
			DELETE FROM telegram_link_codes
			WHERE user_id = $1
		`,
		userID,
	)

	if err != nil {
		return err
	}

	return tx.Commit(queryCtx)
}

func (repo *Repository) GetByTelegramUserID(
	ctx context.Context,
	telegramUserID int64,
) (*User, error) {
	queryCtx, cancel :=
		context.WithTimeout(
			ctx,
			3*time.Second,
		)
	defer cancel()

	var user User

	err := repo.db.QueryRow(
		queryCtx,
		`
			SELECT
				id,
				email,
				created_at,
				updated_at
			FROM users
			WHERE telegram_user_id = $1
		`,
		telegramUserID,
	).Scan(
		&user.ID,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(
			err,
			pgx.ErrNoRows,
		) {
			return nil,
				ErrTelegramUserNotLinked
		}

		return nil, err
	}

	return &user, nil
}
