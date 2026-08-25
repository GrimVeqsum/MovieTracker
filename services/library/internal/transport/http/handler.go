package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"movie-platform/library/internal/movies"
	"movie-platform/library/internal/transport/http/response"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxInternalRequestBodyBytes = 64 * 1024

type Handler struct {
	db *pgxpool.Pool

	movieService *movies.Service
}

func NewHandler(
	db *pgxpool.Pool,
	movieService *movies.Service,
) *Handler {
	return &Handler{
		db: db,

		movieService: movieService,
	}
}

type JSONCheck struct {
	Status string `json:"status"`

	Service string `json:"service"`
}

func (handler *Handler) Healther(
	w http.ResponseWriter,
	r *http.Request,
) {
	response.JSON(
		w,
		http.StatusOK,
		JSONCheck{
			Status: "ok",

			Service: "library",
		},
	)
}

func (handler *Handler) Ready(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx, cancel :=
		context.WithTimeout(
			r.Context(),
			2*time.Second,
		)

	defer cancel()

	if err :=
		handler.db.Ping(
			ctx,
		); err != nil {

		response.Error(
			w,
			http.StatusServiceUnavailable,
			"database_unavailable",
			"database unavailable",
		)

		return
	}

	response.JSON(
		w,
		http.StatusOK,
		JSONCheck{
			Status: "ok",

			Service: "library",
		},
	)
}

type UpdateMetadataRequest struct {
	EventID string `json:"event_id"`

	UserID string `json:"user_id"`

	ExternalID string `json:"external_id"`

	MetadataProvider string `json:"metadata_provider"`

	OriginalTitle string `json:"original_title"`

	Description string `json:"description"`

	ReleaseYear int `json:"release_year"`

	PosterURL string `json:"poster_url"`

	RuntimeMinutes *int `json:"runtime_minutes"`

	Genres []string `json:"genres"`
}

func (handler *Handler) UpdateMetadata(
	w http.ResponseWriter,
	r *http.Request,
) {
	movieID :=
		strings.TrimSpace(
			r.PathValue("id"),
		)

	var request UpdateMetadataRequest

	if err :=
		decodeInternalJSON(
			w,
			r,
			&request,
		); err != nil {

		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_request",
			err.Error(),
		)

		return
	}

	if _, err :=
		uuid.Parse(
			request.EventID,
		); err != nil {

		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_event_id",
			"event_id must be a valid UUID",
		)

		return
	}

	if strings.TrimSpace(
		request.UserID,
	) == "" {

		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_user_id",
			"user_id is required",
		)

		return
	}

	err :=
		handler.movieService.UpdateMetadata(
			r.Context(),
			movies.UpdateMetadataServiceParams{
				EventID: request.EventID,

				ID: movieID,

				UserID: request.UserID,

				ExternalID: request.ExternalID,

				MetadataProvider: request.MetadataProvider,

				OriginalTitle: request.OriginalTitle,

				Description: request.Description,

				ReleaseYear: request.ReleaseYear,

				PosterURL: request.PosterURL,

				RuntimeMinutes: request.RuntimeMinutes,

				Genres: request.Genres,
			},
		)

	if err != nil {
		writeMetadataError(
			w,
			err,
		)

		return
	}

	w.WriteHeader(
		http.StatusNoContent,
	)
}

type MarkMetadataFailedRequest struct {
	EventID string `json:"event_id"`

	UserID string `json:"user_id"`

	Error string `json:"error"`
}

func (handler *Handler) MarkMetadataFailed(
	w http.ResponseWriter,
	r *http.Request,
) {
	movieID :=
		strings.TrimSpace(
			r.PathValue("id"),
		)

	var request MarkMetadataFailedRequest

	if err :=
		decodeInternalJSON(
			w,
			r,
			&request,
		); err != nil {

		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_request",
			err.Error(),
		)

		return
	}

	if _, err :=
		uuid.Parse(
			request.EventID,
		); err != nil {

		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_event_id",
			"event_id must be a valid UUID",
		)

		return
	}

	if strings.TrimSpace(
		request.UserID,
	) == "" {

		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_user_id",
			"user_id is required",
		)

		return
	}

	err :=
		handler.movieService.MarkMetadataFailed(
			r.Context(),
			movies.MarkMetadataFailedServiceParams{
				EventID: request.EventID,

				ID: movieID,

				UserID: request.UserID,

				Error: request.Error,
			},
		)

	if err != nil {
		writeMetadataError(
			w,
			err,
		)

		return
	}

	w.WriteHeader(
		http.StatusNoContent,
	)
}

func decodeInternalJSON(
	w http.ResponseWriter,
	r *http.Request,
	target any,
) error {
	r.Body =
		http.MaxBytesReader(
			w,
			r.Body,
			maxInternalRequestBodyBytes,
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

		return errors.New(
			"invalid request body",
		)
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

func writeMetadataError(
	w http.ResponseWriter,
	err error,
) {
	if errors.Is(
		err,
		movies.ErrMovieNotFound,
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
		"internal server error",
	)
}
