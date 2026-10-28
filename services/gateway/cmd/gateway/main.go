package main

import (
	"context"
	"fmt"
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
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err :=
		config.Load()

	if err != nil {
		return fmt.Errorf("config: %w", err)
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

	policy, err := httptransport.NewForwardingPolicy(cfg.TrustedProxies)
	if err != nil {
		return fmt.Errorf("proxy configuration: %w", err)
	}
	router = policy.Middleware(router)

	addr :=
		":" + cfg.HTTPPort

	server :=
		&http.Server{
			Addr:    addr,
			Handler: router,

			ReadHeaderTimeout: 5 * time.Second,

			ReadTimeout: 15 * time.Second,

			WriteTimeout: 30 * time.Second,

			IdleTimeout: 60 * time.Second,

			MaxHeaderBytes: 1 << 20,
		}

	ctx, stop :=
		signal.NotifyContext(
			context.Background(),
			os.Interrupt,
			syscall.SIGTERM,
		)

	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		log.Println(
			"gateway service started on",
			addr,
		)

		if err :=
			server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {

			serverErrors <- err
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel :=
		context.WithTimeout(
			context.Background(),
			30*time.Second,
		)

	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("gateway shutdown: %w", err)
	}
	select {
	case err := <-serverErrors:
		return fmt.Errorf("gateway server: %w", err)
	default:
		return nil
	}
}
