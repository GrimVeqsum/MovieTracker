package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"movie-platform/gateway/internal/config"
	httptransport "movie-platform/gateway/internal/transport/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Println(
			"config error:",
			err,
		)
		return
	}

	handler :=
		httptransport.NewHandler(
			cfg.AuthServiceURL,
			cfg.LibraryServiceURL,
		)

	webHandler :=
		httptransport.NewWebHandler(
			cfg.TelegramBotUsername,
		)

	authProxy :=
		httptransport.NewReverseProxy(
			cfg.AuthServiceURL,
		)

	libraryProxy :=
		httptransport.NewReverseProxy(
			cfg.LibraryServiceURL,
		)

	router :=
		httptransport.NewRouter(
			handler,
			webHandler,
			cfg.TelegramBotUsername,
			authProxy,
			libraryProxy,
		)

	addr :=
		":" + cfg.HTTPPort

	server :=
		&http.Server{
			Addr:    addr,
			Handler: router,
		}

	ctx, stop :=
		signal.NotifyContext(
			context.Background(),
			os.Interrupt,
			syscall.SIGTERM,
		)

	defer stop()

	go func() {
		log.Println(
			"gateway service started on",
			addr,
		)

		if err :=
			server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {

			log.Printf(
				"server error: %v",
				err,
			)
		}
	}()

	<-ctx.Done()

	log.Println(
		"shutdown signal received",
	)

	shutdownCtx, cancel :=
		context.WithTimeout(
			context.Background(),
			5*time.Second,
		)

	defer cancel()

	if err :=
		server.Shutdown(
			shutdownCtx,
		); err != nil {

		log.Printf(
			"server shutdown error: %v",
			err,
		)
	}

	log.Println(
		"gateway service stopped",
	)
}
