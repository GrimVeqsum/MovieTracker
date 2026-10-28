//go:build integration

package outbox

import (
	"bytes"
	"context"
	"net"
	"os"
	"testing"
	"time"

	librarykafka "movie-platform/library/internal/platform/kafka"
	"movie-platform/library/internal/testutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

func kafkaTopic(t *testing.T) (string, string) {
	t.Helper()
	broker := os.Getenv("TEST_KAFKA_BROKER")
	if broker == "" {
		t.Fatal("TEST_KAFKA_BROKER is required")
	}
	topic := os.Getenv("TEST_KAFKA_TOPIC_PREFIX") + "library-" + uuid.NewString()
	client, err := kgo.NewClient(kgo.SeedBrokers(broker))
	if err != nil {
		t.Fatal(err)
	}
	admin := kadm.NewClient(client)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := admin.CreateTopic(ctx, 1, 1, nil, topic); err != nil {
		client.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.DeleteTopic(ctx, topic); err != nil {
			t.Errorf("delete unique test topic: %v", err)
		}
		client.Close()
	})
	return broker, topic
}
func insertOutboxEvent(t *testing.T, db *pgxpool.Pool) (*Repository, string, []byte) {
	t.Helper()
	repo := NewRepository(db)
	id := uuid.NewString()
	payload := []byte(`{"event_id":"` + id + `","type":"MovieCreated"}`)
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.InsertTx(context.Background(), tx, InsertParams{ID: id, AggregateType: "movie", AggregateID: uuid.NewString(), EventType: "MovieCreated", Payload: map[string]string{"event_id": id, "type": "MovieCreated"}, OccurredAt: time.Now()}); err != nil {
		_ = tx.Rollback(context.Background())
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	return repo, id, payload
}
func eventPublished(t *testing.T, db *pgxpool.Pool, id string) bool {
	t.Helper()
	var published bool
	if err := db.QueryRow(context.Background(), "SELECT published_at IS NOT NULL FROM outbox_events WHERE id=$1", id).Scan(&published); err != nil {
		t.Fatal(err)
	}
	return published
}
func assertKafkaEvents(t *testing.T, broker, topic, eventID string, n int) {
	t.Helper()
	client, err := kgo.NewClient(kgo.SeedBrokers(broker), kgo.ConsumePartitions(map[string]map[int32]kgo.Offset{topic: {0: kgo.NewOffset().AtStart()}}))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	received := 0
	for received < n {
		fetches := client.PollFetches(ctx)
		if err := fetches.Err(); err != nil {
			t.Fatal(err)
		}
		for _, record := range fetches.Records() {
			if !bytes.Contains(record.Value, []byte(eventID)) {
				t.Fatalf("unexpected record: %s", record.Value)
			}
			received++
		}
	}
	if received != n {
		t.Fatalf("got %d records, want %d", received, n)
	}
}

func TestKafkaUnavailableRetainsOutboxThenRecovers(t *testing.T) {
	broker, topic := kafkaTopic(t)
	db := testutil.Database(t)
	repo, id, _ := insertOutboxEvent(t, db)
	// An unreachable listener simulates loss of connectivity to the real broker;
	// recovery below uses the actual isolated Kafka, never a broker mock.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	unreachable := listener.Addr().String()
	_ = listener.Close()
	failedProducer, err := librarykafka.NewProducer(unreachable, topic)
	if err != nil {
		t.Fatal(err)
	}
	failedWorker := NewWorker(repo, failedProducer)
	failedWorker.batchSize = 1
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	if _, err := failedWorker.processBatch(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	failedProducer.Close()
	if eventPublished(t, db, id) {
		t.Fatal("unavailable Kafka acknowledged an event")
	}
	var attempts int
	var lastError *string
	if err := db.QueryRow(context.Background(), "SELECT attempts,last_error FROM outbox_events WHERE id=$1", id).Scan(&attempts, &lastError); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || lastError == nil {
		t.Fatalf("failure not persisted: attempts=%d error=%v", attempts, lastError)
	}
	if _, err := db.Exec(context.Background(), "UPDATE outbox_events SET next_attempt_at=NOW() WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	producer, err := librarykafka.NewProducer(broker, topic)
	if err != nil {
		t.Fatal(err)
	}
	defer producer.Close()
	worker := NewWorker(repo, producer)
	worker.batchSize = 1
	if _, err := worker.processBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !eventPublished(t, db, id) {
		t.Fatal("recovered event remains unpublished")
	}
	assertKafkaEvents(t, broker, topic, id, 1)
}

func TestCrashAfterKafkaAckRedeliversUnmarkedOutbox(t *testing.T) {
	broker, topic := kafkaTopic(t)
	db := testutil.Database(t)
	repo, id, _ := insertOutboxEvent(t, db)
	producer, err := librarykafka.NewProducer(broker, topic)
	if err != nil {
		t.Fatal(err)
	}
	defer producer.Close()
	messages, err := repo.ClaimBatch(context.Background(), uuid.NewString(), 1)
	if err != nil || len(messages) != 1 {
		t.Fatalf("claim: %v %+v", err, messages)
	}
	if err := producer.Publish(context.Background(), messages[0].AggregateID, messages[0].Payload); err != nil {
		t.Fatal(err)
	}
	// Crash point: broker acknowledged, published_at was never persisted. Expire
	// only our test lease rather than waiting a minute or killing shared services.
	if _, err := db.Exec(context.Background(), "UPDATE outbox_events SET locked_at=NOW()-INTERVAL '2 minutes' WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(repo, producer)
	worker.batchSize = 1
	if _, err := worker.processBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !eventPublished(t, db, id) {
		t.Fatal("restart did not acknowledge redelivery")
	}
	assertKafkaEvents(t, broker, topic, id, 2)
}

func TestWorkerShutdownPersistsKafkaAcknowledgement(t *testing.T) {
	broker, topic := kafkaTopic(t)
	db := testutil.Database(t)
	repo, id, _ := insertOutboxEvent(t, db)
	producer, err := librarykafka.NewProducer(broker, topic)
	if err != nil {
		t.Fatal(err)
	}
	defer producer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker := NewWorker(repo, publishFunc(func(ctx context.Context, key string, payload []byte) error {
		err := producer.Publish(ctx, key, payload)
		cancel() // shutdown immediately after the real broker acknowledges
		return err
	}))
	done := make(chan struct{})
	go func() { worker.Run(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("worker did not shut down")
	}
	if !eventPublished(t, db, id) {
		t.Fatal("shutdown failed to persist acknowledged send")
	}
	assertKafkaEvents(t, broker, topic, id, 1)
}
