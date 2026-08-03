package httptransport

import "net/http"

func NewRouter(handler *Handler, authProxy http.Handler, libraryProxy http.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("GET /ready", handler.Ready)

	mux.Handle("/auth/", authProxy)

	mux.Handle("/movies", libraryProxy)
	mux.Handle("/movies/", libraryProxy)

	return mux
}
