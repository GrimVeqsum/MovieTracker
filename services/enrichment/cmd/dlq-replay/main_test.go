package main

import (
	"encoding/base64"
	"encoding/json"
	"github.com/twmb/franz-go/pkg/kgo"
	"testing"
)

func dlqRecord(eventID string) *kgo.Record {
	original := []byte(`{"event_id":"` + eventID + `","type":"MovieCreated","movie_id":"movie-1"}`)
	payload, _ := json.Marshal(deadLetterMessage{SourceTopic: "events", KeyBase64: base64.StdEncoding.EncodeToString([]byte("key")), ValueBase64: base64.StdEncoding.EncodeToString(original)})
	return &kgo.Record{Topic: "events.dlq", Value: payload}
}
func TestDecodeReplayPreservesBytesAndRejectsWrongTopic(t *testing.T) {
	record := dlqRecord("event-1")
	got, err := decodeReplayCandidate(record, "events")
	if err != nil {
		t.Fatal(err)
	}
	if got.EventID != "event-1" || string(got.Key) != "key" {
		t.Fatalf("candidate=%+v", got)
	}
	if _, err := decodeReplayCandidate(record, "other"); err == nil {
		t.Fatal("accepted incorrect source topic")
	}
	record.Value = []byte(`{"source_topic":"events","value_base64":"broken"}`)
	if _, err := decodeReplayCandidate(record, "events"); err == nil {
		t.Fatal("accepted corrupted envelope")
	}
}
func TestFilteredReplayDoesNotUseNormalReplayGroup(t *testing.T) {
	client, err := newReplayClient("127.0.0.1:1", "events.dlq", "normal-group", "event-1")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if group := client.OptValue("ConsumerGroup"); group != "" {
		t.Fatalf("filtered replay uses shared group %v", group)
	}
	normal, err := newReplayClient("127.0.0.1:1", "events.dlq", "normal-group", "")
	if err != nil {
		t.Fatal(err)
	}
	defer normal.Close()
	if normal.OptValue("ConsumerGroup") != "normal-group" {
		t.Fatal("ordinary replay lost durable group")
	}
}
