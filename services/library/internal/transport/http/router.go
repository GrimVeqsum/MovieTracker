package httptransport

import (
	"net/http"

	"movie-platform/library/internal/auth"
	"movie-platform/library/internal/movies"

	httpSwagger "github.com/swaggo/http-swagger"
)

func NewRouter(
	handler *Handler,
	movieHandler *movies.Handler,
	jwtSecret string,
) http.Handler {
	mux := http.NewServeMux()

	authMiddleware := auth.NewMiddleware(jwtSecret)

	mux.HandleFunc("GET /health", handler.Healther)
	mux.HandleFunc("GET /ready", handler.Ready)

	mux.HandleFunc("/swagger/", httpSwagger.WrapHandler)

	mux.Handle("POST /movies", authMiddleware(http.HandlerFunc(movieHandler.Create)))
	mux.Handle("GET /movies", authMiddleware(http.HandlerFunc(movieHandler.GetMovieList)))
	mux.Handle("GET /movies/random", authMiddleware(http.HandlerFunc(movieHandler.GetRandom)))
	mux.Handle("GET /movies/{id}", authMiddleware(http.HandlerFunc(movieHandler.GetOne)))
	mux.Handle("DELETE /movies/{id}", authMiddleware(http.HandlerFunc(movieHandler.Delete)))
	mux.Handle("PATCH /movies/{id}/watched", authMiddleware(http.HandlerFunc(movieHandler.MakeWatched)))
	mux.Handle("PATCH /movies/{id}/unwatched", authMiddleware(http.HandlerFunc(movieHandler.MakeUnwatched)))
	mux.HandleFunc("PATCH /internal/movies/{id}/metadata", handler.UpdateMetadata)

	return mux
}
