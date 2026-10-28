package users

import (
	"net/http"
	"net/url"
	"strings"

	"movie-platform/auth/internal/transport/http/response"
)

// Cookie authentication needs an origin check even with SameSite=Lax:
// sibling subdomains can send same-site POST requests with the user's cookie.
// Non-browser clients without Origin/Fetch Metadata remain supported.
func (handler *Handler) allowBrowserAuthRequest(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		switch r.Header.Get("Sec-Fetch-Site") {
		case "cross-site", "same-site":
			return rejectBrowserAuthRequest(w)
		default:
			return true
		}
	}

	expected := handler.publicOrigin
	if expected == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		expected = scheme + "://" + r.Host
	}
	u, err := url.Parse(origin)
	if err != nil || u.User != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || !strings.EqualFold(origin, expected) {
		return rejectBrowserAuthRequest(w)
	}
	return true
}

func rejectBrowserAuthRequest(w http.ResponseWriter) bool {
	response.Error(w, http.StatusForbidden, "invalid_origin", "request origin is not allowed")
	return false
}
