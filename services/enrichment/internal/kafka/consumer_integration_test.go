//go:build integration

package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"movie-platform/enrichment/internal/library"
	"movie-platform/enrichment/internal/poiskkino"
	"movie-platform/enrichment/internal/testkafka"
)

type commitFaultClient struct {
	*kgo.Client
	cancel  context.CancelFunc
	fail    bool
	commits chan struct{}
}

func (c *commitFaultClient) CommitRecords(ctx context.Context, records ...*kgo.Record) error {
	if c.fail {
		c.cancel()
		return context.Canceled
	}
	err := c.Client.CommitRecords(ctx, records...)
	if err == nil {
		select {
		case c.commits <- struct{}{}:
		default:
		}
	}
	return err
}
func waitConsumer(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("consumer did not stop after cancellation")
	}
}

func TestKafkaCrashBeforeCommitRedeliversWithoutDuplicateResult(t *testing.T) {
	topics := testkafka.Topics(t, "events", "dlq")
	producer := testkafka.Client(t)
	var mu sync.Mutex
	seen := map[string]bool{}
	effects := 0
	var calls atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req library.UpdateMetadataRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if r.Header.Get("X-Service-Token") != "integration-only" {
			t.Error("missing service token")
		}
		calls.Add(1)
		mu.Lock()
		if !seen[req.EventID] {
			seen[req.EventID] = true
			effects++
		}
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()
	record := movieRecord()
	record.Topic = topics[0]
	testkafka.Produce(t, producer, record)
	group := topics[0] + "-consumer"
	first, err := NewConsumer(testkafka.Broker(t), topics[0], topics[1], group, providerStub{result: &poiskkino.MovieDetails{ID: 7, Year: 2020}}, library.NewClient(backend.URL, "integration-only"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	first.client = &commitFaultClient{Client: first.client.(*kgo.Client), cancel: cancel, fail: true}
	done := make(chan error, 1)
	go func() { done <- first.Run(ctx) }()
	waitConsumer(t, done)
	cancel()
	first.Close()
	if calls.Load() != 1 {
		t.Fatalf("first attempt calls=%d", calls.Load())
	}
	second, err := NewConsumer(testkafka.Broker(t), topics[0], topics[1], group, providerStub{result: &poiskkino.MovieDetails{ID: 7, Year: 2020}}, library.NewClient(backend.URL, "integration-only"))
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	secondCtx, secondCancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer secondCancel()
	commits := make(chan struct{}, 1)
	second.client = &commitFaultClient{Client: second.client.(*kgo.Client), commits: commits}
	go func() { done <- second.Run(secondCtx) }()
	select {
	case <-commits:
	case <-secondCtx.Done():
		t.Fatal("redelivery was not committed")
	}
	secondCancel()
	waitConsumer(t, done)
	mu.Lock()
	defer mu.Unlock()
	if calls.Load() != 2 || effects != 1 {
		t.Fatalf("deliveries=%d persisted effects=%d", calls.Load(), effects)
	}
}

type dlqFaultClient struct {
	*kgo.Client
	failures atomic.Int32
}

func (c *dlqFaultClient) ProduceSync(ctx context.Context, records ...*kgo.Record) kgo.ProduceResults {
	if c.failures.Add(1) == 1 {
		return kgo.ProduceResults{{Record: records[0], Err: errors.New("injected broker publish failure")}}
	}
	return c.Client.ProduceSync(ctx, records...)
}
func TestKafkaDLQFailureRetriesAndPreservesOriginal(t *testing.T) {
	topics := testkafka.Topics(t, "events", "dlq")
	producer := testkafka.Client(t)
	original := &kgo.Record{Topic: topics[0], Key: []byte("original-key"), Value: []byte("invalid-json")}
	testkafka.Produce(t, producer, original)
	consumer, err := NewConsumer(testkafka.Broker(t), topics[0], topics[1], topics[0]+"-group", nil, &libraryStub{})
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()
	fault := &dlqFaultClient{Client: consumer.client.(*kgo.Client)}
	consumer.client = fault
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx) }()
	reader := testkafka.Client(t, kgo.ConsumeTopics(topics[1]), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	copied := testkafka.Read(t, reader)
	cancel()
	waitConsumer(t, done)
	var envelope deadLetterMessage
	if err := json.Unmarshal(copied.Value, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.SourceTopic != topics[0] || envelope.SourceOffset != original.Offset || string(copied.Key) != string(original.Key) {
		t.Fatalf("DLQ lost source information: %+v", envelope)
	}
	if fault.failures.Load() < 2 {
		t.Fatal("DLQ publish failure was not retried")
	}
}
