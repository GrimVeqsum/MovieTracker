package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"movie-platform/library/internal/movies"
	"movie-platform/library/internal/transport/http/response"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db           *pgxpool.Pool
	movieService *movies.Service
}

func NewHandler(
	db *pgxpool.Pool,
	movieService *movies.Service,
) *Handler {
	return &Handler{
		db:           db,
		movieService: movieService,
	}
}

type JSONCheck struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// Healther godoc
// @Summary Healthcheck of the services
// @Description Shows health of the services
// @Tags CheckServicesStatus
// @Produce json
// @Success 200 {object} JSONCheck
// @Failure 405   {object} response.ErrorResponse
// @Router /health [get]
func (h *Handler) Healther(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	response.JSON(w, http.StatusOK, JSONCheck{
		Status:  "ok",
		Service: "library",
	})
}

// Ready godoc
// @Summary Readycheck of the services
// @Description Shows readiness of the service and database connection
// @Tags CheckServicesStatus
// @Produce json
// @Success 200 {object} JSONCheck
// @Failure 405 {object} response.ErrorResponse
// @Failure 503 {object} response.ErrorResponse
// @Router /ready [get]
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	err := h.db.Ping(ctx)
	if err != nil {
		response.Error(w, http.StatusServiceUnavailable, "database_unavailable", "database unavailable")
		return
	}
	response.JSON(w, http.StatusOK, JSONCheck{
		Status:  "ok",
		Service: "library",
	})
}

type UpdateMetadataRequest struct {
	UserID           string   `json:"user_id"`
	ExternalID       string   `json:"external_id"`
	MetadataProvider string   `json:"metadata_provider"`
	OriginalTitle    string   `json:"original_title"`
	Description      string   `json:"description"`
	ReleaseYear      int      `json:"release_year"`
	PosterURL        string   `json:"poster_url"`
	RuntimeMinutes   *int     `json:"runtime_minutes"`
	Genres           []string `json:"genres"`
}

func (handler *Handler) UpdateMetadata(
	w http.ResponseWriter,
	r *http.Request,
) {
	movieID := r.PathValue("id")

	var request UpdateMetadataRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(
			w,
			"invalid request body",
			http.StatusBadRequest,
		)
		return
	}

	err := handler.movieService.UpdateMetadata(
		r.Context(),
		movies.UpdateMetadataServiceParams{
			ID:               movieID,
			UserID:           request.UserID,
			ExternalID:       request.ExternalID,
			MetadataProvider: request.MetadataProvider,
			OriginalTitle:    request.OriginalTitle,
			Description:      request.Description,
			ReleaseYear:      request.ReleaseYear,
			PosterURL:        request.PosterURL,
			RuntimeMinutes:   request.RuntimeMinutes,
			Genres:           request.Genres,
		},
	)
	if err != nil {
		if errors.Is(err, movies.ErrMovieNotFound) {
			http.Error(
				w,
				"movie not found",
				http.StatusNotFound,
			)
			return
		}

		http.Error(
			w,
			"internal server error",
			http.StatusInternalServerError,
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
