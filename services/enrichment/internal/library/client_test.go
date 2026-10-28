package library

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientResponseAndServiceCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(serviceTokenHeader) != "test-service-secret" {
			t.Error("missing service authentication")
		}
		if r.URL.Path != "/internal/movies/movie/metadata" {
			t.Errorf("path=%s", r.URL.Path)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("sensitive upstream detail"))
	}))
	defer server.Close()
	err := NewClient(server.URL, "test-service-secret").UpdateMetadata(context.Background(), "movie", UpdateMetadataRequest{})
	var statusErr *HTTPError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != 503 {
		t.Fatalf("error=%v", err)
	}
	if strings.Contains(err.Error(), "sensitive") {
		t.Fatal("upstream body leaked")
	}
}

func TestClientDoesNotForwardServiceTokenOnRedirect(t *testing.T) {
	calls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer server.Close()
	if err := NewClient(server.URL, "secret").MarkMetadataFailed(context.Background(), "movie", MarkMetadataFailedRequest{}); err == nil {
		t.Fatal("expected redirect error")
	}
	if calls != 0 {
		t.Fatal("followed redirect")
	}
}
