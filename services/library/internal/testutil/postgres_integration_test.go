//go:build integration

package testutil

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMigrationsRollbackAndReapply(t *testing.T) {
	pool := Database(t)
	_, file, _, _ := runtime.Caller(0)
	directory := filepath.Join(filepath.Dir(file), "../../migrations")
	down, err := filepath.Glob(filepath.Join(directory, "*.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for i := len(down) - 1; i >= 0; i-- {
		data, err := os.ReadFile(down[i])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(), string(data)); err != nil {
			t.Fatalf("rollback %s: %v", filepath.Base(down[i]), err)
		}
	}
	up, err := filepath.Glob(filepath.Join(directory, "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range up {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(), string(data)); err != nil {
			t.Fatalf("reapply %s: %v", filepath.Base(path), err)
		}
	}
}
