package main

import (
	"context"
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
	cfg, err :=
		config.Load()

	if err != nil {
		log.Printf(
			"config error: %v",
			err,
		)

		return
	}

	movieClient :=
		poiskkino.NewClient(
			cfg.MovieAPIKey,
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
		log.Printf(
			"Kafka consumer error: %v",
			err,
		)

		return
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

	consumer.Run(
		ctx,
	)

	log.Println(
		"enrichment-service stopped",
	)
}
