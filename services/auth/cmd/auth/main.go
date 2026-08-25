package main

import (
	"context"
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
	cfg, err :=
		config.Load()

	if err != nil {
		log.Fatal(
			err,
		)
	}

	db, err :=
		authpostgres.NewConnection(
			context.Background(),
			cfg.DatabaseURL,
		)

	if err != nil {
		log.Fatalf(
			"database connection error: %v",
			err,
		)
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

	go func() {
		log.Println(
			"auth service started on",
			addr,
		)

		if err :=
			server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {

			log.Fatalf(
				"server error: %v",
				err,
			)
		}
	}()

	<-ctx.Done()

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
}
