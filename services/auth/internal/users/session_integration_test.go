//go:build integration

package users

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Each test creates and drops only its own random schema in the isolated test DB.
func integrationRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("AUTH_TEST_DATABASE_URL must point to the isolated integration database")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "auth_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
		admin.Close()
	})
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.MaxConns = 20
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	files, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("find migrations: %v", err)
	}
	for _, file := range files {
		migration, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("apply %s: %v", file, err)
		}
	}
	return NewRepository(pool)
}

func integrationSession(t *testing.T, repo *Repository) (*Service, string) {
	t.Helper()
	user, err := repo.Create(context.Background(), CreateUserParams{
		ID: uuid.NewString(), Email: uuid.NewString() + "@example.test", PasswordHash: "not-used",
	})
	if err != nil {
		t.Fatal(err)
	}
	token, err := generateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateRefreshSession(context.Background(), CreateRefreshSessionParams{
		ID: uuid.NewString(), UserID: user.ID, TokenHash: hashRefreshToken(token), ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	return NewService(repo, strings.Repeat("t", 32), "movietracker-auth", "movietracker-api"), token
}

func TestConcurrentRefreshSingleWinner(t *testing.T) {
	repo := integrationRepository(t)
	service, token := integrationSession(t, repo)
	start := make(chan struct{})
	var wg sync.WaitGroup
	var successes atomic.Int32
	var winner string
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := service.Refresh(context.Background(), token)
			if err == nil {
				successes.Add(1)
				winner = result.RefreshToken
			} else if !errors.Is(err, ErrInvalidRefreshToken) {
				t.Errorf("unexpected refresh error: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("successful rotations=%d, expected exactly one", successes.Load())
	}
	if _, err := service.Refresh(context.Background(), winner); err != nil {
		t.Fatalf("winning token unusable: %v", err)
	}
}

func TestLogoutRevokesSessionAcrossRotation(t *testing.T) {
	repo := integrationRepository(t)
	for _, rotateFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("rotateFirst=%v", rotateFirst), func(t *testing.T) {
			service, original := integrationSession(t, repo)
			current := original
			if rotateFirst {
				result, err := service.Refresh(context.Background(), original)
				if err != nil {
					t.Fatal(err)
				}
				current = result.RefreshToken
			}
			if err := service.Logout(context.Background(), original); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Refresh(context.Background(), current); !errors.Is(err, ErrInvalidRefreshToken) {
				t.Fatalf("logged-out session refreshed: %v", err)
			}
			if err := service.Logout(context.Background(), original); err != nil {
				t.Fatalf("logout must be idempotent: %v", err)
			}
		})
	}
}

func TestLogoutConcurrentWithRefresh(t *testing.T) {
	repo := integrationRepository(t)
	for i := 0; i < 20; i++ {
		service, original := integrationSession(t, repo)
		var result *RefreshResult
		var refreshErr, logoutErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); result, refreshErr = service.Refresh(context.Background(), original) }()
		go func() { defer wg.Done(); logoutErr = service.Logout(context.Background(), original) }()
		wg.Wait()
		if logoutErr != nil || (refreshErr != nil && !errors.Is(refreshErr, ErrInvalidRefreshToken)) {
			t.Fatalf("refresh=%v logout=%v", refreshErr, logoutErr)
		}
		current := original
		if result != nil {
			current = result.RefreshToken
		}
		if _, err := service.Refresh(context.Background(), current); !errors.Is(err, ErrInvalidRefreshToken) {
			t.Fatalf("session alive after concurrent logout: %v", err)
		}
	}
}

