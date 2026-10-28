package postgres

import (
	"context"
	"strings"
	"testing"
)

func TestInvalidDatabaseURLDoesNotExposePassword(t *testing.T) {
	const password = "sensitive-test-password"
	pool, err := NewConnection(context.Background(), "postgres://user:"+password+"@localhost:invalid/db")
	if pool != nil {
		pool.Close()
		t.Fatal("invalid configuration returned a pool")
	}
	if err == nil {
		t.Fatal("invalid port accepted")
	}
	if strings.Contains(err.Error(), password) {
		t.Fatalf("database error exposes password: %v", err)
	}
}
