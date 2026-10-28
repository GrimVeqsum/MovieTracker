package users

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestAccessTokenClaims(t *testing.T) {
	service := NewService(nil, strings.Repeat("s", 32), "movietracker-auth", "movietracker-api")
	for _, tc := range []struct {
		name   string
		change func(*AccessTokenClaims)
		method jwt.SigningMethod
		valid  bool
	}{
		{name: "valid", valid: true},
		{name: "wrong algorithm", method: jwt.SigningMethodHS384},
		{name: "unsigned", method: jwt.SigningMethodNone},
		{name: "missing expiration", change: func(c *AccessTokenClaims) { c.ExpiresAt = nil }},
		{name: "expired", change: func(c *AccessTokenClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute)) }},
		{name: "future issued at", change: func(c *AccessTokenClaims) { c.IssuedAt = jwt.NewNumericDate(time.Now().Add(time.Minute)) }},
		{name: "future not before", change: func(c *AccessTokenClaims) { c.NotBefore = jwt.NewNumericDate(time.Now().Add(time.Minute)) }},
		{name: "wrong issuer", change: func(c *AccessTokenClaims) { c.Issuer = "other" }},
		{name: "wrong audience", change: func(c *AccessTokenClaims) { c.Audience = jwt.ClaimStrings{"other"} }},
		{name: "empty subject", change: func(c *AccessTokenClaims) { c.Subject = "" }},
		{name: "non UUID subject", change: func(c *AccessTokenClaims) { c.Subject = "not-a-user-id" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := AccessTokenClaims{RegisteredClaims: jwt.RegisteredClaims{
				Issuer: service.jwtIssuer, Audience: jwt.ClaimStrings{service.jwtAudience}, Subject: uuid.NewString(),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)), IssuedAt: jwt.NewNumericDate(time.Now()),
			}}
			if tc.change != nil {
				tc.change(&claims)
			}
			method := tc.method
			if method == nil {
				method = jwt.SigningMethodHS256
			}
			var key any = []byte(service.jwtSecret)
			if method == jwt.SigningMethodNone {
				key = jwt.UnsafeAllowNoneSignatureType
			}
			token, err := jwt.NewWithClaims(method, claims).SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			subject, err := service.userIDFromAccessToken(token)
			if tc.valid {
				if err != nil || subject != claims.Subject {
					t.Fatalf("subject=%q err=%v", subject, err)
				}
			} else if !errors.Is(err, ErrInvalidAccessToken) {
				t.Fatalf("expected ErrInvalidAccessToken, got %v", err)
			}
		})
	}
}

func TestBrowserAuthOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, configured, origin, fetchSite string
		wantAllowed                         bool
	}{
		{name: "curl", wantAllowed: true},
		{name: "local browser", origin: "http://example.com", wantAllowed: true},
		{name: "production behind gateway", configured: "https://movies.example.com", origin: "https://movies.example.com", wantAllowed: true},
		{name: "cross origin", origin: "https://attacker.example", fetchSite: "cross-site"},
		{name: "same site sibling", configured: "https://movies.example.com", origin: "https://other.example.com", fetchSite: "same-site"},
		{name: "null origin", origin: "null"},
		{name: "origin with credentials", origin: "http://user@example.com"},
		{name: "origin with path", origin: "http://example.com/path"},
		{name: "cross site without origin", fetchSite: "cross-site"},
		{name: "same site without origin", fetchSite: "same-site"},
		{name: "same origin metadata", fetchSite: "same-origin", wantAllowed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewHandler(nil, false, tc.configured)
			r := httptest.NewRequest(http.MethodPost, "http://example.com/auth/refresh", nil)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Sec-Fetch-Site", tc.fetchSite)
			// Never accept a user-supplied forwarded host as the trusted origin.
			r.Header.Set("X-Forwarded-Host", "attacker.example")
			w := httptest.NewRecorder()
			if allowed := handler.allowBrowserAuthRequest(w, r); allowed != tc.wantAllowed {
				t.Fatalf("allowed=%v want=%v body=%s", allowed, tc.wantAllowed, w.Body.String())
			}
			if !tc.wantAllowed && (w.Code != http.StatusForbidden || len(w.Result().Cookies()) != 0) {
				t.Fatalf("CSRF rejection changed session: status=%d cookies=%v", w.Code, w.Result().Cookies())
			}
		})
	}
}

func TestBrowserAuthEndpointsRejectCSRFBeforeCredentials(t *testing.T) {
	handler := NewHandler(nil, true, "https://movies.example.com")
	for name, endpoint := range map[string]http.HandlerFunc{"login": handler.Login, "refresh": handler.Refresh, "logout": handler.Logout} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "http://auth-app:8082/auth/"+name, strings.NewReader(`{"email":"user@example.com","password":"password"}`))
			r.Header.Set("Origin", "https://attacker.example.com")
			w := httptest.NewRecorder()
			endpoint(w, r)
			if w.Code != http.StatusForbidden || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d headers=%v", w.Code, w.Header())
			}
		})
	}
}

func TestRefreshCookieSecurity(t *testing.T) {
	for _, secure := range []bool{true, false} {
		handler := NewHandler(nil, secure, "")
		w := httptest.NewRecorder()
		handler.setRefreshCookie(w, "test-refresh-token", time.Now().Add(time.Hour))
		cookie := w.Result().Cookies()[0]
		if !cookie.HttpOnly || cookie.Secure != secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/auth" || cookie.Domain != "" || cookie.MaxAge <= 0 {
			t.Fatalf("invalid refresh cookie: %+v", cookie)
		}
		w = httptest.NewRecorder()
		handler.clearRefreshCookie(w)
		cookie = w.Result().Cookies()[0]
		if cookie.MaxAge != -1 || cookie.Path != "/auth" || cookie.Secure != secure || !cookie.HttpOnly {
			t.Fatalf("invalid cookie deletion: %+v", cookie)
		}
	}
}
