package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type CreateRefreshSessionParams struct {
	ID string

	UserID string

	TokenHash []byte

	ExpiresAt time.Time
}

func (repo *Repository) CreateRefreshSession(
	ctx context.Context,
	params CreateRefreshSessionParams,
) error {
	queryCtx, cancel :=
		context.WithTimeout(
			ctx,
			3*time.Second,
		)
	defer cancel()

	_, err :=
		repo.db.Exec(
			queryCtx,
			`
			INSERT INTO refresh_sessions (
				id,
				user_id,
				refresh_token_hash,
				expires_at
			)
			VALUES (
				$1,
				$2,
				$3,
				$4
			)
			`,
			params.ID,
			params.UserID,
			params.TokenHash,
			params.ExpiresAt,
		)

	if err != nil {
		return fmt.Errorf(
			"create refresh session: %w",
			err,
		)
	}

	return nil
}

type RotateRefreshSessionParams struct {
	OldTokenHash []byte

	NewTokenHash []byte
}

type RotatedRefreshSession struct {
	User *User

	ExpiresAt time.Time
}

func (repo *Repository) RotateRefreshSession(
	ctx context.Context,
	params RotateRefreshSessionParams,
) (*RotatedRefreshSession, error) {
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
				"begin refresh session transaction: %w",
				err,
			)
	}

	defer func() {
		_ = tx.Rollback(
			queryCtx,
		)
	}()

	var userID string
	var expiresAt time.Time

	err =
		tx.QueryRow(
			queryCtx,
			`
			UPDATE refresh_sessions
			SET
				refresh_token_hash = $2,
				last_used_at = NOW()
			WHERE refresh_token_hash = $1
			  AND revoked_at IS NULL
			  AND expires_at > NOW()
			RETURNING
				user_id,
				expires_at
			`,
			params.OldTokenHash,
			params.NewTokenHash,
		).
			Scan(
				&userID,
				&expiresAt,
			)

	if err != nil {
		if errors.Is(
			err,
			pgx.ErrNoRows,
		) {
			return nil,
				ErrInvalidRefreshToken
		}

		return nil,
			fmt.Errorf(
				"rotate refresh session: %w",
				err,
			)
	}

	var user User

	err =
		tx.QueryRow(
			queryCtx,
			`
			SELECT
				id,
				email,
				created_at,
				updated_at
			FROM users
			WHERE id = $1
			`,
			userID,
		).
			Scan(
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
				ErrInvalidRefreshToken
		}

		return nil,
			fmt.Errorf(
				"get refresh session user: %w",
				err,
			)
	}

	if err :=
		tx.Commit(
			queryCtx,
		); err != nil {

		return nil,
			fmt.Errorf(
				"commit refresh rotation: %w",
				err,
			)
	}

	return &RotatedRefreshSession{
		User: &user,

		ExpiresAt: expiresAt,
	}, nil
}

func (repo *Repository) RevokeRefreshSession(
	ctx context.Context,
	tokenHash []byte,
) error {
	queryCtx, cancel :=
		context.WithTimeout(
			ctx,
			3*time.Second,
		)
	defer cancel()

	_, err :=
		repo.db.Exec(
			queryCtx,
			`
			UPDATE refresh_sessions
			SET revoked_at = NOW()
			WHERE refresh_token_hash = $1
			  AND revoked_at IS NULL
			`,
			tokenHash,
		)

	if err != nil {
		return fmt.Errorf(
			"revoke refresh session: %w",
			err,
		)
	}

	return nil
}
