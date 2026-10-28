package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type CreateRefreshSessionParams struct {
	ID        string
	UserID    string
	TokenHash []byte
	ExpiresAt time.Time
}

func (repo *Repository) CreateRefreshSession(ctx context.Context, params CreateRefreshSessionParams) error {
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Every issued hash permanently identifies its session until that session is
	// deleted. This lets logout revoke a session even if a concurrent refresh
	// replaced the cookie in the meantime.
	_, err := repo.db.Exec(queryCtx, `
		WITH session AS (
			INSERT INTO refresh_sessions (id, user_id, refresh_token_hash, expires_at)
			VALUES ($1, $2, $3, $4)
			RETURNING id
		)
		INSERT INTO refresh_session_tokens (token_hash, session_id)
		SELECT $3, id FROM session
	`, params.ID, params.UserID, params.TokenHash, params.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create refresh session: %w", err)
	}
	return nil
}

type RotateRefreshSessionParams struct {
	OldTokenHash []byte
	NewTokenHash []byte
}

type RotatedRefreshSession struct {
	User      *User
	ExpiresAt time.Time
}

func (repo *Repository) RotateRefreshSession(ctx context.Context, params RotateRefreshSessionParams) (*RotatedRefreshSession, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := repo.db.Begin(queryCtx)
	if err != nil {
		return nil, fmt.Errorf("begin refresh session transaction: %w", err)
	}
	defer rollbackSession(tx)

	var sessionID, userID string
	var expiresAt time.Time
	// PostgreSQL locks the row and rechecks this predicate after a concurrent
	// writer commits. Exactly one rotation of the same current hash can succeed.
	err = tx.QueryRow(queryCtx, `
		UPDATE refresh_sessions
		SET refresh_token_hash = $2, last_used_at = NOW()
		WHERE refresh_token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()
		RETURNING id, user_id, expires_at
	`, params.OldTokenHash, params.NewTokenHash).Scan(&sessionID, &userID, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidRefreshToken
	}
	if err != nil {
		return nil, fmt.Errorf("rotate refresh session: %w", err)
	}

	// The previous binary may have issued this current token after migration
	// backfilled the history table. Record its association before replacing it.
	if _, err := tx.Exec(queryCtx, `
		INSERT INTO refresh_session_tokens (token_hash, session_id) VALUES ($1, $2)
		ON CONFLICT (token_hash) DO NOTHING
	`, params.OldTokenHash, sessionID); err != nil {
		return nil, fmt.Errorf("record previous refresh session token: %w", err)
	}
	if _, err := tx.Exec(queryCtx, `
		INSERT INTO refresh_session_tokens (token_hash, session_id) VALUES ($1, $2)
	`, params.NewTokenHash, sessionID); err != nil {
		return nil, fmt.Errorf("record refresh session token: %w", err)
	}

	var user User
	err = tx.QueryRow(queryCtx, `
		SELECT id, email, created_at, updated_at FROM users WHERE id = $1
	`, userID).Scan(&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidRefreshToken
	}
	if err != nil {
		return nil, fmt.Errorf("get refresh session user: %w", err)
	}
	if err := tx.Commit(queryCtx); err != nil {
		return nil, fmt.Errorf("commit refresh rotation: %w", err)
	}
	return &RotatedRefreshSession{User: &user, ExpiresAt: expiresAt}, nil
}

func (repo *Repository) RevokeRefreshSession(ctx context.Context, tokenHash []byte) error {
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := repo.db.Begin(queryCtx)
	if err != nil {
		return fmt.Errorf("begin revoke refresh session: %w", err)
	}
	defer rollbackSession(tx)

	var sessionID string
	// Resolve identity before locking/updating. The current-hash fallback also
	// supports sessions written by the old binary after migration backfill. An
	// UPDATE with a hash OR history predicate alone can lose a concurrent legacy
	// rotation: PostgreSQL rechecks the changed hash while its original statement
	// snapshot still cannot see that rotation's newly inserted history row.
	err = tx.QueryRow(queryCtx, `
		SELECT session_id FROM refresh_session_tokens WHERE token_hash = $1
		UNION ALL
		SELECT id FROM refresh_sessions WHERE refresh_token_hash = $1
		LIMIT 1
	`, tokenHash).Scan(&sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("resolve refresh session to revoke: %w", err)
	}
	// Both logout and rotation serialize on this stable session ID. A completed
	// logout prevents every further refresh, regardless of the current hash.
	if _, err := tx.Exec(queryCtx, `
		UPDATE refresh_sessions SET revoked_at = NOW()
		WHERE id = $1 AND revoked_at IS NULL
	`, sessionID); err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}
	if err := tx.Commit(queryCtx); err != nil {
		return fmt.Errorf("commit refresh session revocation: %w", err)
	}
	return nil
}

func rollbackSession(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
