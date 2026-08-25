package http

import (
	"net/http"

	"movie-platform/auth/internal/users"

	httpSwagger "github.com/swaggo/http-swagger"
)

func NewRouter(
	handler *Handler,
	userHandler *users.Handler,
	telegramHandler *users.TelegramHandler,
) http.Handler {
	mux :=
		http.NewServeMux()

	mux.HandleFunc(
		"/swagger/",
		httpSwagger.WrapHandler,
	)

	mux.HandleFunc(
		"GET /health",
		handler.Health,
	)

	mux.HandleFunc(
		"GET /ready",
		handler.Ready,
	)

	mux.HandleFunc(
		"POST /auth/register",
		userHandler.Register,
	)

	mux.HandleFunc(
		"POST /auth/login",
		userHandler.Login,
	)

	mux.HandleFunc(
		"POST /auth/refresh",
		userHandler.Refresh,
	)

	mux.HandleFunc(
		"POST /auth/logout",
		userHandler.Logout,
	)

	mux.HandleFunc(
		"POST /auth/telegram/link-code",
		telegramHandler.CreateLinkCode,
	)

	mux.HandleFunc(
		"POST /internal/telegram/link",
		telegramHandler.Link,
	)

	mux.HandleFunc(
		"POST /internal/telegram/token",
		telegramHandler.Token,
	)

	return mux
}
