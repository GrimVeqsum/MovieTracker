package users

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
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

func NewService(
	repo *Repository,
	jwtSecret string,
) *Service {
	return &Service{
		repo:      repo,
		jwtSecret: jwtSecret,
	}
}

type RegisterParams struct {
	Email    string
	Password string
}

func (service *Service) Register(
	ctx context.Context,
	params RegisterParams,
) (*User, error) {
	email := strings.TrimSpace(
		strings.ToLower(
			params.Email,
		),
	)

	if email == "" {
		return nil, ErrEmailRequired
	}

	parsedEmail, err := mail.ParseAddress(email)
	if err != nil ||
		parsedEmail.Address != email {

		return nil, ErrInvalidEmail
	}

	if len(params.Password) < 8 {
		return nil, ErrPasswordTooShort
	}

	passwordHash, err :=
		bcrypt.GenerateFromPassword(
			[]byte(params.Password),
			bcrypt.DefaultCost,
		)

	if err != nil {
		return nil, err
	}

	return service.repo.Create(
		ctx,
		CreateUserParams{
			ID: uuid.NewString(),

			Email: email,

			PasswordHash: string(passwordHash),
		},
	)
}

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

func (service *Service) Login(
	ctx context.Context,
	params LoginParams,
) (*LoginResult, error) {
	email := strings.TrimSpace(
		strings.ToLower(
			params.Email,
		),
	)

	if email == "" {
		return nil, ErrEmailRequired
	}

	parsedEmail, err :=
		mail.ParseAddress(email)

	if err != nil ||
		parsedEmail.Address != email {

		return nil, ErrInvalidEmail
	}

	user, err :=
		service.repo.GetByEmail(
			ctx,
			email,
		)

	if err != nil {
		if errors.Is(
			err,
			pgx.ErrNoRows,
		) {
			return nil,
				ErrInvalidCredentials
		}

		return nil, err
	}

	err =
		bcrypt.CompareHashAndPassword(
			[]byte(user.PasswordHash),
			[]byte(params.Password),
		)

	if err != nil {
		return nil,
			ErrInvalidCredentials
	}

	resultUser := &User{
		ID:        user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	accessToken, err :=
		service.createAccessToken(
			resultUser,
			24*time.Hour,
		)

	if err != nil {
		return nil, err
	}

	return &LoginResult{
		User:        resultUser,
		AccessToken: accessToken,
	}, nil
}

type TelegramLinkCodeResult struct {
	Code      string
	ExpiresAt time.Time
}

func (service *Service) CreateTelegramLinkCode(
	ctx context.Context,
	accessToken string,
) (*TelegramLinkCodeResult, error) {
	userID, err :=
		service.userIDFromAccessToken(
			accessToken,
		)

	if err != nil {
		return nil, err
	}

	code, err :=
		generateTelegramLinkCode()

	if err != nil {
		return nil, err
	}

	codeHash :=
		hashTelegramLinkCode(code)

	expiresAt :=
		time.Now().
			UTC().
			Add(
				10 * time.Minute,
			)

	err =
		service.repo.CreateTelegramLinkCode(
			ctx,
			userID,
			codeHash,
			expiresAt,
		)

	if err != nil {
		return nil, err
	}

	return &TelegramLinkCodeResult{
		Code:      code,
		ExpiresAt: expiresAt,
	}, nil
}

func (service *Service) LinkTelegram(
	ctx context.Context,
	code string,
	telegramUserID int64,
) error {
	if telegramUserID <= 0 {
		return ErrInvalidTelegramUserID
	}

	normalizedCode :=
		normalizeTelegramLinkCode(
			code,
		)

	if normalizedCode == "" {
		return ErrTelegramLinkCodeNotFound
	}

	codeHash :=
		hashTelegramLinkCode(
			normalizedCode,
		)

	return service.repo.LinkTelegram(
		ctx,
		codeHash,
		telegramUserID,
	)
}

func (service *Service) CreateTokenForTelegram(
	ctx context.Context,
	telegramUserID int64,
) (string, error) {
	if telegramUserID <= 0 {
		return "",
			ErrInvalidTelegramUserID
	}

	user, err :=
		service.repo.GetByTelegramUserID(
			ctx,
			telegramUserID,
		)

	if err != nil {
		return "", err
	}

	return service.createAccessToken(
		user,
		15*time.Minute,
	)
}

func (service *Service) createAccessToken(
	user *User,
	duration time.Duration,
) (string, error) {
	now := time.Now().UTC()

	claims :=
		AccessTokenClaims{
			Email: user.Email,

			RegisteredClaims: jwt.RegisteredClaims{
				Subject: user.ID,

				IssuedAt: jwt.NewNumericDate(
					now,
				),

				ExpiresAt: jwt.NewNumericDate(
					now.Add(
						duration,
					),
				),
			},
		}

	token :=
		jwt.NewWithClaims(
			jwt.SigningMethodHS256,
			claims,
		)

	return token.SignedString(
		[]byte(
			service.jwtSecret,
		),
	)
}

func (service *Service) userIDFromAccessToken(
	tokenString string,
) (string, error) {
	tokenString =
		strings.TrimSpace(
			tokenString,
		)

	if tokenString == "" {
		return "",
			ErrInvalidAccessToken
	}

	claims :=
		&AccessTokenClaims{}

	token, err :=
		jwt.ParseWithClaims(
			tokenString,
			claims,
			func(
				token *jwt.Token,
			) (any, error) {
				return []byte(
					service.jwtSecret,
				), nil
			},
			jwt.WithValidMethods(
				[]string{
					jwt.SigningMethodHS256.Alg(),
				},
			),
		)

	if err != nil ||
		!token.Valid ||
		claims.Subject == "" {

		return "",
			ErrInvalidAccessToken
	}

	return claims.Subject, nil
}

func generateTelegramLinkCode() (
	string,
	error,
) {
	data := make(
		[]byte,
		5,
	)

	if _, err :=
		rand.Read(data); err != nil {

		return "", err
	}

	encoder :=
		base32.StdEncoding.
			WithPadding(
				base32.NoPadding,
			)

	raw :=
		encoder.EncodeToString(
			data,
		)

	return raw[:4] +
			"-" +
			raw[4:],
		nil
}

func normalizeTelegramLinkCode(
	code string,
) string {
	code =
		strings.TrimSpace(
			code,
		)

	code =
		strings.ToUpper(
			code,
		)

	code =
		strings.ReplaceAll(
			code,
			"-",
			"",
		)

	code =
		strings.ReplaceAll(
			code,
			" ",
			"",
		)

	return code
}

func hashTelegramLinkCode(
	code string,
) []byte {
	normalized :=
		normalizeTelegramLinkCode(
			code,
		)

	hash :=
		sha256.Sum256(
			[]byte(normalized),
		)

	return hash[:]
}
