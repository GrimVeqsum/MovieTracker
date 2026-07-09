package auth

import (
	"context"
	"net/http"
	"strings"

	"movie-platform/library/internal/transport/http/response"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userIDKey contextKey = "user_id"

type AccessTokenClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

func NewMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.Error(w, http.StatusUnauthorized, "unauthorized", "authorization header is required")
				return
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				response.Error(w, http.StatusUnauthorized, "unauthorized", "invalid authorization header")
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == "" {
				response.Error(w, http.StatusUnauthorized, "unauthorized", "token is empty")
				return
			}

			claims := &AccessTokenClaims{}

			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				response.Error(w, http.StatusUnauthorized, "unauthorized", "invalid token")
				return
			}

			if claims.Subject == "" {
				response.Error(w, http.StatusUnauthorized, "unauthorized", "user id is empty")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, claims.Subject)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey).(string)
	return userID, ok
}
