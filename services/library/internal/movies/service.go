package movies

import (
	"context"
	"strings"
	"time"
)

type Service struct {
	repo      *Repository
	publisher EventPublisher
}

func NewService(
	repo *Repository,
	publisher EventPublisher,
) *Service {
	return &Service{
		repo:      repo,
		publisher: publisher,
	}
}

// Get movies list

type ListParams struct {
	UserID string
}

func (service *Service) List(
	ctx context.Context,
	params ListParams,
) ([]Movie, error) {
	return service.repo.List(ctx, ListMovieParams{
		UserID: params.UserID,
	})
}

// Create movie

type CreateParams struct {
	UserID      string
	Title       string
	ReleaseYear *int
}

func (service *Service) Create(
	ctx context.Context,
	params CreateParams,
) (*Movie, error) {
	title := strings.TrimSpace(params.Title)

	if title == "" {
		return nil, ErrMovieTitleRequired
	}

	normalizedTitle := strings.ToLower(title)

	movie, err := service.repo.Create(ctx, CreateMovieParams{
		UserID:          params.UserID,
		Title:           title,
		NormalizedTitle: normalizedTitle,
		ReleaseYear:     params.ReleaseYear,
	})
	if err != nil {
		return nil, err
	}

	event := Event{
		Type:       "MovieCreated",
		MovieID:    movie.ID,
		UserID:     movie.UserID,
		OccurredAt: movie.CreatedAt.UTC(),
	}

	if err := service.publisher.Publish(ctx, event); err != nil {
		return nil, err
	}

	return movie, nil
}

// Delete movie

type DeleteParams struct {
	UserID string
	ID     string
}

func (service *Service) Delete(
	ctx context.Context,
	params DeleteParams,
) error {
	err := service.repo.Delete(ctx, DeleteMovieParams{
		UserID: params.UserID,
		ID:     params.ID,
	})
	if err != nil {
		return err
	}

	event := Event{
		Type:       "MovieDeleted",
		MovieID:    params.ID,
		UserID:     params.UserID,
		OccurredAt: time.Now().UTC(),
	}

	if err := service.publisher.Publish(ctx, event); err != nil {
		return err
	}

	return nil
}

// Get movie by id

type GetParams struct {
	UserID string
	ID     string
}

func (service *Service) GetOne(
	ctx context.Context,
	params GetParams,
) (*Movie, error) {
	return service.repo.GetOne(ctx, GetOneParams{
		UserID: params.UserID,
		ID:     params.ID,
	})
}

// Make movie watched

type MakeWatchedParams struct {
	ID     string
	UserID string
	Rating int
	Review *string
}

func (service *Service) MakeWatched(
	ctx context.Context,
	params MakeWatchedParams,
) (*Movie, error) {
	if params.Rating > 10 || params.Rating < 1 {
		return nil, ErrRatingIsOutOfRange
	}

	movie, err := service.repo.UpdateStatus(ctx, UpdateMovieStatusParams{
		ID:     params.ID,
		UserID: params.UserID,
		Status: "watched",
		Rating: &params.Rating,
		Review: params.Review,
	})
	if err != nil {
		return nil, err
	}

	event := Event{
		Type:       "MovieWatched",
		MovieID:    movie.ID,
		UserID:     movie.UserID,
		Rating:     movie.Rating,
		Review:     movie.Review,
		OccurredAt: time.Now().UTC(),
	}

	if err := service.publisher.Publish(ctx, event); err != nil {
		return nil, err
	}

	return movie, nil
}

// Make movie unwatched

type MakeUnwatchedParams struct {
	ID     string
	UserID string
}

func (service *Service) MakeUnwatched(
	ctx context.Context,
	params MakeUnwatchedParams,
) (*Movie, error) {
	movie, err := service.repo.UpdateStatus(ctx, UpdateMovieStatusParams{
		ID:     params.ID,
		UserID: params.UserID,
		Status: "unwatched",
		Rating: nil,
		Review: nil,
	})
	if err != nil {
		return nil, err
	}

	event := Event{
		Type:       "MovieUnwatched",
		MovieID:    movie.ID,
		UserID:     movie.UserID,
		OccurredAt: time.Now().UTC(),
	}

	if err := service.publisher.Publish(ctx, event); err != nil {
		return nil, err
	}

	return movie, nil
}

// Get random movie

type GetRandomParams struct {
	UserID string
}

func (service *Service) GetRandom(
	ctx context.Context,
	params GetRandomParams,
) (*Movie, error) {
	return service.repo.GetRandom(ctx, GetRandomMovieParams{
		UserID: params.UserID,
	})
}
