package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"movie-platform/library/internal/transport/http/response"
)

const serviceTokenHeader = "X-Service-Token"

func NewServiceTokenMiddleware(
	serviceSecret string,
) func(http.Handler) http.Handler {
	expectedSecret := strings.TrimSpace(
		serviceSecret,
	)

	expectedHash := sha256.Sum256(
		[]byte(expectedSecret),
	)

	return func(
		next http.Handler,
	) http.Handler {
		return http.HandlerFunc(
			func(
				w http.ResponseWriter,
				r *http.Request,
			) {
				receivedSecret := strings.TrimSpace(
					r.Header.Get(
						serviceTokenHeader,
					),
				)

				if receivedSecret == "" ||
					expectedSecret == "" {

					response.Error(
						w,
						http.StatusUnauthorized,
						"unauthorized",
						"invalid service token",
					)

					return
				}

				receivedHash := sha256.Sum256(
					[]byte(receivedSecret),
				)

				if subtle.ConstantTimeCompare(
					expectedHash[:],
					receivedHash[:],
				) != 1 {

					response.Error(
						w,
						http.StatusUnauthorized,
						"unauthorized",
						"invalid service token",
					)

					return
				}

				next.ServeHTTP(
					w,
					r,
				)
			},
		)
	}
}
