package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"movie-platform/library/internal/transport/http/response"

	"github.com/golang-jwt/jwt/v5"
)

const jwtClockLeeway = 30 * time.Second

type contextKey string

const userIDKey contextKey = "user_id"

type AccessTokenClaims struct {
	Email string `json:"email"`

	jwt.RegisteredClaims
}

func NewMiddleware(
	jwtSecret string,
	jwtIssuer string,
	jwtAudience string,
) func(http.Handler) http.Handler {
	return func(
		next http.Handler,
	) http.Handler {
		return http.HandlerFunc(
			func(
				w http.ResponseWriter,
				r *http.Request,
			) {
				tokenString, ok :=
					readBearerToken(
						r,
					)

				if !ok {
					response.Error(
						w,
						http.StatusUnauthorized,
						"unauthorized",
						"invalid authorization header",
					)

					return
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
								jwtSecret,
							), nil
						},

						jwt.WithValidMethods(
							[]string{
								jwt.SigningMethodHS256.Alg(),
							},
						),

						jwt.WithIssuer(
							jwtIssuer,
						),

						jwt.WithAudience(
							jwtAudience,
						),

						jwt.WithExpirationRequired(),

						jwt.WithIssuedAt(),

						jwt.WithLeeway(
							jwtClockLeeway,
						),
					)

				if err != nil ||
					!token.Valid {

					response.Error(
						w,
						http.StatusUnauthorized,
						"unauthorized",
						"invalid token",
					)

					return
				}

				if strings.TrimSpace(
					claims.Subject,
				) == "" {

					response.Error(
						w,
						http.StatusUnauthorized,
						"unauthorized",
						"user id is empty",
					)

					return
				}

				ctx :=
					context.WithValue(
						r.Context(),
						userIDKey,
						claims.Subject,
					)

				next.ServeHTTP(
					w,
					r.WithContext(ctx),
				)
			},
		)
	}
}

func readBearerToken(
	r *http.Request,
) (string, bool) {
	parts :=
		strings.Fields(
			r.Header.Get(
				"Authorization",
			),
		)

	if len(parts) != 2 {
		return "", false
	}

	if !strings.EqualFold(
		parts[0],
		"Bearer",
	) {
		return "", false
	}

	token :=
		strings.TrimSpace(
			parts[1],
		)

	if token == "" {
		return "", false
	}

	return token, true
}

func UserIDFromContext(
	ctx context.Context,
) (string, bool) {
	userID, ok :=
		ctx.Value(
			userIDKey,
		).(string)

	return userID, ok
}
