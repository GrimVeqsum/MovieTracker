package kafka

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"movie-platform/enrichment/internal/library"
	"movie-platform/enrichment/internal/poiskkino"
)

type providerStub struct {
	result *poiskkino.MovieDetails
	err    error
}

func (p providerStub) FindMovie(context.Context, string, *int) (*poiskkino.MovieDetails, error) {
	return p.result, p.err
}

type libraryStub struct {
	updates  int
	failures int
	err      error
}

func (s *libraryStub) UpdateMetadata(context.Context, string, library.UpdateMetadataRequest) error {
	s.updates++
	return s.err
}
func (s *libraryStub) MarkMetadataFailed(context.Context, string, library.MarkMetadataFailedRequest) error {
	s.failures++
	return s.err
}
func movieRecord() *kgo.Record {
	return &kgo.Record{Topic: "events", Partition: 0, Offset: 7, Key: []byte("movie"), Value: []byte(`{"event_id":"event-1","version":1,"type":"MovieCreated","movie_id":"movie-1","user_id":"user-1","title":"Film"}`)}
}

type brokerStub struct {
	fetches kgo.Fetches
	commits int
	dlq     []*kgo.Record
	cancel  context.CancelFunc
}

func (b *brokerStub) PollRecords(ctx context.Context, _ int) kgo.Fetches {
	if len(b.fetches) > 0 {
		f := b.fetches
		b.fetches = nil
		return f
	}
	<-ctx.Done()
	return nil
}
func (b *brokerStub) CommitRecords(_ context.Context, _ ...*kgo.Record) error {
	b.commits++
	b.cancel()
	return nil
}
func (b *brokerStub) ProduceSync(_ context.Context, r ...*kgo.Record) kgo.ProduceResults {
	b.dlq = append(b.dlq, r...)
	return kgo.ProduceResults{{Record: r[0]}}
}
func (*brokerStub) AllowRebalance()         {}
func (*brokerStub) CloseAllowingRebalance() {}

func TestRunProcessesRecordsEvenWhenAnotherPartitionHasFetchError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	b := &brokerStub{cancel: cancel, fetches: kgo.Fetches{{Topics: []kgo.FetchTopic{{Topic: "events", Partitions: []kgo.FetchPartition{{Partition: 1, Err: errors.New("partition unavailable")}, {Partition: 0, Records: []*kgo.Record{movieRecord()}}}}}}}}
	lib := &libraryStub{}
	c := &Consumer{client: b, movieProvider: providerStub{result: &poiskkino.MovieDetails{ID: 10, Year: 2020}}, libraryClient: lib}
	if err := c.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if b.commits != 1 || lib.updates != 1 {
		t.Fatalf("commits=%d updates=%d", b.commits, lib.updates)
	}
}

func TestInvalidRecordIsAcknowledgedOnlyAfterLosslessDLQCopy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	record := movieRecord()
	record.Value = []byte("invalid JSON")
	b := &brokerStub{cancel: cancel, fetches: kgo.Fetches{{Topics: []kgo.FetchTopic{{Topic: "events", Partitions: []kgo.FetchPartition{{Records: []*kgo.Record{record}}}}}}}}
	c := &Consumer{client: b, dlqTopic: "events.dlq", libraryClient: &libraryStub{}}
	if err := c.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if b.commits != 1 || len(b.dlq) != 1 {
		t.Fatalf("commits=%d dlq=%d", b.commits, len(b.dlq))
	}
	var payload deadLetterMessage
	if err := json.Unmarshal(b.dlq[0].Value, &payload); err != nil {
		t.Fatal(err)
	}
	original, err := base64.StdEncoding.DecodeString(payload.ValueBase64)
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != string(record.Value) || payload.SourceOffset != record.Offset {
		t.Fatalf("DLQ did not preserve original: %+v", payload)
	}
}

func TestLibraryErrorClassificationAndNotFound(t *testing.T) {
	for _, tc := range []struct {
		status    int
		permanent bool
		success   bool
	}{{404, true, false}, {400, true, false}, {401, true, false}, {429, false, false}, {503, false, false}} {
		err := classifyLibraryError("update", &library.HTTPError{StatusCode: tc.status})
		var p *permanentProcessingError
		if (err == nil) != tc.success || errors.As(err, &p) != tc.permanent {
			t.Errorf("status=%d err=%v", tc.status, err)
		}
	}
	lib := &libraryStub{}
	c := &Consumer{libraryClient: lib, movieProvider: providerStub{err: poiskkino.ErrMovieNotFound}}
	if err := c.process(context.Background(), movieRecord()); err != nil {
		t.Fatal(err)
	}
	if lib.failures != 1 || lib.updates != 0 {
		t.Fatalf("updates=%d failures=%d", lib.updates, lib.failures)
	}
	lib.err = &library.HTTPError{StatusCode: http.StatusNotFound}
	var permanent *permanentProcessingError
	if err := c.process(context.Background(), movieRecord()); !errors.As(err, &permanent) {
		t.Fatalf("missing Library movie must be preserved in DLQ: %v", err)
	}
}

func TestRetryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForRetry(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}
