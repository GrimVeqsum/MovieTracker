package main

import (
	"context"
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
	_ = godotenv.Load()

	cfg, err :=
		config.Load()

	if err != nil {
		log.Printf(
			"config error: %v",
			err,
		)

		return
	}

	db, err :=
		librarypostgres.NewConnection(
			context.Background(),
			cfg.DatabaseURL,
		)

	if err != nil {
		log.Printf(
			"database connection error: %v",
			err,
		)

		return
	}

	defer db.Close()

	kafkaProducer, err :=
		librarykafka.NewProducer(
			cfg.KafkaBroker,
			cfg.KafkaTopic,
		)

	if err != nil {
		log.Printf(
			"Kafka producer error: %v",
			err,
		)

		return
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

	go outboxWorker.Run(
		ctx,
	)

	go func() {
		log.Println(
			"library service started on",
			addr,
		)

		err :=
			server.ListenAndServe()

		if err != nil &&
			err != http.ErrServerClosed {

			log.Printf(
				"HTTP server error: %v",
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

	err =
		server.Shutdown(
			shutdownCtx,
		)

	if err != nil {
		log.Printf(
			"HTTP server shutdown error: %v",
			err,
		)
	}

	log.Println(
		"library service stopped",
	)
}
