package movies

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCreateValidation(t *testing.T) {
	service := NewService(
		nil,
		nil,
	)

	tests := []struct {
		name    string
		title   string
		wantErr error
	}{
		{
			name:    "empty title",
			title:   "   ",
			wantErr: ErrMovieTitleRequired,
		},
		{
			name:    "title too long",
			title:   strings.Repeat("a", 301),
			wantErr: ErrMovieTitleTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				_, err := service.Create(
					context.Background(),
					CreateParams{
						UserID: "test-user",
						Title:  tt.title,
					},
				)

				if !errors.Is(err, tt.wantErr) {
					t.Fatalf(
						"expected error %v, got %v",
						tt.wantErr,
						err,
					)
				}
			},
		)
	}
}

func TestMakeWatchedRatingValidation(t *testing.T) {
	service := NewService(
		nil,
		nil,
	)

	tests := []struct {
		name   string
		rating int
	}{
		{
			name:   "rating below minimum",
			rating: 0,
		},
		{
			name:   "rating above maximum",
			rating: 11,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				_, err := service.MakeWatched(
					context.Background(),
					MakeWatchedParams{
						ID:     "movie-id",
						UserID: "user-id",
						Rating: tt.rating,
					},
				)

				if !errors.Is(
					err,
					ErrRatingIsOutOfRange,
				) {
					t.Fatalf(
						"expected ErrRatingIsOutOfRange, got %v",
						err,
					)
				}
			},
		)
	}
}

func TestMakeWatchedReviewValidation(t *testing.T) {
	service := NewService(
		nil,
		nil,
	)

	review :=
		strings.Repeat(
			"a",
			5001,
		)

	_, err := service.MakeWatched(
		context.Background(),
		MakeWatchedParams{
			ID:     "movie-id",
			UserID: "user-id",
			Rating: 5,
			Review: &review,
		},
	)

	if !errors.Is(
		err,
		ErrMovieReviewTooLong,
	) {
		t.Fatalf(
			"expected ErrMovieReviewTooLong, got %v",
			err,
		)
	}
}
