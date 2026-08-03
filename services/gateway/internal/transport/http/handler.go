package httptransport

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Handler struct {
	authServiceURL    string
	libraryServiceURL string
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func NewHandler(authServiceURL string, libraryServiceURL string) *Handler {
	return &Handler{
		authServiceURL:    authServiceURL,
		libraryServiceURL: libraryServiceURL,
	}
}

func (handler *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:  "ok",
		Service: "gateway",
	})
}

func (handler *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if !checkService(ctx, handler.authServiceURL+"/ready") {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":  "not_ready",
			"service": "gateway",
			"reason":  "auth service is not ready",
		})
		return
	}

	if !checkService(ctx, handler.libraryServiceURL+"/ready") {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":  "not_ready",
			"service": "gateway",
			"reason":  "library service is not ready",
		})
		return
	}

	writeJSON(w, http.StatusOK, HealthResponse{
		Status:  "ready",
		Service: "gateway",
	})
}

func checkService(ctx context.Context, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
