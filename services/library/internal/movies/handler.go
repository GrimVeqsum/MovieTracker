package movies

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"movie-platform/library/internal/auth"
	"movie-platform/library/internal/transport/http/response"

	"github.com/google/uuid"
)

const maxMovieRequestBodyBytes = 64 * 1024

type Handler struct {
	service *Service
}

func NewHandler(
	service *Service,
) *Handler {
	return &Handler{
		service: service,
	}
}

type createMovieRequest struct {
	Title string `json:"title"`

	ReleaseYear *int `json:"release_year"`
}

// Create godoc
// @Summary Create new movie
// @Description Adding new movie to the user's movie list
// @Tags movies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body createMovieRequest true "Movie data"
// @Success 201 {object} Movie
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /movies [post]
func (handler *Handler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request createMovieRequest

	if err :=
		decodeMovieJSON(
			w,
			r,
			&request,
		); err != nil {

		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_json",
			"invalid json",
		)

		return
	}

	userID, ok :=
		auth.UserIDFromContext(
			r.Context(),
		)

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"unauthorized",
			"unauthorized",
		)

		return
	}

	movie, err :=
		handler.service.Create(
			r.Context(),
			CreateParams{
				UserID: userID,

				Title: request.Title,

				ReleaseYear: request.ReleaseYear,
			},
		)

	if err != nil {
		switch {
		case errors.Is(
			err,
			ErrMovieTitleRequired,
		):
			response.Error(
				w,
				http.StatusBadRequest,
				"movie_title_required",
				"movie title is required",
			)

		case errors.Is(
			err,
			ErrMovieTitleTooLong,
		):
			response.Error(
				w,
				http.StatusBadRequest,
				"movie_title_too_long",
				"movie title must contain at most 300 characters",
			)

		case errors.Is(
			err,
			ErrMovieAlreadyExists,
		):
			response.Error(
				w,
				http.StatusConflict,
				"movie_already_exists",
				"movie already exists",
			)

		default:
			response.Error(
				w,
				http.StatusInternalServerError,
				"internal_error",
				"internal error",
			)
		}

		return
	}

	response.JSON(
		w,
		http.StatusCreated,
		movie,
	)
}

// GetMovieList godoc
// @Summary Get movie list
// @Description Returns all active movies for authenticated user
// @Tags movies
// @Produce json
// @Security BearerAuth
// @Success 200 {array} Movie
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /movies [get]
func (handler *Handler) GetMovieList(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok :=
		auth.UserIDFromContext(
			r.Context(),
		)

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"unauthorized",
			"unauthorized",
		)

		return
	}

	movieList, err :=
		handler.service.List(
			r.Context(),
			ListParams{
				UserID: userID,
			},
		)

	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"internal_error",
			"internal error",
		)

		return
	}

	response.JSON(
		w,
		http.StatusOK,
		movieList,
	)
}

// GetRandom godoc
// @Summary Get random movie
// @Description Returns random active movie for authenticated user
// @Tags movies
// @Produce json
// @Security BearerAuth
// @Success 200 {object} Movie
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /movies/random [get]
func (handler *Handler) GetRandom(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok :=
		auth.UserIDFromContext(
			r.Context(),
		)

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"unauthorized",
			"unauthorized",
		)

		return
	}

	movie, err :=
		handler.service.GetRandom(
			r.Context(),
			GetRandomParams{
				UserID: userID,
			},
		)

	if err != nil {
		if errors.Is(
			err,
			ErrMovieNotFound,
		) {
			response.Error(
				w,
				http.StatusNotFound,
				"movie_not_found",
				"movie not found",
			)

			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"internal_error",
			"internal error",
		)

		return
	}

	response.JSON(
		w,
		http.StatusOK,
		movie,
	)
}

// GetOne godoc
// @Summary Get movie by id
// @Description Returns one movie by id for authenticated user
// @Tags movies
// @Produce json
// @Security BearerAuth
// @Param id path string true "Movie ID"
// @Success 200 {object} Movie
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /movies/{id} [get]
func (handler *Handler) GetOne(
	w http.ResponseWriter,
	r *http.Request,
) {
	movieID, ok :=
		readMovieID(
			w,
			r,
		)

	if !ok {
		return
	}

	userID, ok :=
		auth.UserIDFromContext(
			r.Context(),
		)

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"unauthorized",
			"unauthorized",
		)

		return
	}

	movie, err :=
		handler.service.GetOne(
			r.Context(),
			GetParams{
				UserID: userID,

				ID: movieID,
			},
		)

	if err != nil {
		if errors.Is(
			err,
			ErrMovieNotFound,
		) {
			response.Error(
				w,
				http.StatusNotFound,
				"movie_not_found",
				"movie not found",
			)

			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"internal_error",
			"internal error",
		)

		return
	}

	response.JSON(
		w,
		http.StatusOK,
		movie,
	)
}

