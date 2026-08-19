package config

import (
	"errors"
	"os"
)

type Config struct {
	HTTPPort    string
	DatabaseURL string
	JWTSecret   string
	KafkaBroker string
	KafkaTopic  string
}

func Load() (Config, error) {
	httpPort := os.Getenv("LIBRARY_HTTP_PORT")
	if httpPort == "" {
		httpPort = "8081"
	}

	libURL := os.Getenv("LIBRARY_DATABASE_URL")
	if libURL == "" {
		return Config{}, errors.New("LIBRARY_DATABASE_URL is empty")
	}

	jwtSecret := os.Getenv("LIBRARY_JWT_SECRET")
	if jwtSecret == "" {
		return Config{}, errors.New("LIBRARY_JWT_SECRET is empty")
	}

	kafkaBroker := os.Getenv("LIBRARY_KAFKA_BROKER")
	if kafkaBroker == "" {
		return Config{}, errors.New("LIBRARY_KAFKA_BROKER is empty")
	}

	kafkaTopic := os.Getenv("LIBRARY_KAFKA_TOPIC")
	if kafkaTopic == "" {
		return Config{}, errors.New("LIBRARY_KAFKA_TOPIC is empty")
	}

	return Config{
		HTTPPort:    httpPort,
		DatabaseURL: libURL,
		JWTSecret:   jwtSecret,
		KafkaBroker: kafkaBroker,
		KafkaTopic:  kafkaTopic,
	}, nil
}
