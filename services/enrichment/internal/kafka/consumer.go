package kafka

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"movie-platform/enrichment/internal/events"
	"movie-platform/enrichment/internal/library"
	"movie-platform/enrichment/internal/poiskkino"

	"github.com/twmb/franz-go/pkg/kgo"
)

const maxProcessingAttempts = 5

type MovieProvider interface {
	FindMovie(
		ctx context.Context,
		title string,
		releaseYear *int,
	) (*poiskkino.MovieDetails, error)
}

type LibraryClient interface {
	UpdateMetadata(
		ctx context.Context,
		movieID string,
		request library.UpdateMetadataRequest,
	) error

	MarkMetadataFailed(
		ctx context.Context,
		movieID string,
		request library.MarkMetadataFailedRequest,
	) error
}

type Consumer struct {
	client *kgo.Client

	dlqTopic string

	movieProvider MovieProvider

	libraryClient LibraryClient
}

func NewConsumer(
	broker string,
	topic string,
	dlqTopic string,
	group string,
	movieProvider MovieProvider,
	libraryClient LibraryClient,
) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(
			broker,
		),

		kgo.ConsumerGroup(
			group,
		),

		kgo.ConsumeTopics(
			topic,
		),

		kgo.ConsumeResetOffset(
			kgo.NewOffset().
				AtStart(),
		),

		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil,
			fmt.Errorf(
				"create Kafka consumer: %w",
				err,
			)
	}

	return &Consumer{
		client: client,

		dlqTopic: dlqTopic,

		movieProvider: movieProvider,

		libraryClient: libraryClient,
	}, nil
}

func (consumer *Consumer) Run(
	ctx context.Context,
) {
	for {
		if ctx.Err() != nil {
			return
		}

		fetches := consumer.client.PollRecords(
			ctx,
			1,
		)

		if ctx.Err() != nil {
			return
		}

		if fetchErrors := fetches.Errors(); len(fetchErrors) > 0 {

			for _, fetchErr := range fetchErrors {
				log.Printf(
					"Kafka poll error: %v",
					fetchErr,
				)
			}

			continue
		}

		for _, record := range fetches.Records() {
			err := consumer.handleRecord(
				ctx,
				record,
			)
			if err != nil {
				if ctx.Err() != nil {
					return
				}

				log.Printf(
					"Kafka record was not resolved: topic=%s partition=%d offset=%d error=%v",
					record.Topic,
					record.Partition,
					record.Offset,
					err,
				)

				continue
			}

			if err := consumer.commitUntilSuccess(
				ctx,
				record,
			); err != nil {

				if ctx.Err() != nil {
					return
				}

				log.Printf(
					"Kafka commit stopped: %v",
					err,
				)

				return
			}
		}
	}
}

func (consumer *Consumer) handleRecord(
	ctx context.Context,
	record *kgo.Record,
) error {
	var lastErr error

	for attempt := 1; attempt <= maxProcessingAttempts; attempt++ {

		err := consumer.process(
			ctx,
			record,
		)

		if err == nil {
			return nil
		}

		lastErr = err

		var permanentErr *permanentProcessingError

		if errors.As(
			err,
			&permanentErr,
		) {
			log.Printf(
				"non-retryable event error: topic=%s partition=%d offset=%d error=%v",
				record.Topic,
				record.Partition,
				record.Offset,
				err,
			)

			consumer.markFailedBestEffort(
				ctx,
				record,
				err,
			)

			return consumer.publishDLQUntilSuccess(
				ctx,
				record,
				err,
				attempt,
			)
		}

		log.Printf(
			"event processing failed: topic=%s partition=%d offset=%d attempt=%d/%d error=%v",
			record.Topic,
			record.Partition,
			record.Offset,
			attempt,
			maxProcessingAttempts,
			err,
		)

		if attempt ==
			maxProcessingAttempts {

			break
		}

		if err := waitForRetry(
			ctx,
			processingRetryDelay(
				attempt,
			),
		); err != nil {

			return err
		}
	}

	log.Printf(
		"event retries exhausted: topic=%s partition=%d offset=%d",
		record.Topic,
		record.Partition,
		record.Offset,
	)

	consumer.markFailedBestEffort(
		ctx,
		record,
		lastErr,
	)

	return consumer.publishDLQUntilSuccess(
		ctx,
		record,
		lastErr,
		maxProcessingAttempts,
	)
}

