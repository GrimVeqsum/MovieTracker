package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Producer struct {
	client *kgo.Client

	topic string
}

func NewProducer(
	broker string,
	topic string,
) (*Producer, error) {
	client, err :=
		kgo.NewClient(
			kgo.SeedBrokers(
				broker,
			),

			kgo.ClientID(
				"library-service",
			),

			kgo.RequiredAcks(
				kgo.AllISRAcks(),
			),

			kgo.RecordDeliveryTimeout(
				10*time.Second,
			),
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"create kafka client: %w",
				err,
			)
	}

	return &Producer{
		client: client,

		topic: topic,
	}, nil
}

func (producer *Producer) Publish(
	ctx context.Context,
	key string,
	payload []byte,
) error {
	record :=
		&kgo.Record{
			Topic: producer.topic,

			Key: []byte(key),

			Value: payload,
		}

	publishCtx, cancel :=
		context.WithTimeout(
			ctx,
			10*time.Second,
		)

	defer cancel()

	err :=
		producer.client.
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
	producer.client.Close()
}
