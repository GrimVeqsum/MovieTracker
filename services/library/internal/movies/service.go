package movies

import (
	"context"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// Get movies list
type ListParams struct {
	UserID string
}

func (service *Service) List(ctx context.Context, params ListParams) ([]Movie, error) {
	return service.repo.List(ctx, ListMovieParams{
		UserID: params.UserID,
	})
}

// Сreate movie
type CreateParams struct {
	UserID      string
	Title       string
	ReleaseYear *int
}

func (service *Service) Create(ctx context.Context, params CreateParams) (*Movie, error) {
	title := strings.TrimSpace(params.Title)

	if title == "" {
		return nil, ErrMovieTitleRequired
	}

	normalizedTitle := strings.ToLower(title)

	return service.repo.Create(ctx, CreateMovieParams{
		UserID:          params.UserID,
		Title:           title,
		NormalizedTitle: normalizedTitle,
		ReleaseYear:     params.ReleaseYear,
	})
}

// delete movie
type DeleteParams struct {
	UserID string
	ID     string
}

func (service *Service) Delete(ctx context.Context, params DeleteParams) error {
	return service.repo.Delete(ctx, DeleteMovieParams{
		UserID: params.UserID,
		ID:     params.ID,
	})
}

// Get movie by id
type GetParams struct {
	UserID string
	ID     string
}

func (service *Service) GetOne(ctx context.Context, params GetParams) (*Movie, error) {
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

func (service *Service) MakeWatched(ctx context.Context, params MakeWatchedParams) (*Movie, error) {
	if params.Rating > 10 || params.Rating < 1 {
		return nil, ErrRatingIsOutOfRange
	}

	return service.repo.UpdateStatus(ctx, UpdateMovieStatusParams{
		ID:     params.ID,
		UserID: params.UserID,
		Status: "watched",
		Rating: &params.Rating,
		Review: params.Review,
	})
}

// Make movie unwatched

type MakeUnwatchedParams struct {
	ID     string
	UserID string
}

func (service *Service) MakeUnwatched(ctx context.Context, params MakeUnwatchedParams) (*Movie, error) {

	return service.repo.UpdateStatus(ctx, UpdateMovieStatusParams{
		ID:     params.ID,
		UserID: params.UserID,
		Status: "unwatched",
		Rating: nil,
		Review: nil,
	})
}

// GET random movie
type GetRandomParams struct {
	UserID string
}

func (service *Service) GetRandom(ctx context.Context, params GetRandomParams) (*Movie, error) {
	return service.repo.GetRandom(ctx, GetRandomMovieParams{
		UserID: params.UserID,
	})
}