func (consumer *Consumer) process(
	ctx context.Context,
	record *kgo.Record,
) error {
	var event events.MovieEvent

	if err := json.Unmarshal(
		record.Value,
		&event,
	); err != nil {

		return newPermanentProcessingError(
			fmt.Errorf(
				"invalid Kafka event JSON: %w",
				err,
			),
		)
	}

	if event.Type != "MovieCreated" {
		return nil
	}

	if strings.TrimSpace(
		event.EventID,
	) == "" {

		return newPermanentProcessingError(
			errors.New(
				"MovieCreated has empty event_id",
			),
		)
	}

	if strings.TrimSpace(
		event.MovieID,
	) == "" {

		return newPermanentProcessingError(
			errors.New(
				"MovieCreated has empty movie_id",
			),
		)
	}

	if strings.TrimSpace(
		event.UserID,
	) == "" {

		return newPermanentProcessingError(
			errors.New(
				"MovieCreated has empty user_id",
			),
		)
	}

	if strings.TrimSpace(
		event.Title,
	) == "" {

		return newPermanentProcessingError(
			errors.New(
				"MovieCreated has empty title",
			),
		)
	}

	log.Printf(
		"MovieCreated received: event_id=%s movie_id=%s title=%q",
		event.EventID,
		event.MovieID,
		event.Title,
	)

	movie, err := consumer.movieProvider.FindMovie(
		ctx,
		event.Title,
		event.ReleaseYear,
	)
	if err != nil {
		if errors.Is(
			err,
			poiskkino.ErrMovieNotFound,
		) {
			log.Printf(
				"movie not found: event_id=%s movie_id=%s title=%q",
				event.EventID,
				event.MovieID,
				event.Title,
			)

			err := consumer.libraryClient.MarkMetadataFailed(
				ctx,
				event.MovieID,
				library.MarkMetadataFailedRequest{
					EventID: event.EventID,

					UserID: event.UserID,

					Error: "movie not found in poiskkino",
				},
			)
			if err != nil {
				return fmt.Errorf(
					"mark movie metadata failed: %w",
					err,
				)
			}

			log.Printf(
				"movie metadata marked failed: event_id=%s movie_id=%s reason=not_found",
				event.EventID,
				event.MovieID,
			)

			return nil
		}

		return fmt.Errorf(
			"find movie metadata: %w",
			err,
		)
	}

	log.Printf(
		"movie found: id=%d name=%q year=%d",
		movie.ID,
		movie.Name,
		movie.Year,
	)

	if movie.MovieLength != nil {
		log.Printf(
			"runtime: %d minutes",
			*movie.MovieLength,
		)
	}

	genres := make(
		[]string,
		0,
		len(movie.Genres),
	)

	for _, genre := range movie.Genres {
		genreName := strings.TrimSpace(
			genre.Name,
		)

		if genreName == "" {
			continue
		}

		genres = append(
			genres,
			genreName,
		)
	}

	err = consumer.libraryClient.UpdateMetadata(
		ctx,
		event.MovieID,
		library.UpdateMetadataRequest{
			EventID: event.EventID,

			UserID: event.UserID,

			ExternalID: strconv.Itoa(
				movie.ID,
			),

			MetadataProvider: "poiskkino",

			OriginalTitle: movie.AlternativeName,

			Description: movie.Description,

			ReleaseYear: movie.Year,

			PosterURL: movie.Poster.URL,

			RuntimeMinutes: movie.MovieLength,

			Genres: genres,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"save movie metadata: %w",
			err,
		)
	}

	log.Printf(
		"movie metadata saved: event_id=%s movie_id=%s",
		event.EventID,
		event.MovieID,
	)

	return nil
}

func (consumer *Consumer) markFailedBestEffort(
	ctx context.Context,
	record *kgo.Record,
	processingErr error,
) {
	var event events.MovieEvent

	if err := json.Unmarshal(
		record.Value,
		&event,
	); err != nil {

		return
	}

	if event.Type != "MovieCreated" {
		return
	}

	if strings.TrimSpace(
		event.EventID,
	) == "" ||
		strings.TrimSpace(
			event.MovieID,
		) == "" ||
		strings.TrimSpace(
			event.UserID,
		) == "" {

		return
	}

	err := consumer.libraryClient.MarkMetadataFailed(
		ctx,
		event.MovieID,
		library.MarkMetadataFailedRequest{
			EventID: event.EventID,

			UserID: event.UserID,

			Error: truncateError(
				processingErr,
			),
		},
	)
	if err != nil {
		log.Printf(
			"failed to mark movie metadata as failed: event_id=%s movie_id=%s error=%v",
			event.EventID,
			event.MovieID,
			err,
		)

		return
	}

	log.Printf(
		"movie metadata marked failed: event_id=%s movie_id=%s",
		event.EventID,
		event.MovieID,
	)
}

type deadLetterMessage struct {
	SourceTopic string `json:"source_topic"`

	SourcePartition int32 `json:"source_partition"`

	SourceOffset int64 `json:"source_offset"`

	KeyBase64 string `json:"key_base64"`

	ValueBase64 string `json:"value_base64"`

	Error string `json:"error"`

	Attempts int `json:"attempts"`

	FailedAt time.Time `json:"failed_at"`
}

