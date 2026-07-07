package movies

import "time"

type Movie struct {
	ID              string     `json:"id" example:"60fe0ecb-7b2d-4d9d-a324-6b15a45a4df5"`
	UserID          string     `json:"user_id" example:"11111111-1111-1111-1111-111111111111"`
	Title           string     `json:"title" example:"Interstellar"`
	NormalizedTitle string     `json:"normalized_title" example:"interstellar"`
	ReleaseYear     *int       `json:"release_year" example:"2014"`
	Status          string     `json:"status" example:"watched" enums:"unwatched,watched"`
	Rating          *int       `json:"rating" example:"9" minimum:"1" maximum:"10"`
	Review          *string    `json:"review" example:"Good movie"`
	CreatedAt       time.Time  `json:"created_at" example:"2026-07-05T20:58:31+03:00"`
	UpdatedAt       time.Time  `json:"updated_at" example:"2026-07-05T20:59:16+03:00"`
	WatchedAt       *time.Time `json:"watched_at" example:"2026-07-05T20:58:31+03:00"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}
