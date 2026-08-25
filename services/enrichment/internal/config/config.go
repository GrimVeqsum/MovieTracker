package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	KafkaBroker string
	KafkaTopic  string

	KafkaDLQTopic string

	KafkaGroup string

	MovieAPIKey string

	LibraryURL string

	LibraryServiceSecret string
}

func Load() (Config, error) {
	kafkaBroker :=
		strings.TrimSpace(
			os.Getenv(
				"ENRICHMENT_KAFKA_BROKER",
			),
		)

	if kafkaBroker == "" {
		return Config{},
			errors.New(
				"ENRICHMENT_KAFKA_BROKER is empty",
			)
	}

	kafkaTopic :=
		strings.TrimSpace(
			os.Getenv(
				"ENRICHMENT_KAFKA_TOPIC",
			),
		)

	if kafkaTopic == "" {
		return Config{},
			errors.New(
				"ENRICHMENT_KAFKA_TOPIC is empty",
			)
	}

	kafkaDLQTopic :=
		strings.TrimSpace(
			os.Getenv(
				"ENRICHMENT_KAFKA_DLQ_TOPIC",
			),
		)

	if kafkaDLQTopic == "" {
		kafkaDLQTopic =
			kafkaTopic + ".dlq"
	}

	kafkaGroup :=
		strings.TrimSpace(
			os.Getenv(
				"ENRICHMENT_KAFKA_GROUP",
			),
		)

	if kafkaGroup == "" {
		kafkaGroup =
			"movie-enrichment"
	}

	movieAPIKey, err :=
		loadSecret(
			"MOVIE_API_KEY",
			"MOVIE_API_KEY_FILE",
		)

	if err != nil {
		return Config{}, err
	}

	libraryURL :=
		strings.TrimSpace(
			os.Getenv(
				"ENRICHMENT_LIBRARY_URL",
			),
		)

	if libraryURL == "" {
		return Config{},
			errors.New(
				"ENRICHMENT_LIBRARY_URL is empty",
			)
	}

	libraryServiceSecret, err :=
		loadSecret(
			"ENRICHMENT_LIBRARY_SERVICE_SECRET",
			"ENRICHMENT_LIBRARY_SERVICE_SECRET_FILE",
		)

	if err != nil {
		return Config{}, err
	}

	return Config{
		KafkaBroker: kafkaBroker,

		KafkaTopic: kafkaTopic,

		KafkaDLQTopic: kafkaDLQTopic,

		KafkaGroup: kafkaGroup,

		MovieAPIKey: movieAPIKey,

		LibraryURL: libraryURL,

		LibraryServiceSecret: libraryServiceSecret,
	}, nil
}

func loadSecret(
	envName string,
	fileEnvName string,
) (string, error) {
	if value :=
		strings.TrimSpace(
			os.Getenv(envName),
		); value != "" {

		return value, nil
	}

	filePath :=
		strings.TrimSpace(
			os.Getenv(fileEnvName),
		)

	if filePath == "" {
		return "",
			fmt.Errorf(
				"%s or %s must be set",
				envName,
				fileEnvName,
			)
	}

	data, err :=
		os.ReadFile(
			filePath,
		)

	if err != nil {
		return "",
			fmt.Errorf(
				"read secret file %s: %w",
				filePath,
				err,
			)
	}

	value :=
		strings.TrimSpace(
			string(data),
		)

	if value == "" {
		return "",
			fmt.Errorf(
				"secret file %s is empty",
				filePath,
			)
	}

	return value, nil
}
