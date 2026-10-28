package poiskkino

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFindMovieUsesLocalAPIAndMatchesYear(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-KEY") != "test-key" {
			t.Error("missing API key")
		}
		switch r.URL.Path {
		case "/movie/search":
			if r.URL.Query().Get("query") != "A & B" {
				t.Errorf("incorrect escaped query: %s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"docs":[{"id":1,"year":2000},{"id":2,"year":2010}]}`))
		case "/movie/2":
			w.Write([]byte(`{"id":2,"name":"A & B","year":2010}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	year := 2010
	got, err := NewClientWithBaseURL("test-key", server.URL).FindMovie(context.Background(), "A & B", &year)
	if err != nil || got.ID != 2 {
		t.Fatalf("movie=%v error=%v", got, err)
	}
}

func TestFindMovieFailureModes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		body     string
		notFound bool
	}{
		{"not found", 200, `{"docs":[]}`, true},
		{"malformed", 200, `broken`, false},
		{"rate limited", 429, `private key`, false},
		{"unavailable", 503, `private key`, false},
		{"oversized", 200, strings.Repeat(" ", (2<<20)+1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer server.Close()
			_, err := NewClientWithBaseURL("test-key", server.URL).FindMovie(context.Background(), "movie", nil)
			if err == nil {
				t.Fatal("expected error")
			}
			if tc.notFound && !errors.Is(err, ErrMovieNotFound) {
				t.Fatalf("not found: %v", err)
			}
			if strings.Contains(err.Error(), "private key") {
				t.Fatal("response body leaked")
			}
		})
	}
}

func TestClientDoesNotFollowRedirectOrIgnoreCancellation(t *testing.T) {
	calls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer server.Close()
	client := NewClientWithBaseURL("secret", server.URL)
	if _, err := client.FindMovie(context.Background(), "movie", nil); err == nil {
		t.Fatal("expected redirect error")
	}
	if calls != 0 {
		t.Fatal("followed redirect with API key")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.FindMovie(ctx, "movie", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}
