package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	HTTPPort    string
	DatabaseURL string
	JWTSecret   string
	KafkaBroker string
	KafkaTopic  string
}

func Load() (Config, error) {
	httpPort := os.Getenv(
		"LIBRARY_HTTP_PORT",
	)
	if httpPort == "" {
		httpPort = "8081"
	}

	databaseURL := os.Getenv(
		"LIBRARY_DATABASE_URL",
	)
	if databaseURL == "" {
		return Config{},
			errors.New(
				"LIBRARY_DATABASE_URL is empty",
			)
	}

	jwtSecret, err := loadSecret(
		"LIBRARY_JWT_SECRET",
		"LIBRARY_JWT_SECRET_FILE",
	)
	if err != nil {
		return Config{}, err
	}

	kafkaBroker := os.Getenv(
		"LIBRARY_KAFKA_BROKER",
	)
	if kafkaBroker == "" {
		return Config{},
			errors.New(
				"LIBRARY_KAFKA_BROKER is empty",
			)
	}

	kafkaTopic := os.Getenv(
		"LIBRARY_KAFKA_TOPIC",
	)
	if kafkaTopic == "" {
		return Config{},
			errors.New(
				"LIBRARY_KAFKA_TOPIC is empty",
			)
	}

	return Config{
		HTTPPort:    httpPort,
		DatabaseURL: databaseURL,
		JWTSecret:   jwtSecret,
		KafkaBroker: kafkaBroker,
		KafkaTopic:  kafkaTopic,
	}, nil
}

func loadSecret(
	envName string,
	fileEnvName string,
) (string, error) {
	if value := strings.TrimSpace(
		os.Getenv(envName),
	); value != "" {
		return value, nil
	}

	filePath := strings.TrimSpace(
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

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "",
			fmt.Errorf(
				"read secret file %s: %w",
				filePath,
				err,
			)
	}

	value := strings.TrimSpace(
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
