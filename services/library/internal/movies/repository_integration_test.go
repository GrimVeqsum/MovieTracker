//go:build integration

package movies

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"movie-platform/library/internal/outbox"
	"movie-platform/library/internal/testutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testRepositories(t *testing.T) (*pgxpool.Pool, *Repository, *TransactionalRepository) {
	t.Helper()
	db := testutil.Database(t)
	return db, NewRepository(db), NewTransactionalRepository(db, outbox.NewRepository(db))
}
func createTestMovie(t *testing.T, writer *TransactionalRepository, user, title string, year *int) *Movie {
	t.Helper()
	id := uuid.NewString()
	movie, err := writer.Create(context.Background(), CreateMovieWithEventParams{ID: id, UserID: user, Title: title, NormalizedTitle: title, ReleaseYear: year}, testEvent(id, user, "MovieCreated"))
	if err != nil {
		t.Fatal(err)
	}
	return movie
}
func testEvent(movie, user, kind string) Event {
	return Event{EventID: uuid.NewString(), Version: 1, MovieID: movie, UserID: user, Type: kind, OccurredAt: time.Now().UTC()}
}
func count(t *testing.T, db *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestUserIsolationAndAtomicOutbox(t *testing.T) {
	db, repo, writer := testRepositories(t)
	ctx := context.Background()
	owner, intruder := uuid.NewString(), uuid.NewString()
	movie := createTestMovie(t, writer, owner, "private movie", nil)
	if _, err := repo.GetOne(ctx, GetOneParams{ID: movie.ID, UserID: intruder}); !errors.Is(err, ErrMovieNotFound) {
		t.Fatalf("foreign read: %v", err)
	}
	if err := writer.Delete(ctx, DeleteMovieParams{ID: movie.ID, UserID: intruder}, testEvent(movie.ID, intruder, "MovieDeleted")); !errors.Is(err, ErrMovieNotFound) {
		t.Fatalf("foreign delete: %v", err)
	}
	rating := 8
	if _, err := writer.UpdateStatus(ctx, UpdateMovieStatusParams{ID: movie.ID, UserID: intruder, Status: "watched", Rating: &rating}, testEvent(movie.ID, intruder, "MovieWatched")); !errors.Is(err, ErrMovieNotFound) {
		t.Fatalf("foreign update: %v", err)
	}
	if got := count(t, db, "SELECT count(*) FROM outbox_events"); got != 1 {
		t.Fatalf("unauthorized operation wrote event: %d", got)
	}

	// A duplicate outbox primary key fails after the movie INSERT/UPDATE. Every
	// corresponding movie mutation must roll back with the event insert.
	var duplicateEvent string
	if err := db.QueryRow(ctx, "SELECT id FROM outbox_events LIMIT 1").Scan(&duplicateEvent); err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	event := testEvent(id, owner, "MovieCreated")
	event.EventID = duplicateEvent
	if _, err := writer.Create(ctx, CreateMovieWithEventParams{ID: id, UserID: owner, Title: "rolled back", NormalizedTitle: "rolled back"}, event); err == nil {
		t.Fatal("expected outbox conflict")
	}
	if n := count(t, db, "SELECT count(*) FROM movies WHERE id=$1", id); n != 0 {
		t.Fatal("movie committed without event")
	}
	event.MovieID = movie.ID
	event.Type = "MovieDeleted"
	if err := writer.Delete(ctx, DeleteMovieParams{ID: movie.ID, UserID: owner}, event); err == nil {
		t.Fatal("expected delete outbox conflict")
	}
	event.Type = "MovieWatched"
	if _, err := writer.UpdateStatus(ctx, UpdateMovieStatusParams{ID: movie.ID, UserID: owner, Status: "watched", Rating: &rating}, event); err == nil {
		t.Fatal("expected update outbox conflict")
	}
	unchanged, err := repo.GetOne(ctx, GetOneParams{ID: movie.ID, UserID: owner})
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Status != "unwatched" || unchanged.DeletedAt != nil {
		t.Fatalf("mutation committed without event: %+v", unchanged)
	}
	if n := count(t, db, "SELECT count(*) FROM outbox_events"); n != 1 {
		t.Fatalf("outbox count=%d", n)
	}
}

func TestSoftDeleteUniqueAndContextCancellation(t *testing.T) {
	db, repo, writer := testRepositories(t)
	ctx := context.Background()
	user := uuid.NewString()
	movie := createTestMovie(t, writer, user, "same title", nil)
	id := uuid.NewString()
	if _, err := writer.Create(ctx, CreateMovieWithEventParams{ID: id, UserID: user, Title: "same title", NormalizedTitle: "same title"}, testEvent(id, user, "MovieCreated")); !errors.Is(err, ErrMovieAlreadyExists) {
		t.Fatalf("NULL year uniqueness: %v", err)
	}
	if err := writer.Delete(ctx, DeleteMovieParams{ID: movie.ID, UserID: user}, testEvent(movie.ID, user, "MovieDeleted")); err != nil {
		t.Fatal(err)
	}
	createTestMovie(t, writer, user, "same title", nil)
	if _, err := repo.GetOne(ctx, GetOneParams{ID: movie.ID, UserID: user}); !errors.Is(err, ErrMovieNotFound) {
		t.Fatalf("soft deleted read: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	id = uuid.NewString()
	if _, err := writer.Create(canceled, CreateMovieWithEventParams{ID: id, UserID: user, Title: "canceled", NormalizedTitle: "canceled"}, testEvent(id, user, "MovieCreated")); err == nil {
		t.Fatal("canceled transaction succeeded")
	}
	if n := count(t, db, "SELECT count(*) FROM movies WHERE id=$1", id); n != 0 {
		t.Fatal("canceled movie exists")
	}
}

func TestMetadataConcurrentDuplicateAndReplay(t *testing.T) {
	db, repo, writer := testRepositories(t)
	ctx := context.Background()
	user := uuid.NewString()
	movie := createTestMovie(t, writer, user, "metadata", nil)
	eventID := uuid.NewString()
	if err := repo.MarkMetadataFailed(ctx, MarkMetadataFailedParams{EventID: eventID, ID: movie.ID, UserID: user, Error: "temporary upstream failure"}); err != nil {
		t.Fatal(err)
	}
	params := UpdateMetadataIdempotentParams{EventID: eventID, ID: movie.ID, UserID: user, ExternalID: "42", MetadataProvider: "test", OriginalTitle: "Original", Description: "first result", ReleaseYear: 2010, Genres: []string{"Drama", "Mystery", "Drama"}}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- repo.UpdateMetadataIdempotent(ctx, params) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	params.Description = "duplicate must not overwrite"
	if err := repo.UpdateMetadataIdempotent(ctx, params); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkMetadataFailed(ctx, MarkMetadataFailedParams{EventID: eventID, ID: movie.ID, UserID: user, Error: "late failure"}); err != nil {
		t.Fatal(err)
	}
	result, err := repo.GetOne(ctx, GetOneParams{ID: movie.ID, UserID: user})
	if err != nil {
		t.Fatal(err)
	}
	if result.MetadataStatus != "ready" || result.MetadataError != nil || result.Description == nil || *result.Description != "first result" || len(result.Genres) != 2 {
		t.Fatalf("metadata not idempotent: %+v", result)
	}
	if n := count(t, db, "SELECT count(*) FROM processed_events WHERE event_id=$1", eventID); n != 1 {
		t.Fatalf("processed_events=%d", n)
	}
	// A rejected cross-user result must not reserve the event ID either.
	params.EventID = uuid.NewString()
	params.UserID = uuid.NewString()
	if err := repo.UpdateMetadataIdempotent(ctx, params); !errors.Is(err, ErrMovieNotFound) {
		t.Fatalf("foreign metadata: %v", err)
	}
	if n := count(t, db, "SELECT count(*) FROM processed_events WHERE event_id=$1", params.EventID); n != 0 {
		t.Fatal("failed metadata reserved inbox event")
	}
}

func TestMetadataPreservesMovieIdentity(t *testing.T) {
	_, repo, writer := testRepositories(t)
	ctx := context.Background()
	user := uuid.NewString()
	year := 2010
	createTestMovie(t, writer, user, "identity", &year)
	missingYear := createTestMovie(t, writer, user, "identity", nil)
	params := UpdateMetadataIdempotentParams{EventID: uuid.NewString(), ID: missingYear.ID, UserID: user, Description: "valid metadata", ReleaseYear: year}
	if err := repo.UpdateMetadataIdempotent(ctx, params); err != nil {
		t.Fatalf("year collision poisoned event: %v", err)
	}
	result, err := repo.GetOne(ctx, GetOneParams{ID: missingYear.ID, UserID: user})
	if err != nil {
		t.Fatal(err)
	}
	if result.MetadataStatus != "ready" || result.ReleaseYear != nil {
		t.Fatalf("collision must preserve identity: %+v", result)
	}
	specifiedYear := createTestMovie(t, writer, user, "explicit year", &year)
	params.ID = specifiedYear.ID
	params.EventID = uuid.NewString()
	params.ReleaseYear = 2020
	if err := repo.UpdateMetadataIdempotent(ctx, params); err != nil {
		t.Fatal(err)
	}
	result, err = repo.GetOne(ctx, GetOneParams{ID: specifiedYear.ID, UserID: user})
	if err != nil {
		t.Fatal(err)
	}
	if result.ReleaseYear == nil || *result.ReleaseYear != year {
		t.Fatalf("metadata overwrote user year: %+v", result)
	}
}

func TestConcurrentStatusAndDeleteNeverResurrectsMovie(t *testing.T) {
	db, repo, writer := testRepositories(t)
	ctx := context.Background()
	user := uuid.NewString()
	movie := createTestMovie(t, writer, user, "concurrent delete", nil)
	const writers = 8
	outcomes := make(chan error, writers+1)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rating := 7
			_, err := writer.UpdateStatus(ctx, UpdateMovieStatusParams{ID: movie.ID, UserID: user, Status: "watched", Rating: &rating}, testEvent(movie.ID, user, "MovieWatched"))
			outcomes <- err
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		outcomes <- writer.Delete(ctx, DeleteMovieParams{ID: movie.ID, UserID: user}, testEvent(movie.ID, user, "MovieDeleted"))
	}()
	wg.Wait()
	close(outcomes)
	committed := 1
	for err := range outcomes {
		if err == nil {
			committed++
		} else if !errors.Is(err, ErrMovieNotFound) {
			t.Fatal(err)
		}
	}
	if _, err := repo.GetOne(ctx, GetOneParams{ID: movie.ID, UserID: user}); !errors.Is(err, ErrMovieNotFound) {
		t.Fatalf("deleted movie reappeared: %v", err)
	}
	if n := count(t, db, "SELECT count(*) FROM outbox_events WHERE aggregate_id=$1", movie.ID); n != committed {
		t.Fatalf("outbox=%d committed operations=%d", n, committed)
	}
}
