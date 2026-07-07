package httptransport

import (
	"context"
	"movie-platform/library/internal/transport/http/response"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{
		db: db,
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
