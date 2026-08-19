package movies

import (
	"context"
	"time"
)

type Event struct {
	Type       string    `json:"type"`
	MovieID    string    `json:"movie_id"`
	UserID     string    `json:"user_id"`
	Rating     *int      `json:"rating,omitempty"`
	Review     *string   `json:"review,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

type EventPublisher interface {
	Publish(ctx context.Context, event Event) error
}
