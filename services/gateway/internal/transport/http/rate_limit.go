package httptransport

import (
	"encoding/json"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rateLimitEntry struct {
	windowStartedAt time.Time
	lastSeenAt      time.Time
	requestCount    int
}

type IPRateLimiter struct {
	mu sync.Mutex

	entries map[string]*rateLimitEntry

	limit int

	window time.Duration

	lastCleanup time.Time
}

func NewIPRateLimiter(
	limit int,
	window time.Duration,
) *IPRateLimiter {
	return &IPRateLimiter{
		entries: make(
			map[string]*rateLimitEntry,
		),

		limit: limit,

		window: window,

		lastCleanup: time.Now(),
	}
}

func (limiter *IPRateLimiter) Middleware(
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(
		func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			ip :=
				clientIP(
					r,
				)

			allowed, retryAfter :=
				limiter.allow(
					ip,
				)

			if !allowed {
				writeRateLimitError(
					w,
					retryAfter,
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

func (limiter *IPRateLimiter) allow(
	key string,
) (bool, time.Duration) {
	now :=
		time.Now()

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	limiter.cleanupLocked(
		now,
	)

	entry, exists :=
		limiter.entries[key]

	if !exists {
		limiter.entries[key] =
			&rateLimitEntry{
				windowStartedAt: now,

				lastSeenAt: now,

				requestCount: 1,
			}

		return true, 0
	}

	entry.lastSeenAt =
		now

	if now.Sub(
		entry.windowStartedAt,
	) >= limiter.window {

		entry.windowStartedAt =
			now

		entry.requestCount =
			1

		return true, 0
	}

	if entry.requestCount >=
		limiter.limit {

		retryAfter :=
			entry.windowStartedAt.
				Add(
					limiter.window,
				).
				Sub(
					now,
				)

		if retryAfter < time.Second {
			retryAfter =
				time.Second
		}

		return false,
			retryAfter
	}

	entry.requestCount++

	return true, 0
}

func (limiter *IPRateLimiter) cleanupLocked(
	now time.Time,
) {
	if now.Sub(
		limiter.lastCleanup,
	) < limiter.window {

		return
	}

	expireBefore :=
		now.Add(
			-2 * limiter.window,
		)

	for key, entry := range limiter.entries {

		if entry.lastSeenAt.Before(
			expireBefore,
		) {
			delete(
				limiter.entries,
				key,
			)
		}
	}

	limiter.lastCleanup =
		now
}

func clientIP(
	r *http.Request,
) string {
	forwardedFor :=
		strings.TrimSpace(
			r.Header.Get(
				"X-Forwarded-For",
			),
		)

	if forwardedFor != "" {
		first :=
			strings.TrimSpace(
				strings.Split(
					forwardedFor,
					",",
				)[0],
			)

		if parsedIP :=
			net.ParseIP(
				first,
			); parsedIP != nil {

			return parsedIP.String()
		}
	}

	host, _, err :=
		net.SplitHostPort(
			r.RemoteAddr,
		)

	if err == nil {
		if parsedIP :=
			net.ParseIP(
				host,
			); parsedIP != nil {

			return parsedIP.String()
		}

		return host
	}

	if parsedIP :=
		net.ParseIP(
			r.RemoteAddr,
		); parsedIP != nil {

		return parsedIP.String()
	}

	return r.RemoteAddr
}

func writeRateLimitError(
	w http.ResponseWriter,
	retryAfter time.Duration,
) {
	seconds :=
		int(
			math.Ceil(
				retryAfter.Seconds(),
			),
		)

	if seconds < 1 {
		seconds = 1
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.Header().Set(
		"Retry-After",
		strconv.Itoa(
			seconds,
		),
	)

	w.WriteHeader(
		http.StatusTooManyRequests,
	)

	_ =
		json.NewEncoder(
			w,
		).Encode(
			map[string]any{
				"error": map[string]string{
					"code": "rate_limit_exceeded",

					"message": "too many requests",
				},
			},
		)
}
