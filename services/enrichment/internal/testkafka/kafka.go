//go:build integration

// Package testkafka provisions unique topics in the isolated integration broker.
package testkafka

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func Broker(t *testing.T) string {
	t.Helper()
	broker := os.Getenv("TEST_KAFKA_BROKER")
	if broker == "" {
		t.Fatal("TEST_KAFKA_BROKER is required: use the isolated integration Compose project")
	}
	return broker
}
func Client(t *testing.T, opts ...kgo.Opt) *kgo.Client {
	t.Helper()
	base := []kgo.Opt{kgo.SeedBrokers(Broker(t)), kgo.RequiredAcks(kgo.AllISRAcks()), kgo.RecordDeliveryTimeout(10 * time.Second)}
	client, err := kgo.NewClient(append(base, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.CloseAllowingRebalance)
	return client
}
func Topics(t *testing.T, suffixes ...string) []string {
	t.Helper()
	client := Client(t)
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	prefix := strings.Trim(os.Getenv("TEST_KAFKA_TOPIC_PREFIX"), ".-_")
	if prefix == "" {
		prefix = "movietracker-integration"
	}
	prefix += "-" + hex.EncodeToString(random[:])
	names := make([]string, len(suffixes))
	req := kmsg.NewPtrCreateTopicsRequest()
	req.TimeoutMillis = 10000
	for i, suffix := range suffixes {
		names[i] = prefix + "-" + suffix
		topic := kmsg.NewCreateTopicsRequestTopic()
		topic.Topic = names[i]
		topic.NumPartitions = 1
		topic.ReplicationFactor = 1
		req.Topics = append(req.Topics, topic)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	response, err := client.Request(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	for _, topic := range response.(*kmsg.CreateTopicsResponse).Topics {
		if err := kerr.ErrorForCode(topic.ErrorCode); err != nil {
			t.Fatalf("create topic %s: %v", topic.Topic, err)
		}
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		del := kmsg.NewPtrDeleteTopicsRequest()
		del.TopicNames = names
		del.TimeoutMillis = 5000
		if _, err := client.Request(ctx, del); err != nil {
			t.Errorf("cleanup test topics: %v", err)
		}
	})
	return names
}
func Produce(t *testing.T, client *kgo.Client, records ...*kgo.Record) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := client.ProduceSync(ctx, records...).FirstErr(); err != nil {
		t.Fatal(err)
	}
}
func Read(t *testing.T, client *kgo.Client) *kgo.Record {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for ctx.Err() == nil {
		fetches := client.PollRecords(ctx, 1)
		if records := fetches.Records(); len(records) > 0 {
			return records[0]
		}
		for _, err := range fetches.Errors() {
			t.Log(err)
		}
	}
	t.Fatal("timed out waiting for Kafka record")
	return nil
}
