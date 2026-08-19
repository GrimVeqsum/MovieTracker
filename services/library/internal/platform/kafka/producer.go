package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"movie-platform/library/internal/movies"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Producer struct {
	Client *kgo.Client
	Topic  string
}

func NewProducer(
	broker string,
	topic string,
) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(broker),
		kgo.ClientID("library-service"),

		kgo.RequiredAcks(kgo.AllISRAcks()),

		kgo.RecordDeliveryTimeout(10*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create kafka client: %w",
			err,
		)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		client.Close()

		return nil, fmt.Errorf(
			"connect to kafka: %w",
			err,
		)
	}

	admin := kadm.NewClient(client)

	topics, err := admin.ListTopics(
		ctx,
		topic,
	)
	if err != nil {
		client.Close()

		return nil, fmt.Errorf(
			"check kafka topic: %w",
			err,
		)
	}

	if !topics.Has(topic) {
		client.Close()

		return nil, fmt.Errorf(
			"kafka topic %q does not exist",
			topic,
		)
	}

	return &Producer{
		Client: client,
		Topic:  topic,
	}, nil
}

func (producer *Producer) Publish(
	ctx context.Context,
	event movies.Event,
) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf(
			"marshal kafka event: %w",
			err,
		)
	}

	record := &kgo.Record{
		Topic: producer.Topic,
		Key:   []byte(event.MovieID),
		Value: data,
	}

	publishCtx, cancel := context.WithTimeout(
		ctx,
		10*time.Second,
	)
	defer cancel()

	err = producer.Client.
		ProduceSync(
			publishCtx,
			record,
		).
		FirstErr()

	if err != nil {
		return fmt.Errorf(
			"produce kafka event: %w",
			err,
		)
	}

	return nil
}

func (producer *Producer) Close() {
	producer.Client.Close()
}
