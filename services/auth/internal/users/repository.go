package users

import (
	"context"
	"errors"
	"time"

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

// create
type CreateUserParams struct {
	ID           string
	Email        string
	PasswordHash string
}

func (repo *Repository) Create(ctx context.Context, params CreateUserParams) (*User, error) {
	query := `
		INSERT INTO users (
			id,
			email,
			password_hash
		)
		VALUES ($1, $2, $3)
		RETURNING
			id,
			email,
			created_at,
			updated_at
	`

	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var user User

	err := repo.db.QueryRow(
		queryCtx,
		query,
		params.ID,
		params.Email,
		params.PasswordHash,
	).Scan(
		&user.ID,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrUserAlreadyExists
		}

		return nil, err
	}

	return &user, nil
}

// log in
type UserWithPasswordHash struct {
	User
	PasswordHash string
}

func (repo *Repository) GetByEmail(ctx context.Context, email string) (*UserWithPasswordHash, error) {
	query := `
		SELECT
			id,
			email,
			password_hash,
			created_at,
			updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`

	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var user UserWithPasswordHash

	err := repo.db.QueryRow(queryCtx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &user, nil
}