func TestRefreshRotationRollsBackOnTokenHistoryFailure(t *testing.T) {
	repo := integrationRepository(t)
	service, token := integrationSession(t, repo)
	_, err := repo.db.Exec(context.Background(), `
		CREATE FUNCTION reject_test_token() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'injected history failure'; END $$;
		CREATE TRIGGER reject_test_token BEFORE INSERT ON refresh_session_tokens
		FOR EACH ROW EXECUTE FUNCTION reject_test_token();
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refresh(context.Background(), token); err == nil {
		t.Fatal("expected injected failure")
	}
	if _, err := repo.db.Exec(context.Background(), "DROP TRIGGER reject_test_token ON refresh_session_tokens"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refresh(context.Background(), token); err != nil {
		t.Fatalf("failed rotation consumed original token: %v", err)
	}
}

func TestLateRefreshFailureDoesNotClearWinningCookie(t *testing.T) {
	repo := integrationRepository(t)
	service, original := integrationSession(t, repo)
	if _, err := service.Refresh(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(service, false, "")
	r := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	r.AddCookie(&http.Cookie{Name: refreshCookieName, Value: original})
	w := httptest.NewRecorder()
	handler.Refresh(w, r)
	if w.Code != http.StatusUnauthorized || len(w.Result().Cookies()) != 0 {
		t.Fatalf("late failure overwrote winning cookie: status=%d cookies=%v", w.Code, w.Result().Cookies())
	}
	if strings.Contains(w.Body.String(), original) {
		t.Fatal("token leaked in response")
	}
}

func TestCanceledRefreshPreservesSession(t *testing.T) {
	repo := integrationRepository(t)
	service, token := integrationSession(t, repo)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Refresh(ctx, token); err == nil {
		t.Fatal("canceled refresh succeeded")
	}
	if _, err := service.Refresh(context.Background(), token); err != nil {
		t.Fatalf("canceled rotation consumed token: %v", err)
	}
}

func TestExpiredAndRevokedRefreshTokensAreRejected(t *testing.T) {
	repo := integrationRepository(t)
	for _, expired := range []bool{false, true} {
		service, token := integrationSession(t, repo)
		query := "UPDATE refresh_sessions SET revoked_at = NOW() WHERE refresh_token_hash = $1"
		if expired {
			query = "UPDATE refresh_sessions SET expires_at = NOW() - INTERVAL '1 minute' WHERE refresh_token_hash = $1"
		}
		if _, err := repo.db.Exec(context.Background(), query, hashRefreshToken(token)); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Refresh(context.Background(), token); !errors.Is(err, ErrInvalidRefreshToken) {
			t.Fatalf("expired=%v token was accepted: %v", expired, err)
		}
	}
}

func TestTokenHistoryMigrationBackfillsExistingSessions(t *testing.T) {
	repo := integrationRepository(t)
	service, token := integrationSession(t, repo)
	// Only this test's random schema is affected. Simulate a session created by
	// the previous application version before migration 000004 is deployed.
	if _, err := repo.db.Exec(context.Background(), "DROP TABLE refresh_session_tokens"); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../migrations/000004_refresh_session_tokens.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.db.Exec(context.Background(), string(migration)); err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refresh(context.Background(), token); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("pre-migration session was not revoked: %v", err)
	}
}

func legacyIntegrationSession(t *testing.T, repo *Repository) (*Service, string) {
	t.Helper()
	service, token := integrationSession(t, repo)
	// Simulate the previous binary creating/rotating a session between the
	// migration backfill and deployment of the new application image.
	if _, err := repo.db.Exec(context.Background(), "DELETE FROM refresh_session_tokens WHERE token_hash = $1", hashRefreshToken(token)); err != nil {
		t.Fatal(err)
	}
	return service, token
}

func TestMigrationGapSessionCanBeRevoked(t *testing.T) {
	repo := integrationRepository(t)
	service, token := legacyIntegrationSession(t, repo)
	if err := service.Logout(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refresh(context.Background(), token); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("legacy session was not revoked: %v", err)
	}
}

func TestMigrationGapRotationKeepsPreviousTokenAssociation(t *testing.T) {
	repo := integrationRepository(t)
	service, previous := legacyIntegrationSession(t, repo)
	result, err := service.Refresh(context.Background(), previous)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(context.Background(), previous); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refresh(context.Background(), result.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("rotated legacy session was not revoked: %v", err)
	}
}

func TestMigrationGapLogoutConcurrentWithRotation(t *testing.T) {
	repo := integrationRepository(t)
	for i := 0; i < 20; i++ {
		service, previous := legacyIntegrationSession(t, repo)
		var result *RefreshResult
		var refreshErr, logoutErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); result, refreshErr = service.Refresh(context.Background(), previous) }()
		go func() { defer wg.Done(); logoutErr = service.Logout(context.Background(), previous) }()
		wg.Wait()
		if logoutErr != nil || (refreshErr != nil && !errors.Is(refreshErr, ErrInvalidRefreshToken)) {
			t.Fatalf("legacy race refresh=%v logout=%v", refreshErr, logoutErr)
		}
		current := previous
		if result != nil {
			current = result.RefreshToken
		}
		if _, err := service.Refresh(context.Background(), current); !errors.Is(err, ErrInvalidRefreshToken) {
			t.Fatalf("legacy race kept session active: %v", err)
		}
	}
}
