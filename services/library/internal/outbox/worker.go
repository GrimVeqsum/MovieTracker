package outbox

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
)

type Publisher interface {
	Publish(
		ctx context.Context,
		key string,
		payload []byte,
	) error
}

type Worker struct {
	repo *Repository

	publisher Publisher

	batchSize int

	pollInterval time.Duration
}

func NewWorker(
	repo *Repository,
	publisher Publisher,
) *Worker {
	return &Worker{
		repo: repo,

		publisher: publisher,

		batchSize: 20,

		pollInterval: 1 * time.Second,
	}
}

func (worker *Worker) Run(
	ctx context.Context,
) {
	log.Println(
		"outbox worker started",
	)

	defer log.Println(
		"outbox worker stopped",
	)

	for {
		if ctx.Err() != nil {
			return
		}

		processed, err :=
			worker.processBatch(
				ctx,
			)

		if err != nil {
			if ctx.Err() != nil {
				return
			}

			log.Printf(
				"outbox worker error: %v",
				err,
			)
		}

		if processed > 0 {
			continue
		}

		timer :=
			time.NewTimer(
				worker.pollInterval,
			)

		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}

			return

		case <-timer.C:
		}
	}
}

func (worker *Worker) processBatch(
	ctx context.Context,
) (int, error) {
	lockID :=
		uuid.NewString()

	messages, err :=
		worker.repo.ClaimBatch(
			ctx,
			lockID,
			worker.batchSize,
		)

	if err != nil {
		return 0, err
	}

	for _, message := range messages {

		err :=
			worker.publisher.Publish(
				ctx,
				message.AggregateID,
				message.Payload,
			)

		if err != nil {
			log.Printf(
				"outbox publish failed: event_id=%s event_type=%s attempt=%d error=%v",
				message.ID,
				message.EventType,
				message.Attempts,
				err,
			)

			markErr :=
				worker.repo.MarkFailed(
					ctx,
					message.ID,
					lockID,
					message.Attempts,
					err,
				)

			if markErr != nil {
				log.Printf(
					"mark outbox failure failed: event_id=%s error=%v",
					message.ID,
					markErr,
				)
			}

			continue
		}

		err =
			worker.repo.MarkPublished(
				ctx,
				message.ID,
				lockID,
			)

		if err != nil {
			log.Printf(
				"mark outbox published failed: event_id=%s error=%v",
				message.ID,
				err,
			)

			continue
		}

		log.Printf(
			"outbox event published: event_id=%s event_type=%s aggregate_id=%s",
			message.ID,
			message.EventType,
			message.AggregateID,
		)
	}

	return len(messages), nil
}
