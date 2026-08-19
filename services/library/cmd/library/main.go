package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"movie-platform/library/internal/config"
	"movie-platform/library/internal/movies"
	librarykafka "movie-platform/library/internal/platform/kafka"
	librarypostgres "movie-platform/library/internal/platform/postgres"
	httptransport "movie-platform/library/internal/transport/http"

	_ "movie-platform/library/docs"

	"github.com/joho/godotenv"
)

// @title Movie Library API
// @version 1.0
// @description API for managing user's movie library.
// @host localhost:8081
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token. Example: "Bearer eyJhbGciOi..."
func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Println("ошибка загрузки config:", err)
		return
	}

	db, err := librarypostgres.NewConnection(
		context.Background(),
		cfg.DatabaseURL,
	)
	if err != nil {
		log.Println("ошибка подключения к БД:", err)
		return
	}
	defer db.Close()

	kafkaProducer, err := librarykafka.NewProducer(
		cfg.KafkaBroker,
		cfg.KafkaTopic,
	)
	if err != nil {
		log.Println("ошибка создания Kafka producer:", err)
		return
	}
	defer kafkaProducer.Close()

	movieRepo := movies.NewRepository(db)

	movieService := movies.NewService(
		movieRepo,
		kafkaProducer,
	)

	movieHandler := movies.NewHandler(movieService)

	handler := httptransport.NewHandler(db)

	router := httptransport.NewRouter(
		handler,
		movieHandler,
		cfg.JWTSecret,
	)

	addr := ":" + cfg.HTTPPort

	log.Println("Сервер запущен на", addr)

	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Printf("ошибка HTTP-сервера: %v", err)
		}
	}()

	<-ctx.Done()

	log.Println("получен сигнал завершения")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		log.Printf(
			"ошибка остановки HTTP-сервера: %v",
			err,
		)
	}

	log.Println("HTTP-сервер остановлен")
}
