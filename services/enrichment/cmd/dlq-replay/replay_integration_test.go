//go:build integration

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"movie-platform/enrichment/internal/testkafka"
)

func produceDLQ(t *testing.T, topic, source, eventID string) {
	t.Helper()
	record := dlqRecord(eventID)
	var payload deadLetterMessage
	if err := json.Unmarshal(record.Value, &payload); err != nil {
		t.Fatal(err)
	}
	payload.SourceTopic = source
	value, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	testkafka.Produce(t, testkafka.Client(t), &kgo.Record{Topic: topic, Key: []byte("key"), Value: value})
}

func TestKafkaFilteredReplayPreservesNormalReplayQueue(t *testing.T) {
	topics := testkafka.Topics(t, "events", "dlq")
	produceDLQ(t, topics[1], topics[0], "event-1")
	produceDLQ(t, topics[1], topics[0], "event-2")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	filtered, err := newReplayClient(testkafka.Broker(t), topics[1], topics[0]+"-replay", "event-2")
	if err != nil {
		t.Fatal(err)
	}
	if err := replay(ctx, filtered, topics[0], 1, 20*time.Second, "event-2"); err != nil {
		t.Fatal(err)
	}
	filtered.CloseAllowingRebalance()
	normal, err := newReplayClient(testkafka.Broker(t), topics[1], topics[0]+"-replay", "")
	if err != nil {
		t.Fatal(err)
	}
	defer normal.CloseAllowingRebalance()
	if err := replay(ctx, normal, topics[0], 2, 20*time.Second, ""); err != nil {
		t.Fatal(err)
	}
	reader := testkafka.Client(t, kgo.ConsumeTopics(topics[0]), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	counts := map[string]int{}
	for i := 0; i < 3; i++ {
		record := testkafka.Read(t, reader)
		var event movieEvent
		if err := json.Unmarshal(record.Value, &event); err != nil {
			t.Fatal(err)
		}
		counts[event.EventID]++
		if string(record.Key) != "key" {
			t.Fatal("replay changed original key")
		}
	}
	if counts["event-1"] != 1 || counts["event-2"] != 2 {
		t.Fatalf("events lost from shared replay queue: %v", counts)
	}
}

type replayFaultClient struct {
	*kgo.Client
	publishFailure bool
	commitFailure  bool
}

func (c *replayFaultClient) ProduceSync(ctx context.Context, records ...*kgo.Record) kgo.ProduceResults {
	if c.publishFailure {
		return kgo.ProduceResults{{Record: records[0], Err: errors.New("injected publish failure")}}
	}
	return c.Client.ProduceSync(ctx, records...)
}
func (c *replayFaultClient) CommitRecords(ctx context.Context, records ...*kgo.Record) error {
	if c.commitFailure {
		return errors.New("injected crash before commit")
	}
	return c.Client.CommitRecords(ctx, records...)
}
func TestKafkaReplayFailuresDoNotAcknowledgeSource(t *testing.T) {
	for _, commitFailure := range []bool{false, true} {
		name := "publish"
		if commitFailure {
			name = "before offset commit"
		}
		t.Run(name, func(t *testing.T) {
			topics := testkafka.Topics(t, "events", "dlq")
			produceDLQ(t, topics[1], topics[0], "event-1")
			group := topics[0] + "-replay"
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			first, err := newReplayClient(testkafka.Broker(t), topics[1], group, "")
			if err != nil {
				t.Fatal(err)
			}
			fault := &replayFaultClient{Client: first, publishFailure: !commitFailure, commitFailure: commitFailure}
			if err := replay(ctx, fault, topics[0], 1, 20*time.Second, ""); err == nil {
				t.Fatal("injected crash was ignored")
			}
			first.CloseAllowingRebalance()
			recovered, err := newReplayClient(testkafka.Broker(t), topics[1], group, "")
			if err != nil {
				t.Fatal(err)
			}
			defer recovered.CloseAllowingRebalance()
			if err := replay(ctx, recovered, topics[0], 1, 20*time.Second, ""); err != nil {
				t.Fatal(err)
			}
			reader := testkafka.Client(t, kgo.ConsumeTopics(topics[0]), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
			copies := 1
			if commitFailure {
				copies = 2
			}
			original, _ := base64.StdEncoding.DecodeString(func() string {
				var message deadLetterMessage
				json.Unmarshal(dlqRecord("event-1").Value, &message)
				return message.ValueBase64
			}())
			for i := 0; i < copies; i++ {
				if record := testkafka.Read(t, reader); string(record.Value) != string(original) {
					t.Fatal("replay changed original payload")
				}
			}
		})
	}
}
