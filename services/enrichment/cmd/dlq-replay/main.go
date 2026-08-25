package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	defaultReplayGroup = "movie-enrichment-dlq-replay"

	defaultLimit = 10

	defaultIdleTimeout = 10 * time.Second

	publishTimeout = 10 * time.Second

	commitTimeout = 10 * time.Second
)

var errIdleTimeout = errors.New(
	"DLQ replay idle timeout",
)

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

type movieEvent struct {
	EventID string `json:"event_id"`

	Type string `json:"type"`

	MovieID string `json:"movie_id"`
}

type replayCandidate struct {
	EventID string

	MovieID string

	Key []byte

	Value []byte
}

func main() {
	if err := run(); err != nil {
		log.Printf(
			"DLQ replay failed: %v",
			err,
		)

		os.Exit(1)
	}
}

func run() error {
	var limit int

	var idleTimeout time.Duration

	var groupOverride string

	var eventIDFilter string

	flag.IntVar(
		&limit,
		"limit",
		defaultLimit,
		"maximum number of unique DLQ events to replay",
	)

	flag.DurationVar(
		&idleTimeout,
		"idle-timeout",
		defaultIdleTimeout,
		"how long to wait for another DLQ message before exiting",
	)

	flag.StringVar(
		&groupOverride,
		"group",
		"",
		"Kafka consumer group used for DLQ replay",
	)

	flag.StringVar(
		&eventIDFilter,
		"event-id",
		"",
		"replay only this event_id",
	)

	flag.Parse()

	if limit <= 0 {
		return errors.New(
			"limit must be greater than zero",
		)
	}

	if idleTimeout <= 0 {
		return errors.New(
			"idle-timeout must be greater than zero",
		)
	}

	eventIDFilter =
		strings.TrimSpace(
			eventIDFilter,
		)

	broker, err :=
		requiredEnv(
			"ENRICHMENT_KAFKA_BROKER",
		)
	if err != nil {
		return err
	}

	targetTopic, err :=
		requiredEnv(
			"ENRICHMENT_KAFKA_TOPIC",
		)
	if err != nil {
		return err
	}

	dlqTopic :=
		strings.TrimSpace(
			os.Getenv(
				"ENRICHMENT_KAFKA_DLQ_TOPIC",
			),
		)

	if dlqTopic == "" {
		dlqTopic =
			targetTopic + ".dlq"
	}

	group :=
		strings.TrimSpace(
			groupOverride,
		)

	if group == "" {
		group =
			strings.TrimSpace(
				os.Getenv(
					"ENRICHMENT_DLQ_REPLAY_GROUP",
				),
			)
	}

	if group == "" {
		group =
			defaultReplayGroup
	}

	client, err :=
		kgo.NewClient(
			kgo.SeedBrokers(
				broker,
			),

			kgo.ClientID(
				"enrichment-dlq-replay",
			),

			kgo.ConsumerGroup(
				group,
			),

			kgo.ConsumeTopics(
				dlqTopic,
			),

			kgo.ConsumeResetOffset(
				kgo.NewOffset().
					AtStart(),
			),

			kgo.DisableAutoCommit(),
		)
	if err != nil {
		return fmt.Errorf(
			"create Kafka client: %w",
			err,
		)
	}

	defer client.Close()

	ctx, stop :=
		signal.NotifyContext(
			context.Background(),
			os.Interrupt,
			syscall.SIGTERM,
		)

	defer stop()

	log.Printf(
		"DLQ replay started: broker=%s dlq_topic=%s target_topic=%s group=%s limit=%d event_id_filter=%q",
		broker,
		dlqTopic,
		targetTopic,
		group,
		limit,
		eventIDFilter,
	)

	replayed :=
		0

	seenEventIDs :=
		make(
			map[string]struct{},
		)

	for replayed < limit {
		record, err :=
			pollOne(
				ctx,
				client,
				idleTimeout,
			)

		if err != nil {
			if errors.Is(
				err,
				errIdleTimeout,
			) {
				log.Printf(
					"DLQ replay finished: no new messages for %s, replayed=%d",
					idleTimeout,
					replayed,
				)

				return nil
			}

			return err
		}

		if record == nil {
			continue
		}

		candidate, err :=
			decodeReplayCandidate(
				record,
				targetTopic,
			)

		if err != nil {
			return fmt.Errorf(
				"decode DLQ record topic=%s partition=%d offset=%d: %w",
				record.Topic,
				record.Partition,
				record.Offset,
				err,
			)
		}

		// Если пользователь указал конкретный event_id,
		// остальные DLQ-сообщения просто пропускаем.
		//
		// Но offset обязательно commit-им,
		// иначе эта replay-group будет снова читать
		// те же записи.
		if eventIDFilter != "" &&
			candidate.EventID != eventIDFilter {

			if err :=
				commitRecord(
					ctx,
					client,
					record,
				); err != nil {

				return fmt.Errorf(
					"commit skipped DLQ record: %w",
					err,
				)
			}

			continue
		}

		// Один event_id может оказаться в DLQ несколько раз,
		// например если его уже replay-или,
		// но внешний API снова упал.
		//
		// В рамках одного запуска повторно его
		// в movie.events не отправляем.
		if _, exists :=
			seenEventIDs[candidate.EventID]; exists {

			log.Printf(
				"duplicate DLQ event skipped: event_id=%s source_partition=%d source_offset=%d",
				candidate.EventID,
				record.Partition,
				record.Offset,
			)

			if err :=
				commitRecord(
					ctx,
					client,
					record,
				); err != nil {

				return fmt.Errorf(
					"commit duplicate DLQ record: %w",
					err,
				)
			}

			continue
		}

		if err :=
			publishCandidate(
				ctx,
				client,
				candidate,
				targetTopic,
				record.Topic,
			); err != nil {

			return fmt.Errorf(
				"replay DLQ record topic=%s partition=%d offset=%d: %w",
				record.Topic,
				record.Partition,
				record.Offset,
				err,
			)
		}

		if err :=
			commitRecord(
				ctx,
				client,
				record,
			); err != nil {

			return fmt.Errorf(
				"commit replayed DLQ record event_id=%s: %w",
				candidate.EventID,
				err,
			)
		}

		seenEventIDs[candidate.EventID] = struct{}{}

		replayed++

		log.Printf(
			"DLQ event replayed: event_id=%s movie_id=%s source_partition=%d source_offset=%d replayed=%d/%d",
			candidate.EventID,
			candidate.MovieID,
			record.Partition,
			record.Offset,
			replayed,
			limit,
		)

		if eventIDFilter != "" {
			log.Printf(
				"DLQ replay finished: requested event_id was replayed",
			)

			return nil
		}
	}

	log.Printf(
		"DLQ replay finished: limit reached, replayed=%d",
		replayed,
	)

	return nil
}

