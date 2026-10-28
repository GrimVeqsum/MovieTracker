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

	_ "movie-platform/library/docs"

	"movie-platform/library/internal/config"
	"movie-platform/library/internal/movies"
	"movie-platform/library/internal/outbox"
	librarykafka "movie-platform/library/internal/platform/kafka"
	librarypostgres "movie-platform/library/internal/platform/postgres"
	httptransport "movie-platform/library/internal/transport/http"

	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		log.Printf("library service failed: %v", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()

	cfg, err :=
		config.Load()

	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	db, err :=
		librarypostgres.NewConnection(
			context.Background(),
			cfg.DatabaseURL,
		)

	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}

	defer db.Close()

	kafkaProducer, err :=
		librarykafka.NewProducer(
			cfg.KafkaBroker,
			cfg.KafkaTopic,
		)

	if err != nil {
		return fmt.Errorf("create Kafka producer: %w", err)
	}

	defer kafkaProducer.Close()

	outboxRepository :=
		outbox.NewRepository(
			db,
		)

	movieRepository :=
		movies.NewRepository(
			db,
		)

	transactionalMovieRepository :=
		movies.NewTransactionalRepository(
			db,
			outboxRepository,
		)

	movieService :=
		movies.NewService(
			movieRepository,
			transactionalMovieRepository,
		)

	movieHandler :=
		movies.NewHandler(
			movieService,
		)

	handler :=
		httptransport.NewHandler(
			db,
			movieService,
		)

	router :=
		httptransport.NewRouter(
			handler,
			movieHandler,
			cfg.JWTSecret,
			cfg.JWTIssuer,
			cfg.JWTAudience,
			cfg.EnrichmentServiceSecret,
		)

	addr :=
		":" + cfg.HTTPPort

	server :=
		&http.Server{
			Addr: addr,

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

	outboxWorker :=
		outbox.NewWorker(
			outboxRepository,
			kafkaProducer,
		)

	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		outboxWorker.Run(ctx)
	}()

	serverErrors := make(chan error, 1)
	go func() {
		log.Println(
			"library service started on",
			addr,
		)

		err :=
			server.ListenAndServe()

		if err != nil &&
			err != http.ErrServerClosed {

			serverErrors <- err
			stop()
		}
	}()

	<-ctx.Done()

	log.Println(
		"shutdown signal received",
	)

	shutdownCtx, cancel :=
		context.WithTimeout(
			context.Background(),
			15*time.Second,
		)

	defer cancel()

	err =
		shutdownServer(
			shutdownCtx,
			server,
		)

	if err != nil {
		log.Printf(
			"HTTP server shutdown error: %v",
			err,
		)
	}

	<-workerDone
	log.Println("library service stopped")
	select {
	case serverErr := <-serverErrors:
		return serverErr
	default:
		return err
	}
}

// Shutdown alone leaves active request contexts alive when its deadline expires.
// Close cancels them before PostgreSQL waits for checked-out connections.
func shutdownServer(ctx context.Context, server *http.Server) error {
	if err := server.Shutdown(ctx); err != nil {
		_ = server.Close()
		return err
	}
	return nil
}
