package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"movie-platform/enrichment/internal/config"
	kafkaconsumer "movie-platform/enrichment/internal/kafka"
	"movie-platform/enrichment/internal/library"
	"movie-platform/enrichment/internal/poiskkino"
)

func main() {
	if err := run(); err != nil {
		log.Printf("enrichment-service failed: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err :=
		config.Load()

	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	movieClient :=
		poiskkino.NewClientWithBaseURL(
			cfg.MovieAPIKey,
			cfg.MovieAPIURL,
		)

	libraryClient :=
		library.NewClient(
			cfg.LibraryURL,
			cfg.LibraryServiceSecret,
		)

	consumer, err :=
		kafkaconsumer.NewConsumer(
			cfg.KafkaBroker,
			cfg.KafkaTopic,
			cfg.KafkaDLQTopic,
			cfg.KafkaGroup,
			movieClient,
			libraryClient,
		)

	if err != nil {
		return err
	}

	defer consumer.Close()

	ctx, stop :=
		signal.NotifyContext(
			context.Background(),
			os.Interrupt,
			syscall.SIGTERM,
		)

	defer stop()

	log.Println(
		"enrichment-service started",
	)

	if err := consumer.Run(ctx); err != nil {
		return err
	}

	log.Println(
		"enrichment-service stopped",
	)
	return nil
}