func pollOne(
	ctx context.Context,
	client *kgo.Client,
	idleTimeout time.Duration,
) (*kgo.Record, error) {
	pollCtx, cancel :=
		context.WithTimeout(
			ctx,
			idleTimeout,
		)

	defer cancel()

	fetches :=
		client.PollRecords(
			pollCtx,
			1,
		)

	records :=
		fetches.Records()

	if len(records) > 0 {
		return records[0], nil
	}

	if ctx.Err() != nil {
		return nil,
			ctx.Err()
	}

	if errors.Is(
		pollCtx.Err(),
		context.DeadlineExceeded,
	) {
		return nil,
			errIdleTimeout
	}

	fetchErrors :=
		fetches.Errors()

	if len(fetchErrors) > 0 {
		return nil,
			fmt.Errorf(
				"poll DLQ: %v",
				fetchErrors[0],
			)
	}

	return nil, nil
}

func decodeReplayCandidate(
	record *kgo.Record,
	targetTopic string,
) (replayCandidate, error) {
	var dlqMessage deadLetterMessage

	if err :=
		json.Unmarshal(
			record.Value,
			&dlqMessage,
		); err != nil {

		return replayCandidate{},
			fmt.Errorf(
				"decode DLQ message: %w",
				err,
			)
	}

	sourceTopic :=
		strings.TrimSpace(
			dlqMessage.SourceTopic,
		)

	if sourceTopic == "" {
		return replayCandidate{},
			errors.New(
				"DLQ message has empty source_topic",
			)
	}

	if sourceTopic != targetTopic {
		return replayCandidate{},
			fmt.Errorf(
				"DLQ source topic %q does not match configured target topic %q",
				sourceTopic,
				targetTopic,
			)
	}

	key, err :=
		base64.StdEncoding.
			DecodeString(
				dlqMessage.KeyBase64,
			)

	if err != nil {
		return replayCandidate{},
			fmt.Errorf(
				"decode original Kafka key: %w",
				err,
			)
	}

	value, err :=
		base64.StdEncoding.
			DecodeString(
				dlqMessage.ValueBase64,
			)

	if err != nil {
		return replayCandidate{},
			fmt.Errorf(
				"decode original Kafka value: %w",
				err,
			)
	}

	if len(value) == 0 {
		return replayCandidate{},
			errors.New(
				"decoded original Kafka value is empty",
			)
	}

	var event movieEvent

	if err :=
		json.Unmarshal(
			value,
			&event,
		); err != nil {

		return replayCandidate{},
			fmt.Errorf(
				"decode original movie event: %w",
				err,
			)
	}

	event.EventID =
		strings.TrimSpace(
			event.EventID,
		)

	event.MovieID =
		strings.TrimSpace(
			event.MovieID,
		)

	if event.EventID == "" {
		return replayCandidate{},
			errors.New(
				"original event has empty event_id",
			)
	}

	if event.MovieID == "" {
		return replayCandidate{},
			errors.New(
				"original event has empty movie_id",
			)
	}

	if event.Type != "MovieCreated" {
		return replayCandidate{},
			fmt.Errorf(
				"unsupported original event type %q",
				event.Type,
			)
	}

	return replayCandidate{
		EventID: event.EventID,

		MovieID: event.MovieID,

		Key: key,

		Value: value,
	}, nil
}

