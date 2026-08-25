package users

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	jwtClockLeeway = 30 * time.Second

	accessTokenTTL = 15 * time.Minute

	telegramAccessTokenTTL = 15 * time.Minute

	refreshSessionTTL = 30 * 24 * time.Hour

	maxEmailLength = 254

	maxPasswordBytes = 72
)

type Service struct {
	repo *Repository

	jwtSecret string

	jwtIssuer string

	jwtAudience string
}

func NewService(
	repo *Repository,
	jwtSecret string,
	jwtIssuer string,
	jwtAudience string,
) *Service {
	return &Service{
		repo: repo,

		jwtSecret: jwtSecret,

		jwtIssuer: jwtIssuer,

		jwtAudience: jwtAudience,
	}
}

type RegisterParams struct {
	Email string

	Password string
}

func (service *Service) Register(
	ctx context.Context,
	params RegisterParams,
) (*User, error) {
	email :=
		normalizeEmail(
			params.Email,
		)

	if email == "" {
		return nil,
			ErrEmailRequired
	}

	if len(email) >
		maxEmailLength {

		return nil,
			ErrEmailTooLong
	}

	parsedEmail, err :=
		mail.ParseAddress(
			email,
		)

	if err != nil ||
		parsedEmail.Address != email {

		return nil,
			ErrInvalidEmail
	}

	if len(params.Password) < 8 {
		return nil,
			ErrPasswordTooShort
	}

	if len(
		[]byte(
			params.Password,
		),
	) > maxPasswordBytes {

		return nil,
			ErrPasswordTooLong
	}

	passwordHash, err :=
		bcrypt.GenerateFromPassword(
			[]byte(
				params.Password,
			),
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

			PasswordHash: string(
				passwordHash,
			),
		},
	)
}

type LoginParams struct {
	Email string

	Password string
}

type LoginResult struct {
	User *User

	AccessToken string

	RefreshToken string

	RefreshExpiresAt time.Time
}

type AccessTokenClaims struct {
	Email string `json:"email"`

	jwt.RegisteredClaims
}

func (service *Service) Login(
	ctx context.Context,
	params LoginParams,
) (*LoginResult, error) {
	email :=
		normalizeEmail(
			params.Email,
		)

	if email == "" {
		return nil,
			ErrInvalidCredentials
	}

	if len(email) >
		maxEmailLength {

		return nil,
			ErrInvalidCredentials
	}

	if len(
		[]byte(
			params.Password,
		),
	) > maxPasswordBytes {

		return nil,
			ErrInvalidCredentials
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
			[]byte(
				user.PasswordHash,
			),
			[]byte(
				params.Password,
			),
		)

	if err != nil {
		return nil,
			ErrInvalidCredentials
	}

	resultUser :=
		&User{
			ID: user.ID,

			Email: user.Email,

			CreatedAt: user.CreatedAt,

			UpdatedAt: user.UpdatedAt,
		}

	accessToken, err :=
		service.createAccessToken(
			resultUser,
			accessTokenTTL,
		)

	if err != nil {
		return nil, err
	}

	refreshToken, err :=
		generateRefreshToken()

	if err != nil {
		return nil, err
	}

	refreshExpiresAt :=
		time.Now().
			UTC().
			Add(
				refreshSessionTTL,
			)

	err =
		service.repo.CreateRefreshSession(
			ctx,
			CreateRefreshSessionParams{
				ID: uuid.NewString(),

				UserID: resultUser.ID,

				TokenHash: hashRefreshToken(
					refreshToken,
				),

				ExpiresAt: refreshExpiresAt,
			},
		)

	if err != nil {
		return nil, err
	}

	return &LoginResult{
		User: resultUser,

		AccessToken: accessToken,

		RefreshToken: refreshToken,

		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

type RefreshResult struct {
	AccessToken string

	RefreshToken string

	RefreshExpiresAt time.Time
}

func (service *Service) Refresh(
	ctx context.Context,
	refreshToken string,
) (*RefreshResult, error) {
	refreshToken =
		strings.TrimSpace(
			refreshToken,
		)

	if refreshToken == "" {
		return nil,
			ErrInvalidRefreshToken
	}

	newRefreshToken, err :=
		generateRefreshToken()

	if err != nil {
		return nil, err
	}

	session, err :=
		service.repo.RotateRefreshSession(
			ctx,
			RotateRefreshSessionParams{
				OldTokenHash: hashRefreshToken(
					refreshToken,
				),

				NewTokenHash: hashRefreshToken(
					newRefreshToken,
				),
			},
		)

	if err != nil {
		return nil, err
	}

	accessToken, err :=
		service.createAccessToken(
			session.User,
			accessTokenTTL,
		)

	if err != nil {
		return nil, err
	}

	return &RefreshResult{
		AccessToken: accessToken,

		RefreshToken: newRefreshToken,

		RefreshExpiresAt: session.ExpiresAt,
	}, nil
}

func (service *Service) Logout(
	ctx context.Context,
	refreshToken string,
) error {
	refreshToken =
		strings.TrimSpace(
			refreshToken,
		)

	if refreshToken == "" {
		return nil
	}

	return service.repo.RevokeRefreshSession(
		ctx,
		hashRefreshToken(
			refreshToken,
		),
	)
}

type TelegramLinkCodeResult struct {
	Code string

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
		hashTelegramLinkCode(
			code,
		)

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
		Code: code,

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
		telegramAccessTokenTTL,
	)
}

func (service *Service) createAccessToken(
	user *User,
	duration time.Duration,
) (string, error) {
	now :=
		time.Now().
			UTC()

	claims :=
		AccessTokenClaims{
			Email: user.Email,

			RegisteredClaims: jwt.RegisteredClaims{
				Issuer: service.jwtIssuer,

				Subject: user.ID,

				Audience: jwt.ClaimStrings{
					service.jwtAudience,
				},

				ExpiresAt: jwt.NewNumericDate(
					now.Add(
						duration,
					),
				),

				NotBefore: jwt.NewNumericDate(
					now,
				),

				IssuedAt: jwt.NewNumericDate(
					now,
				),

				ID: uuid.NewString(),
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

			jwt.WithIssuer(
				service.jwtIssuer,
			),

			jwt.WithAudience(
				service.jwtAudience,
			),

			jwt.WithExpirationRequired(),

			jwt.WithIssuedAt(),

			jwt.WithLeeway(
				jwtClockLeeway,
			),
		)

	if err != nil ||
		!token.Valid ||
		strings.TrimSpace(
			claims.Subject,
		) == "" {

		return "",
			ErrInvalidAccessToken
	}

	return claims.Subject, nil
}

func normalizeEmail(
	email string,
) string {
	return strings.TrimSpace(
		strings.ToLower(
			email,
		),
	)
}

func generateRefreshToken() (
	string,
	error,
) {
	data :=
		make(
			[]byte,
			32,
		)

	if _, err :=
		rand.Read(
			data,
		); err != nil {

		return "", err
	}

	return hex.EncodeToString(
		data,
	), nil
}

func hashRefreshToken(
	token string,
) []byte {
	hash :=
		sha256.Sum256(
			[]byte(
				strings.TrimSpace(
					token,
				),
			),
		)

	return hash[:]
}

func generateTelegramLinkCode() (
	string,
	error,
) {
	data :=
		make(
			[]byte,
			5,
		)

	if _, err :=
		rand.Read(
			data,
		); err != nil {

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
			[]byte(
				normalized,
			),
		)

	return hash[:]
}
