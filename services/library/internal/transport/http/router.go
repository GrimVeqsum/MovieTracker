package httptransport

import (
	"movie-platform/library/internal/movies"
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"
)

func NewRouter(handler *Handler, movieHandler *movies.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handler.Healther)
	mux.HandleFunc("GET /ready", handler.Ready)
	mux.HandleFunc("GET /swagger/", httpSwagger.WrapHandler)
	mux.HandleFunc("POST /movies", movieHandler.Create)
	mux.HandleFunc("GET /movies", movieHandler.GetMovieList)
	mux.HandleFunc("DELETE /movies/{id}", movieHandler.Delete)
	mux.HandleFunc("GET /movies/{id}", movieHandler.GetOne)
	mux.HandleFunc("GET /movies/random", movieHandler.GetRandom)
	mux.HandleFunc("PATCH /movies/{id}/watched", movieHandler.MakeWatched)
	mux.HandleFunc("PATCH /movies/{id}/unwatched", movieHandler.MakeUnwatched)

	return mux
}
