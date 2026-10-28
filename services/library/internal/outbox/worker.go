package outbox

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
)

type Publisher interface {
	Publish(ctx context.Context, key string, payload []byte) error
}

// Store keeps delivery state in PostgreSQL. Publishing and marking cannot be
// one transaction: a crash between them deliberately causes redelivery.
type Store interface {
	ClaimBatch(context.Context, string, int) ([]Message, error)
	MarkPublished(context.Context, string, string) error
	MarkFailed(context.Context, string, string, int, error) error
}

type Worker struct {
	repo         Store
	publisher    Publisher
	batchSize    int
	pollInterval time.Duration
}

func NewWorker(repo Store, publisher Publisher) *Worker {
	return &Worker{repo: repo, publisher: publisher, batchSize: 20, pollInterval: time.Second}
}

func (worker *Worker) Run(ctx context.Context) {
	log.Println("outbox worker started")
	defer log.Println("outbox worker stopped")
	for ctx.Err() == nil {
		processed, err := worker.processBatch(ctx)
		if err != nil && ctx.Err() == nil {
			log.Printf("outbox worker error: %v", err)
		}
		if processed > 0 {
			continue
		}
		timer := time.NewTimer(worker.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (worker *Worker) processBatch(ctx context.Context) (int, error) {
	processed := 0
	for processed < worker.batchSize && ctx.Err() == nil {
		lockID := uuid.NewString()
		// Claim just before publishing. Claiming twenty records at once allowed
		// their one-minute leases to expire while earlier ten-second sends retried.
		messages, err := worker.repo.ClaimBatch(ctx, lockID, 1)
		if err != nil {
			return processed, err
		}
		if len(messages) == 0 {
			return processed, nil
		}
		message := messages[0]
		publishErr := worker.publisher.Publish(ctx, message.AggregateID, message.Payload)
		// Cancellation stops new work, but recording an acknowledged result must
		// still finish before the caller closes PostgreSQL and the producer.
		resultCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		if publishErr != nil {
			err = worker.repo.MarkFailed(resultCtx, message.ID, lockID, message.Attempts, publishErr)
			log.Printf("outbox publish failed: event_id=%s attempt=%d", message.ID, message.Attempts)
		} else {
			err = worker.repo.MarkPublished(resultCtx, message.ID, lockID)
		}
		cancel()
		processed++
		if err != nil {
			return processed, err
		}
	}
	return processed, nil
}
