package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestJWTClaimsAndSignature(t *testing.T) {
	const secret = "test-only-secret-with-at-least-32-bytes"
	now := time.Now()
	valid := func() AccessTokenClaims {
		return AccessTokenClaims{RegisteredClaims: jwt.RegisteredClaims{Subject: uuid.NewString(), Issuer: "movietracker-auth", Audience: jwt.ClaimStrings{"movietracker-api"}, ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)), IssuedAt: jwt.NewNumericDate(now)}}
	}
	cases := []struct {
		name   string
		mutate func(*AccessTokenClaims)
		method jwt.SigningMethod
		key    any
		want   int
	}{
		{name: "valid", want: http.StatusNoContent},
		{name: "missing expiration", mutate: func(c *AccessTokenClaims) { c.ExpiresAt = nil }, want: 401},
		{name: "expired", mutate: func(c *AccessTokenClaims) { c.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Minute)) }, want: 401},
		{name: "wrong issuer", mutate: func(c *AccessTokenClaims) { c.Issuer = "attacker" }, want: 401},
		{name: "wrong audience", mutate: func(c *AccessTokenClaims) { c.Audience = jwt.ClaimStrings{"other"} }, want: 401},
		{name: "future issued at", mutate: func(c *AccessTokenClaims) { c.IssuedAt = jwt.NewNumericDate(now.Add(time.Hour)) }, want: 401},
		{name: "future not before", mutate: func(c *AccessTokenClaims) { c.NotBefore = jwt.NewNumericDate(now.Add(time.Hour)) }, want: 401},
		{name: "invalid subject", mutate: func(c *AccessTokenClaims) { c.Subject = "invalid-user-id" }, want: 401},
		{name: "empty subject", mutate: func(c *AccessTokenClaims) { c.Subject = "" }, want: 401},
		{name: "wrong signature", key: []byte("other-secret"), want: 401},
		{name: "wrong algorithm", method: jwt.SigningMethodHS384, want: 401},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			claims := valid()
			if tt.mutate != nil {
				tt.mutate(&claims)
			}
			method := tt.method
			if method == nil {
				method = jwt.SigningMethodHS256
			}
			key := tt.key
			if key == nil {
				key = []byte(secret)
			}
			signed, err := jwt.NewWithClaims(method, claims).SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodGet, "/movies", nil)
			request.Header.Set("Authorization", "Bearer "+signed)
			recorder := httptest.NewRecorder()
			NewMiddleware(secret, "movietracker-auth", "movietracker-api")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got, ok := UserIDFromContext(r.Context()); !ok || got != claims.Subject {
					t.Errorf("user context = %q %v", got, ok)
				}
				w.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(recorder, request)
			if recorder.Code != tt.want {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