func (consumer *Consumer) publishDLQUntilSuccess(
	ctx context.Context,
	record *kgo.Record,
	processingErr error,
	attempts int,
) error {
	dlqAttempt := 1

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := consumer.publishDLQ(
			ctx,
			record,
			processingErr,
			attempts,
		)

		if err == nil {
			log.Printf(
				"event moved to DLQ: source_topic=%s partition=%d offset=%d dlq_topic=%s",
				record.Topic,
				record.Partition,
				record.Offset,
				consumer.dlqTopic,
			)

			return nil
		}

		log.Printf(
			"DLQ publish failed: attempt=%d error=%v",
			dlqAttempt,
			err,
		)

		if err := waitForRetry(
			ctx,
			dlqRetryDelay(
				dlqAttempt,
			),
		); err != nil {

			return err
		}

		dlqAttempt++
	}
}

func (consumer *Consumer) publishDLQ(
	ctx context.Context,
	record *kgo.Record,
	processingErr error,
	attempts int,
) error {
	message := deadLetterMessage{
		SourceTopic: record.Topic,

		SourcePartition: record.Partition,

		SourceOffset: record.Offset,

		KeyBase64: base64.StdEncoding.
			EncodeToString(
				record.Key,
			),

		ValueBase64: base64.StdEncoding.
			EncodeToString(
				record.Value,
			),

		Error: truncateError(
			processingErr,
		),

		Attempts: attempts,

		FailedAt: time.Now().UTC(),
	}

	payload, err := json.Marshal(
		message,
	)
	if err != nil {
		return fmt.Errorf(
			"marshal DLQ message: %w",
			err,
		)
	}

	publishCtx, cancel := context.WithTimeout(
		ctx,
		10*time.Second,
	)
	defer cancel()

	recordToDLQ := &kgo.Record{
		Topic: consumer.dlqTopic,

		Key: record.Key,

		Value: payload,
	}

	err = consumer.client.
		ProduceSync(
			publishCtx,
			recordToDLQ,
		).
		FirstErr()

	if err != nil {
		return fmt.Errorf(
			"publish DLQ record: %w",
			err,
		)
	}

	return nil
}

func (consumer *Consumer) commitUntilSuccess(
	ctx context.Context,
	record *kgo.Record,
) error {
	attempt := 1

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := consumer.client.CommitRecords(
			ctx,
			record,
		)

		if err == nil {
			return nil
		}

		log.Printf(
			"Kafka commit failed: topic=%s partition=%d offset=%d attempt=%d error=%v",
			record.Topic,
			record.Partition,
			record.Offset,
			attempt,
			err,
		)

		if err := waitForRetry(
			ctx,
			commitRetryDelay(
				attempt,
			),
		); err != nil {

			return err
		}

		attempt++
	}
}

type permanentProcessingError struct {
	err error
}

func (
	err *permanentProcessingError,
) Error() string {
	return err.err.Error()
}

func (
	err *permanentProcessingError,
) Unwrap() error {
	return err.err
}

func newPermanentProcessingError(
	err error,
) error {
	return &permanentProcessingError{
		err: err,
	}
}

func processingRetryDelay(
	attempt int,
) time.Duration {
	switch attempt {
	case 1:
		return 2 * time.Second

	case 2:
		return 4 * time.Second

	case 3:
		return 8 * time.Second

	default:
		return 16 * time.Second
	}
}

func dlqRetryDelay(
	attempt int,
) time.Duration {
	delay := time.Second *
		time.Duration(
			1<<minInt(
				attempt-1,
				5,
			),
		)

	if delay > 30*time.Second {
		return 30 * time.Second
	}

	return delay
}

func commitRetryDelay(
	attempt int,
) time.Duration {
	delay := time.Second *
		time.Duration(
			1<<minInt(
				attempt-1,
				4,
			),
		)

	if delay > 10*time.Second {
		return 10 * time.Second
	}

	return delay
}

func waitForRetry(
	ctx context.Context,
	delay time.Duration,
) error {
	timer := time.NewTimer(
		delay,
	)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-timer.C:
		return nil
	}
}

func truncateError(
	err error,
) string {
	if err == nil {
		return ""
	}

	const maxRunes = 2000

	runes := []rune(
		err.Error(),
	)

	if len(runes) <= maxRunes {
		return string(runes)
	}

	return string(
		runes[:maxRunes],
	)
}

func minInt(
	left int,
	right int,
) int {
	if left < right {
		return left
	}

	return right
}

func (consumer *Consumer) Close() {
	consumer.client.Close()
}
