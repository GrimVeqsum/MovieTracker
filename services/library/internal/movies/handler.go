package movies

import (
	"encoding/json"
	"errors"
	"log"
	"movie-platform/library/internal/transport/http/response"
	"net/http"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

type createMovieRequest struct {
	Title       string `json:"title"`
	ReleaseYear *int   `json:"release_year"`
}

// Create godoc
// @Summary Create new movie
// @Description Adding new movie to the list
// @Tags movies
// @Param request body createMovieRequest true "Movie data"
// @Success 200 {object} Movie
// @Failure 500 {object} response.ErrorResponse
// @Router /movies [post]
func (handler *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req createMovieRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_json", "invalid json")
		return
	}

	testUserID := "11111111-1111-1111-1111-111111111111"

	movie, err := handler.service.Create(r.Context(), CreateParams{
		UserID:      testUserID,
		Title:       req.Title,
		ReleaseYear: req.ReleaseYear,
	})
	if err != nil {
		if errors.Is(err, ErrMovieTitleRequired) {
			response.Error(w, http.StatusBadRequest, "movie_title_required", "movie title is required")
			return
		}

		if errors.Is(err, ErrMovieAlreadyExists) {
			response.Error(w, http.StatusConflict, "movie_already_exists", "movie already exists")
			return
		}

		response.Error(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(movie)
}

// GetMovieList godoc
// @Summary Get movie list
// @Description Returns all active movies for user
// @Tags movies
// @Produce json
// @Success 200 {array} Movie
// @Failure 500 {object} response.ErrorResponse
// @Router /movies [get]
func (handler *Handler) GetMovieList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	testUserID := "11111111-1111-1111-1111-111111111111"

	movieList, err := handler.service.List(r.Context(), ListParams{
		UserID: testUserID,
	})

	if err != nil {
		response.Error(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(movieList)
}

// Delete godoc
// @Summary Delete one movie
// @Description Deleting one movie from the list
// @Tags movies
// @Param id path string true "Movie ID"
// @Success 204 "No content"
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /movies/{id} [delete]
func (handler *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		response.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	movieID := r.PathValue("id")
	testUserID := "11111111-1111-1111-1111-111111111111"

	err := handler.service.Delete(r.Context(), DeleteParams{
		UserID: testUserID,
		ID:     movieID,
	})

	if err != nil {
		if errors.Is(err, ErrMovieNotFound) {
			response.Error(w, http.StatusNotFound, "movie_not_found", "movie not found")
			return
		}

		response.Error(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	w.WriteHeader(http.StatusOK)
	log.Println("Deleted movie with ID:", movieID)
}

// GetOne godoc
// @Summary Get movie by id
// @Description Returns one movie by id
// @Tags movies
// @Produce json
// @Param id path string true "Movie ID"
// @Success 200 {object} Movie
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /movies/{id} [get]
func (handler *Handler) GetOne(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	movieID := r.PathValue("id")
	testUserID := "11111111-1111-1111-1111-111111111111"

	if movieID == "" {
		response.Error(w, http.StatusBadRequest, "movie_id_required", "movie id is required")
		return
	}

	movie, err := handler.service.GetOne(r.Context(), GetParams{
		UserID: testUserID,
		ID:     movieID,
	})

	if err != nil {
		if errors.Is(err, ErrMovieNotFound) {
			response.Error(w, http.StatusNotFound, "movie_not_found", "movie not found")
			return
		}

		response.Error(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(movie)
}

type makeWatchedRequest struct {
	Rating int     `json:"rating"`
	Review *string `json:"review"`
}

// MakeWatched godoc
// @Summary Mark movie as watched
// @Description Changing movie status to "watched"
// @Tags movies
// @Produce json
// @Param request body makeWatchedRequest true "Movie data"
// @Param id path string true "Movie ID"
// @Success 200 {object} Movie
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /movies/{id}/watched [patch]
func (handler *Handler) MakeWatched(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		response.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	movieID := r.PathValue("id")

	if movieID == "" {
		response.Error(w, http.StatusBadRequest, "movie_id_required", "movie id is required")
		return
	}

	var req makeWatchedRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_json", "invalid json")
		return
	}

	testUserID := "11111111-1111-1111-1111-111111111111"

	movie, err := handler.service.MakeWatched(r.Context(), MakeWatchedParams{
		ID:     movieID,
		UserID: testUserID,
		Rating: req.Rating,
		Review: req.Review,
	})

	if err != nil {
		if errors.Is(err, ErrMovieNotFound) {
			response.Error(w, http.StatusNotFound, "movie_not_found", "movie not found")
			return
		}

		if errors.Is(err, ErrRatingIsOutOfRange) {
			response.Error(w, http.StatusBadRequest, "rating_out_of_range", "movie rating must be in range 1-10")
			return
		}

		response.Error(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(movie)
}

// MakeUnwatched godoc
// @Summary Mark movie as unwatched
// @Description Changing movie status to "unwatched"
// @Tags movies
// @Produce json
// @Param id path string true "Movie ID"
// @Success 204 "No Content"
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /movies/{id}/unwatched [patch]
func (handler *Handler) MakeUnwatched(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		response.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	movieID := r.PathValue("id")

	if movieID == "" {
		response.Error(w, http.StatusBadRequest, "movie_id_required", "movie id is required")
		return
	}

	testUserID := "11111111-1111-1111-1111-111111111111"

	movie, err := handler.service.MakeUnwatched(r.Context(), MakeUnwatchedParams{
		ID:     movieID,
		UserID: testUserID,
	})

	if err != nil {
		if errors.Is(err, ErrMovieNotFound) {
			response.Error(w, http.StatusNotFound, "movie_not_found", "movie not found")
			return
		}

		response.Error(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(movie)
}

// GetRandom godoc
// @Summary Get random movie
// @Description Returns random active movie for user
// @Tags movies
// @Produce json
// @Success 200 {object} Movie
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /movies/random [get]
func (handler *Handler) GetRandom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	testUserID := "11111111-1111-1111-1111-111111111111"

	movie, err := handler.service.GetRandom(r.Context(), GetRandomParams{
		UserID: testUserID,
	})
	if err != nil {
		if errors.Is(err, ErrMovieNotFound) {
			response.Error(w, http.StatusNotFound, "movie_not_found", "movie not found")
			return
		}

		response.Error(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(movie)
}
