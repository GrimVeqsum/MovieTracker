package movies

import (
	"encoding/json"
	"errors"
	"log"
	"movie-platform/library/internal/auth"
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

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	movie, err := handler.service.Create(r.Context(), CreateParams{
		UserID:      userID,
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
// @Description Returns all active movies for authenticated user
// @Tags movies
// @Produce json
// @Security BearerAuth
// @Success 200 {array} Movie
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /movies [get]
func (handler *Handler) GetMovieList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	movieList, err := handler.service.List(r.Context(), ListParams{
		UserID: userID,
	})

	if err != nil {
		response.Error(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(movieList)
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
func (handler *Handler) GetRandom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	movie, err := handler.service.GetRandom(r.Context(), GetRandomParams{
		UserID: userID,
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
func (handler *Handler) GetOne(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	movieID := r.PathValue("id")
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	if movieID == "" {
		response.Error(w, http.StatusBadRequest, "movie_id_required", "movie id is required")
		return
	}

	movie, err := handler.service.GetOne(r.Context(), GetParams{
		UserID: userID,
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

// Delete godoc
// @Summary Delete movie
// @Description Soft deletes movie by id for authenticated user
// @Tags movies
// @Security BearerAuth
// @Param id path string true "Movie ID"
// @Success 204 "No Content"
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /movies/{id} [delete]
func (handler *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		response.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	movieID := r.PathValue("id")
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	err := handler.service.Delete(r.Context(), DeleteParams{
		UserID: userID,
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

type makeWatchedRequest struct {
	Rating int     `json:"rating"`
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

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	movie, err := handler.service.MakeWatched(r.Context(), MakeWatchedParams{
		ID:     movieID,
		UserID: userID,
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

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	movie, err := handler.service.MakeUnwatched(r.Context(), MakeUnwatchedParams{
		ID:     movieID,
		UserID: userID,
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
