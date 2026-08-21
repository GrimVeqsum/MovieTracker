package movies

import "time"

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Movie struct {
	ID              string `json:"id"`
	UserID          string `json:"user_id"`
	Title           string `json:"title"`
	NormalizedTitle string `json:"normalized_title"`
	ReleaseYear     *int   `json:"release_year"`

	ExternalID       *string `json:"external_id,omitempty"`
	MetadataProvider *string `json:"metadata_provider,omitempty"`
	OriginalTitle    *string `json:"original_title,omitempty"`
	Description      *string `json:"description,omitempty"`
	PosterURL        *string `json:"poster_url,omitempty"`
	RuntimeMinutes   *int    `json:"runtime_minutes,omitempty"`

	MetadataStatus string  `json:"metadata_status"`
	MetadataError  *string `json:"metadata_error,omitempty"`

	Genres []Genre `json:"genres,omitempty"`

	Status string  `json:"status"`
	Rating *int    `json:"rating"`
	Review *string `json:"review"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	WatchedAt *time.Time `json:"watched_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}