func publishCandidate(
	ctx context.Context,
	client *kgo.Client,
	candidate replayCandidate,
	targetTopic string,
	dlqTopic string,
) error {
	publishCtx, cancel :=
		context.WithTimeout(
			ctx,
			publishTimeout,
		)

	defer cancel()

	record :=
		&kgo.Record{
			Topic: targetTopic,

			Key: candidate.Key,

			Value: candidate.Value,

			Headers: []kgo.RecordHeader{
				{
					Key: "x-movietracker-replayed",

					Value: []byte("true"),
				},
				{
					Key: "x-movietracker-dlq-topic",

					Value: []byte(
						dlqTopic,
					),
				},
			},
		}

	err :=
		client.
			ProduceSync(
				publishCtx,
				record,
			).
			FirstErr()

	if err != nil {
		return fmt.Errorf(
			"publish original event: %w",
			err,
		)
	}

	return nil
}

func commitRecord(
	ctx context.Context,
	client *kgo.Client,
	record *kgo.Record,
) error {
	commitCtx, cancel :=
		context.WithTimeout(
			ctx,
			commitTimeout,
		)

	defer cancel()

	if err :=
		client.CommitRecords(
			commitCtx,
			record,
		); err != nil {

		return fmt.Errorf(
			"commit DLQ offset: %w",
			err,
		)
	}

	return nil
}

func requiredEnv(
	name string,
) (string, error) {
	value :=
		strings.TrimSpace(
			os.Getenv(name),
		)

	if value == "" {
		return "",
			fmt.Errorf(
				"%s is empty",
				name,
			)
	}

	return value, nil
}
