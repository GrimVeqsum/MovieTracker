package movies

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	maxMovieTitleRunes = 300

	maxMovieReviewRunes = 5000
)

type Service struct {
	repo *Repository

	writer *TransactionalRepository
}

func NewService(
	repo *Repository,
	writer *TransactionalRepository,
) *Service {
	return &Service{
		repo: repo,

		writer: writer,
	}
}

// List movies

type ListParams struct {
	UserID string
}

func (service *Service) List(
	ctx context.Context,
	params ListParams,
) ([]Movie, error) {
	return service.repo.List(
		ctx,
		ListMovieParams{
			UserID: params.UserID,
		},
	)
}

// Create movie

type CreateParams struct {
	UserID string

	Title string

	ReleaseYear *int
}

func (service *Service) Create(
	ctx context.Context,
	params CreateParams,
) (*Movie, error) {
	title :=
		strings.TrimSpace(
			params.Title,
		)

	if title == "" {
		return nil,
			ErrMovieTitleRequired
	}

	if utf8.RuneCountInString(
		title,
	) > maxMovieTitleRunes {

		return nil,
			ErrMovieTitleTooLong
	}

	normalizedTitle :=
		strings.ToLower(
			title,
		)

	movieID :=
		uuid.NewString()

	event :=
		Event{
			EventID: uuid.NewString(),

			Version: 1,

			Type: "MovieCreated",

			MovieID: movieID,

			UserID: params.UserID,

			Title: title,

			ReleaseYear: params.ReleaseYear,

			OccurredAt: time.Now().
				UTC(),
		}

	return service.writer.Create(
		ctx,
		CreateMovieWithEventParams{
			ID: movieID,

			UserID: params.UserID,

			Title: title,

			NormalizedTitle: normalizedTitle,

			ReleaseYear: params.ReleaseYear,
		},
		event,
	)
}

// Delete movie

type DeleteParams struct {
	UserID string

	ID string
}

func (service *Service) Delete(
	ctx context.Context,
	params DeleteParams,
) error {
	event :=
		Event{
			EventID: uuid.NewString(),

			Version: 1,

			Type: "MovieDeleted",

			MovieID: params.ID,

			UserID: params.UserID,

			OccurredAt: time.Now().
				UTC(),
		}

	return service.writer.Delete(
		ctx,
		DeleteMovieParams{
			UserID: params.UserID,

			ID: params.ID,
		},
		event,
	)
}

// Get movie by id

type GetParams struct {
	UserID string

	ID string
}

func (service *Service) GetOne(
	ctx context.Context,
	params GetParams,
) (*Movie, error) {
	return service.repo.GetOne(
		ctx,
		GetOneParams{
			UserID: params.UserID,

			ID: params.ID,
		},
	)
}

// Make movie watched

type MakeWatchedParams struct {
	ID string

	UserID string

	Rating int

	Review *string
}

func (service *Service) MakeWatched(
	ctx context.Context,
	params MakeWatchedParams,
) (*Movie, error) {
	if params.Rating > 10 ||
		params.Rating < 1 {

		return nil,
			ErrRatingIsOutOfRange
	}

	if params.Review != nil &&
		utf8.RuneCountInString(
			*params.Review,
		) > maxMovieReviewRunes {

		return nil,
			ErrMovieReviewTooLong
	}

	rating :=
		params.Rating

	event :=
		Event{
			EventID: uuid.NewString(),

			Version: 1,

			Type: "MovieWatched",

			MovieID: params.ID,

			UserID: params.UserID,

			Rating: &rating,

			Review: params.Review,

			OccurredAt: time.Now().
				UTC(),
		}

	return service.writer.UpdateStatus(
		ctx,
		UpdateMovieStatusParams{
			ID: params.ID,

			UserID: params.UserID,

			Status: "watched",

			Rating: &rating,

			Review: params.Review,
		},
		event,
	)
}

// Make movie unwatched

type MakeUnwatchedParams struct {
	ID string

	UserID string
}

func (service *Service) MakeUnwatched(
	ctx context.Context,
	params MakeUnwatchedParams,
) (*Movie, error) {
	event :=
		Event{
			EventID: uuid.NewString(),

			Version: 1,

			Type: "MovieUnwatched",

			MovieID: params.ID,

			UserID: params.UserID,

			OccurredAt: time.Now().
				UTC(),
		}

	return service.writer.UpdateStatus(
		ctx,
		UpdateMovieStatusParams{
			ID: params.ID,

			UserID: params.UserID,

			Status: "unwatched",

			Rating: nil,

			Review: nil,
		},
		event,
	)
}

// Get random movie

type GetRandomParams struct {
	UserID string
}

func (service *Service) GetRandom(
	ctx context.Context,
	params GetRandomParams,
) (*Movie, error) {
	return service.repo.GetRandom(
		ctx,
		GetRandomMovieParams{
			UserID: params.UserID,
		},
	)
}

// Update metadata

type UpdateMetadataServiceParams struct {
	EventID string

	ID string

	UserID string

	ExternalID string

	MetadataProvider string

	OriginalTitle string

	Description string

	ReleaseYear int

	PosterURL string

	RuntimeMinutes *int

	Genres []string
}

func (service *Service) UpdateMetadata(
	ctx context.Context,
	params UpdateMetadataServiceParams,
) error {
	return service.repo.UpdateMetadataIdempotent(
		ctx,
		UpdateMetadataIdempotentParams{
			EventID: params.EventID,

			ID: params.ID,

			UserID: params.UserID,

			ExternalID: params.ExternalID,

			MetadataProvider: params.MetadataProvider,

			OriginalTitle: params.OriginalTitle,

			Description: params.Description,

			ReleaseYear: params.ReleaseYear,

			PosterURL: params.PosterURL,

			RuntimeMinutes: params.RuntimeMinutes,

			Genres: params.Genres,
		},
	)
}

// Mark metadata failed

type MarkMetadataFailedServiceParams struct {
	EventID string

	ID string

	UserID string

	Error string
}

func (service *Service) MarkMetadataFailed(
	ctx context.Context,
	params MarkMetadataFailedServiceParams,
) error {
	return service.repo.MarkMetadataFailed(
		ctx,
		MarkMetadataFailedParams{
			EventID: params.EventID,

			ID: params.ID,

			UserID: params.UserID,

			Error: params.Error,
		},
	)
}
