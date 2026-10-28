package outbox

import (
	"context"
	"errors"
	"testing"
)

type testStore struct {
	pending           []Message
	claimed           bool
	published, failed int
	resultContextErr  error
}

func (s *testStore) ClaimBatch(ctx context.Context, lockID string, limit int) ([]Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit != 1 {
		return nil, errors.New("worker claimed a batch before delivery")
	}
	if s.claimed || len(s.pending) == 0 {
		return nil, nil
	}
	s.claimed = true
	return s.pending[:1], nil
}
func (s *testStore) MarkPublished(ctx context.Context, id, lock string) error {
	s.resultContextErr = ctx.Err()
	s.published++
	return nil
}
func (s *testStore) MarkFailed(ctx context.Context, id, lock string, attempt int, cause error) error {
	s.resultContextErr = ctx.Err()
	s.failed++
	return nil
}

type publishFunc func(context.Context, string, []byte) error

func (f publishFunc) Publish(ctx context.Context, key string, value []byte) error {
	return f(ctx, key, value)
}

func TestCancellationPersistsDeliveryResult(t *testing.T) {
	for _, publishError := range []error{nil, errors.New("Kafka unavailable")} {
		t.Run(map[bool]string{true: "acknowledged", false: "failed"}[publishError == nil], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			store := &testStore{pending: []Message{{ID: "event", AggregateID: "movie", Attempts: 1}}}
			worker := NewWorker(store, publishFunc(func(context.Context, string, []byte) error { cancel(); return publishError }))
			worker.Run(ctx)
			if store.resultContextErr != nil {
				t.Fatalf("delivery update used canceled context: %v", store.resultContextErr)
			}
			if publishError == nil && (store.published != 1 || store.failed != 0) {
				t.Fatalf("ack result not persisted: %+v", store)
			}
			if publishError != nil && (store.published != 0 || store.failed != 1) {
				t.Fatalf("failure incorrectly acknowledged: %+v", store)
			}
		})
	}
}

func TestRetryBackoffBounded(t *testing.T) {
	for _, attempt := range []int{-1, 0, 1, 2, 6, 7, 1000} {
		d := retryDelay(attempt)
		if d.Seconds() < 1 || d.Seconds() > 60 {
			t.Fatalf("attempt=%d delay=%v", attempt, d)
		}
	}
}
