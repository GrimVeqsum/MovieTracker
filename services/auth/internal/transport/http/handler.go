package http

import (
	"context"
	"net/http"
	"time"

	"movie-platform/auth/internal/transport/http/response"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db *pgxpool.Pool
}

type CheckResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{
		db: db,
	}
}

func (handler *Handler) Health(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, CheckResponse{
		Status:  "ok",
		Service: "auth",
	})
}

func (handler *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := handler.db.Ping(ctx); err != nil {
		response.Error(w, http.StatusServiceUnavailable, "db_not_ready", "database is not ready")
		return
	}

	response.JSON(w, http.StatusOK, CheckResponse{
		Status:  "ready",
		Service: "auth",
	})
}
