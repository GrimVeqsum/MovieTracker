package movies

import "time"

type Event struct {
	EventID string `json:"event_id"`

	Version int `json:"version"`

	Type string `json:"type"`

	MovieID string `json:"movie_id"`

	UserID string `json:"user_id"`

	Title string `json:"title,omitempty"`

	ReleaseYear *int `json:"release_year,omitempty"`

	Rating *int `json:"rating,omitempty"`

	Review *string `json:"review,omitempty"`

	OccurredAt time.Time `json:"occurred_at"`
}
