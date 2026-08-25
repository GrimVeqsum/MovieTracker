package httptransport

import (
	"net/http"
	"time"
)

func NewRouter(
	handler *Handler,
	webHandler *WebHandler,
	telegramBotUsername string,
	authProxy http.Handler,
	libraryProxy http.Handler,
) http.Handler {
	mux :=
		http.NewServeMux()

	loginLimiter :=
		NewIPRateLimiter(
			5,
			time.Minute,
		)

	registerLimiter :=
		NewIPRateLimiter(
			3,
			time.Minute,
		)

	refreshLimiter :=
		NewIPRateLimiter(
			30,
			time.Minute,
		)

	telegramLinkLimiter :=
		NewIPRateLimiter(
			10,
			time.Minute,
		)

	mux.HandleFunc(
		"GET /{$}",
		func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			webHandler.Index(
				w,
				r,
				telegramBotUsername,
			)
		},
	)

	mux.HandleFunc(
		"GET /health",
		handler.Health,
	)

	mux.HandleFunc(
		"GET /ready",
		handler.Ready,
	)

	// Ограничиваем публичные Auth endpoints,
	// которые особенно интересны для brute force
	// или массового создания данных.

	mux.Handle(
		"POST /auth/login",
		loginLimiter.Middleware(
			authProxy,
		),
	)

	mux.Handle(
		"POST /auth/register",
		registerLimiter.Middleware(
			authProxy,
		),
	)

	mux.Handle(
		"POST /auth/refresh",
		refreshLimiter.Middleware(
			authProxy,
		),
	)

	mux.Handle(
		"POST /auth/telegram/link-code",
		telegramLinkLimiter.Middleware(
			authProxy,
		),
	)

	// Остальные /auth/* запросы продолжают
	// работать через обычный Auth proxy.
	mux.Handle(
		"/auth/",
		authProxy,
	)

	mux.Handle(
		"/movies",
		libraryProxy,
	)

	mux.Handle(
		"/movies/",
		libraryProxy,
	)

	return mux
}
