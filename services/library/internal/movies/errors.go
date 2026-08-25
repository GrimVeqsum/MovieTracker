package movies

import "errors"

var (
	ErrMovieAlreadyExists = errors.New(
		"movie already exists",
	)

	ErrMovieNotFound = errors.New(
		"movie not found",
	)

	ErrMovieTitleRequired = errors.New(
		"movie title is required",
	)

	ErrMovieTitleTooLong = errors.New(
		"movie title is too long",
	)

	ErrRatingIsOutOfRange = errors.New(
		"movie rating must be in range 1-10",
	)

	ErrMovieReviewTooLong = errors.New(
		"movie review is too long",
	)
)
