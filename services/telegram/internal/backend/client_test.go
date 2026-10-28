package backend

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTelegramMovieFlowWithLocalBackends(t *testing.T) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/telegram/token" || r.Header.Get("X-Service-Token") != "test-service-secret" {
			t.Error("missing internal authentication")
		}
		var body struct {
			TelegramUserID int64 `json:"telegram_user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TelegramUserID != 42 {
			t.Errorf("invalid body: %v %+v", err, body)
		}
		w.Write([]byte(`{"access_token":"test-access","token_type":"Bearer"}`))
	}))
	defer auth.Close()
	library := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-access" || r.Header.Get("X-Service-Token") != "" {
			t.Error("wrong library authentication")
		}
		if r.Method == "POST" {
			w.WriteHeader(201)
			w.Write([]byte(`{"id":"movie-1","title":"Inception"}`))
			return
		}
		w.Write([]byte(`[{"id":"movie-1","title":"Inception"}]`))
	}))
	defer library.Close()
	client := NewClient(auth.URL, library.URL, "test-service-secret")
	movie, err := client.AddMovie(context.Background(), 42, "Inception", nil)
	if err != nil || movie.ID != "movie-1" {
		t.Fatalf("add: %v %v", movie, err)
	}
	movies, err := client.GetMovies(context.Background(), 42)
	if err != nil || len(movies) != 1 {
		t.Fatalf("list: %v %v", movies, err)
	}
}

func TestTelegramUnlinkedUserDoesNotReachLibrary(t *testing.T) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error":{"code":"telegram_not_linked"}}`))
	}))
	defer auth.Close()
	library := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("unlinked user reached library") }))
	defer library.Close()
	_, err := NewClient(auth.URL, library.URL, "test-secret").GetMovies(context.Background(), 42)
	if !errors.Is(err, ErrTelegramNotLinked) {
		t.Fatalf("got %v", err)
	}
}
