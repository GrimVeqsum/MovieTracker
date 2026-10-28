package postgres

import (
	"context"
	"strings"
	"testing"
)

func TestMalformedDatabaseURLDoesNotLeakPassword(t *testing.T) {
	pool, err := NewConnection(context.Background(), "postgres://user:fixture-password%zz@localhost/db")
	if pool != nil || err == nil {
		t.Fatalf("expected configuration error, pool=%v err=%v", pool, err)
	}
	if strings.Contains(err.Error(), "fixture-password") || strings.Contains(err.Error(), "postgres://") {
		t.Fatalf("DSN leaked: %s", err)
	}
}
