package httptransport

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIPRateLimiterBlocksRequestsOverLimit(
	t *testing.T,
) {
	limiter :=
		NewIPRateLimiter(
			2,
			time.Minute,
		)

	next :=
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				r *http.Request,
			) {
				w.WriteHeader(
					http.StatusNoContent,
				)
			},
		)

	handler :=
		limiter.Middleware(
			next,
		)

	for i := 0; i < 2; i++ {
		request :=
			httptest.NewRequest(
				http.MethodPost,
				"/auth/login",
				nil,
			)

		request.RemoteAddr =
			"203.0.113.10:12345"

		recorder :=
			httptest.NewRecorder()

		handler.ServeHTTP(
			recorder,
			request,
		)

		if recorder.Code !=
			http.StatusNoContent {

			t.Fatalf(
				"request %d: expected %d, got %d",
				i+1,
				http.StatusNoContent,
				recorder.Code,
			)
		}
	}

	request :=
		httptest.NewRequest(
			http.MethodPost,
			"/auth/login",
			nil,
		)

	request.RemoteAddr =
		"203.0.113.10:12345"

	recorder :=
		httptest.NewRecorder()

	handler.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code !=
		http.StatusTooManyRequests {

		t.Fatalf(
			"expected %d, got %d",
			http.StatusTooManyRequests,
			recorder.Code,
		)
	}

	if recorder.Header().
		Get(
			"Retry-After",
		) == "" {

		t.Fatal(
			"expected Retry-After header",
		)
	}
}

func TestIPRateLimiterSeparatesClients(
	t *testing.T,
) {
	limiter :=
		NewIPRateLimiter(
			1,
			time.Minute,
		)

	if allowed, _ :=
		limiter.allow(
			"203.0.113.10",
		); !allowed {

		t.Fatal(
			"first client should be allowed",
		)
	}

	if allowed, _ :=
		limiter.allow(
			"203.0.113.10",
		); allowed {

		t.Fatal(
			"first client should be rate limited",
		)
	}

	if allowed, _ :=
		limiter.allow(
			"203.0.113.11",
		); !allowed {

		t.Fatal(
			"second client should have its own limit",
		)
	}
}

func TestClientIPUsesForwardedAddress(
	t *testing.T,
) {
	request :=
		httptest.NewRequest(
			http.MethodGet,
			"/",
			nil,
		)

	request.Header.Set(
		"X-Forwarded-For",
		"198.51.100.25, 10.0.0.1",
	)

	got :=
		clientIP(
			request,
		)

	if got != "198.51.100.25" {
		t.Fatalf(
			"expected 198.51.100.25, got %s",
			got,
		)
	}
}
