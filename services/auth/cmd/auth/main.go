package main

import (
	"context"
	"log"
	_ "movie-platform/auth/docs"
	"movie-platform/auth/internal/config"
	authpostgres "movie-platform/auth/internal/platform/postgres"
	httptransport "movie-platform/auth/internal/transport/http"
	"movie-platform/auth/internal/users"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// @title Movie Auth API
// @version 1.0
// @description API for user registration, login and logout.
// @host localhost:8082
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token. Example: "Bearer eyJhbGciOi..."
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db, err := authpostgres.NewConnection(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection error: %v", err)
	}
	defer db.Close()

	handler := httptransport.NewHandler(db)

	userRepo := users.NewRepository(db)
	userService := users.NewService(userRepo, cfg.JWTSecret)
	userHandler := users.NewHandler(userService)

	server := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: httptransport.NewRouter(handler, userHandler),
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	go func() {
		log.Println("auth service started on :" + cfg.HTTPPort)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}