// Delete godoc
// @Summary Delete movie
// @Description Soft deletes movie by id for authenticated user
// @Tags movies
// @Security BearerAuth
// @Param id path string true "Movie ID"
// @Success 204 "No Content"
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /movies/{id} [delete]
func (handler *Handler) Delete(
	w http.ResponseWriter,
	r *http.Request,
) {
	movieID, ok :=
		readMovieID(
			w,
			r,
		)

	if !ok {
		return
	}

	userID, ok :=
		auth.UserIDFromContext(
			r.Context(),
		)

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"unauthorized",
			"unauthorized",
		)

		return
	}

	err :=
		handler.service.Delete(
			r.Context(),
			DeleteParams{
				UserID: userID,

				ID: movieID,
			},
		)

	if err != nil {
		if errors.Is(
			err,
			ErrMovieNotFound,
		) {
			response.Error(
				w,
				http.StatusNotFound,
				"movie_not_found",
				"movie not found",
			)

			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"internal_error",
			"internal error",
		)

		return
	}

	w.WriteHeader(
		http.StatusNoContent,
	)
}

type makeWatchedRequest struct {
	Rating int `json:"rating"`

	Review *string `json:"review"`
}

// MakeWatched godoc
// @Summary Mark movie as watched
// @Description Changes movie status to watched and saves rating/review
// @Tags movies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Movie ID"
// @Param request body makeWatchedRequest true "Watched movie data"
// @Success 200 {object} Movie
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /movies/{id}/watched [patch]
func (handler *Handler) MakeWatched(
	w http.ResponseWriter,
	r *http.Request,
) {
	movieID, ok :=
		readMovieID(
			w,
			r,
		)

	if !ok {
		return
	}

	var request makeWatchedRequest

	if err :=
		decodeMovieJSON(
			w,
			r,
			&request,
		); err != nil {

		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_json",
			"invalid json",
		)

		return
	}

	userID, ok :=
		auth.UserIDFromContext(
			r.Context(),
		)

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"unauthorized",
			"unauthorized",
		)

		return
	}

	movie, err :=
		handler.service.MakeWatched(
			r.Context(),
			MakeWatchedParams{
				ID: movieID,

				UserID: userID,

				Rating: request.Rating,

				Review: request.Review,
			},
		)

	if err != nil {
		switch {
		case errors.Is(
			err,
			ErrMovieNotFound,
		):
			response.Error(
				w,
				http.StatusNotFound,
				"movie_not_found",
				"movie not found",
			)

		case errors.Is(
			err,
			ErrRatingIsOutOfRange,
		):
			response.Error(
				w,
				http.StatusBadRequest,
				"rating_out_of_range",
				"movie rating must be in range 1-10",
			)

		case errors.Is(
			err,
			ErrMovieReviewTooLong,
		):
			response.Error(
				w,
				http.StatusBadRequest,
				"movie_review_too_long",
				"movie review must contain at most 5000 characters",
			)

		default:
			response.Error(
				w,
				http.StatusInternalServerError,
				"internal_error",
				"internal error",
			)
		}

		return
	}

	response.JSON(
		w,
		http.StatusOK,
		movie,
	)
}

// MakeUnwatched godoc
// @Summary Mark movie as unwatched
// @Description Changes movie status to unwatched and clears rating, review and watched_at
// @Tags movies
// @Produce json
// @Security BearerAuth
// @Param id path string true "Movie ID"
// @Success 200 {object} Movie
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /movies/{id}/unwatched [patch]
func (handler *Handler) MakeUnwatched(
	w http.ResponseWriter,
	r *http.Request,
) {
	movieID, ok :=
		readMovieID(
			w,
			r,
		)

	if !ok {
		return
	}

	userID, ok :=
		auth.UserIDFromContext(
			r.Context(),
		)

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"unauthorized",
			"unauthorized",
		)

		return
	}

	movie, err :=
		handler.service.MakeUnwatched(
			r.Context(),
			MakeUnwatchedParams{
				ID: movieID,

				UserID: userID,
			},
		)

	if err != nil {
		if errors.Is(
			err,
			ErrMovieNotFound,
		) {
			response.Error(
				w,
				http.StatusNotFound,
				"movie_not_found",
				"movie not found",
			)

			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"internal_error",
			"internal error",
		)

		return
	}

	response.JSON(
		w,
		http.StatusOK,
		movie,
	)
}

func decodeMovieJSON(
	w http.ResponseWriter,
	r *http.Request,
	target any,
) error {
	r.Body =
		http.MaxBytesReader(
			w,
			r.Body,
			maxMovieRequestBodyBytes,
		)

	decoder :=
		json.NewDecoder(
			r.Body,
		)

	decoder.DisallowUnknownFields()

	if err :=
		decoder.Decode(
			target,
		); err != nil {

		return err
	}

	var extra any

	if err :=
		decoder.Decode(
			&extra,
		); !errors.Is(
		err,
		io.EOF,
	) {

		return errors.New(
			"request body must contain one JSON object",
		)
	}

	return nil
}

func readMovieID(
	w http.ResponseWriter,
	r *http.Request,
) (string, bool) {
	rawID :=
		strings.TrimSpace(
			r.PathValue(
				"id",
			),
		)

	if rawID == "" {
		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_movie_id",
			"movie id is required",
		)

		return "", false
	}

	movieID, err :=
		uuid.Parse(
			rawID,
		)

	if err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_movie_id",
			"movie id must be a valid UUID",
		)

		return "", false
	}

	return movieID.String(),
		true
}
