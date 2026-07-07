package movies

import (
	"errors"
)

var ErrMovieAlreadyExists = errors.New("movie already exists")
var ErrMovieNotFound = errors.New("movie not found")
var ErrMovieTitleRequired = errors.New("movie title is required")
var ErrRatingIsOutOfRange = errors.New("movie rating must be in range 1-10")
