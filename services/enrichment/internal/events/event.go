package events

import "time"

type MovieEvent struct {
	EventID string `json:"event_id"`
	Version int    `json:"version"`
	Type    string `json:"type"`
	MovieID string `json:"movie_id"`
	UserID  string `json:"user_id"`

	Title       string `json:"title"`
	ReleaseYear *int   `json:"release_year,omitempty"`

	OccurredAt time.Time `json:"occurred_at"`
}
