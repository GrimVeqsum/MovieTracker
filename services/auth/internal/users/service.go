package users

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo      *Repository
	jwtSecret string
}

func NewService(repo *Repository, jwtSecret string) *Service {
	return &Service{
		repo:      repo,
		jwtSecret: jwtSecret,
	}
}

// create
type RegisterParams struct {
	Email    string
	Password string
}

func (service *Service) Register(ctx context.Context, params RegisterParams) (*User, error) {
	email := strings.TrimSpace(strings.ToLower(params.Email))
	if email == "" {
		return nil, ErrEmailRequired
	}

	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || parsedEmail.Address != email {
		return nil, ErrInvalidEmail
	}

	if len(params.Password) < 8 {
		return nil, ErrPasswordTooShort
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	return service.repo.Create(ctx, CreateUserParams{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: string(passwordHash),
	})
}

// log in
type LoginParams struct {
	Email    string
	Password string
}

type LoginResult struct {
	User        *User
	AccessToken string
}

type AccessTokenClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

func (service *Service) Login(ctx context.Context, params LoginParams) (*LoginResult, error) {
	email := strings.TrimSpace(strings.ToLower(params.Email))
	if email == "" {
		return nil, ErrEmailRequired
	}

	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || parsedEmail.Address != email {
		return nil, ErrInvalidEmail
	}

	user, err := service.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}

		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(params.Password))
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	tokenExpiresAt := time.Now().Add(24 * time.Hour)

	claims := AccessTokenClaims{
		Email: user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(tokenExpiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	accessToken, err := token.SignedString([]byte(service.jwtSecret))
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		User: &User{
			ID:        user.ID,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
		AccessToken: accessToken,
	}, nil
}
