package config

import (
	"errors"
	"os"
)

type Config struct {
	KafkaBroker string
	KafkaTopic  string
	KafkaGroup  string
	MovieAPIKey string
	LibraryURL  string
}

func Load() (Config, error) {
	kafkaBroker := os.Getenv(
		"ENRICHMENT_KAFKA_BROKER",
	)
	if kafkaBroker == "" {
		return Config{},
			errors.New(
				"ENRICHMENT_KAFKA_BROKER is empty",
			)
	}

	kafkaTopic := os.Getenv(
		"ENRICHMENT_KAFKA_TOPIC",
	)
	if kafkaTopic == "" {
		return Config{},
			errors.New(
				"ENRICHMENT_KAFKA_TOPIC is empty",
			)
	}

	kafkaGroup := os.Getenv(
		"ENRICHMENT_KAFKA_GROUP",
	)
	if kafkaGroup == "" {
		kafkaGroup = "movie-enrichment"
	}

	movieAPIKey := os.Getenv(
		"MOVIE_API_KEY",
	)
	if movieAPIKey == "" {
		return Config{},
			errors.New(
				"MOVIE_API_KEY is empty",
			)
	}

	libraryURL := os.Getenv(
		"ENRICHMENT_LIBRARY_URL",
	)
	if libraryURL == "" {
		return Config{},
			errors.New(
				"ENRICHMENT_LIBRARY_URL is empty",
			)
	}

	return Config{
		KafkaBroker: kafkaBroker,
		KafkaTopic:  kafkaTopic,
		KafkaGroup:  kafkaGroup,
		MovieAPIKey: movieAPIKey,
		LibraryURL:  libraryURL,
	}, nil
}
