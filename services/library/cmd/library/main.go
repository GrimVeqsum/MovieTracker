package main

import (
	"context"
	"log"
	"movie-platform/library/internal/config"
	"movie-platform/library/internal/movies"
	librarypostgres "movie-platform/library/internal/platform/postgres"
	httptransport "movie-platform/library/internal/transport/http"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "movie-platform/library/docs"

	"github.com/joho/godotenv"
)

// @title Movie Library API
// @version 1.0
// @description API for managing user's movie library.
// @host localhost:8081
// @BasePath /
func main() {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Println("Ошибка подключения к бд", err)
		return
	}
	db, err := librarypostgres.NewConnection(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Println("ошибка подключения к БД:", err)
		return
	}
	defer db.Close()

	movieRepo := movies.NewRepository(db)
	movieService := movies.NewService(movieRepo)
	movieHandler := movies.NewHandler(movieService)

	handler := httptransport.NewHandler(db)
	router := httptransport.NewRouter(handler, movieHandler)
	addr := ":" + cfg.HTTPPort

	log.Println("Сервер запущен на", addr)
	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}
	//follow notifications
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		log.Printf("ошибка остановки HTTP-сервера: %v", err)
	}
	log.Println("HTTP-сервер остановлен")
}
