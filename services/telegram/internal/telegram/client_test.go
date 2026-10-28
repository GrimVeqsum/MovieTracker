package telegram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTelegramLocalAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/getMe":
			w.Write([]byte(`{"ok":true,"result":{"id":42,"username":"test_bot"}}`))
		case "/getUpdates":
			if r.URL.Query().Get("offset") != "15" || r.URL.Query().Get("timeout") != "30" {
				t.Errorf("unexpected poll query %s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"ok":true,"result":[{"update_id":15,"message":{"from":{"id":9},"chat":{"id":9},"text":"/movies"}}]}`))
		case "/sendMessage":
			if r.Method != "POST" {
				t.Error("expected POST")
			}
			w.Write([]byte(`{"ok":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewClient("test-token")
	client.baseURL = server.URL
	user, err := client.GetMe(context.Background())
	if err != nil || user.ID != 42 {
		t.Fatalf("getMe: %v %v", user, err)
	}
	updates, err := client.GetUpdates(context.Background(), 15)
	if err != nil || len(updates) != 1 || updates[0].Message.From.ID != 9 {
		t.Fatalf("getUpdates: %v %v", updates, err)
	}
	if err := client.SendMessage(context.Background(), 9, "ready"); err != nil {
		t.Fatal(err)
	}
}

func TestTelegramTransportErrorsDoNotExposeToken(t *testing.T) {
	client := NewClient("VERY-SECRET-TOKEN")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.GetMe(ctx)
	if err == nil || strings.Contains(err.Error(), "VERY-SECRET-TOKEN") || !errors.Is(err, context.Canceled) {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestTelegramRejectsRedirectAndHTTPFailure(t *testing.T) {
	for _, code := range []int{302, 500} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "http://127.0.0.1:1/stolen")
				w.WriteHeader(code)
				w.Write([]byte(`{"ok":true,"result":{"id":42}}`))
			}))
			defer server.Close()
			client := NewClient("test-token")
			client.baseURL = server.URL
			if _, err := client.GetMe(context.Background()); err == nil {
				t.Fatal("expected HTTP error")
			}
		})
	}
}
