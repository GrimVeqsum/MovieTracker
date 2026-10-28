package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "movie-platform/auth/docs"

	"movie-platform/auth/internal/config"
	authpostgres "movie-platform/auth/internal/platform/postgres"
	httptransport "movie-platform/auth/internal/transport/http"
	"movie-platform/auth/internal/users"
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
		return fmt.Errorf("load config: %w", err)
	}

	db, err :=
		authpostgres.NewConnection(
			context.Background(),
			cfg.DatabaseURL,
		)

	if err != nil {
		return fmt.Errorf("database connection: %w", err)
	}

	defer db.Close()

	handler :=
		httptransport.NewHandler(
			db,
		)

	userRepo :=
		users.NewRepository(
			db,
		)

	userService :=
		users.NewService(
			userRepo,
			cfg.JWTSecret,
			cfg.JWTIssuer,
			cfg.JWTAudience,
		)

	userHandler :=
		users.NewHandler(
			userService,
			cfg.CookieSecure,
			cfg.PublicOrigin,
		)

	telegramHandler :=
		users.NewTelegramHandler(
			userService,
			cfg.TelegramServiceSecret,
		)

	addr :=
		":" + cfg.HTTPPort

	server :=
		&http.Server{
			Addr: addr,

			Handler: httptransport.NewRouter(
				handler,
				userHandler,
				telegramHandler,
			),

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

	serveErrors := make(chan error, 1)
	go func() {
		log.Println("auth service started on", addr)
		serveErrors <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-serveErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		// Cancel active connection contexts before waiting for database users.
		_ = server.Close()
		return fmt.Errorf("shutdown HTTP: %w", err)
	}
	return nil
}
