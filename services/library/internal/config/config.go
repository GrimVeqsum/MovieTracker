package config

import (
	"errors"
	"os"
)

type Config struct {
	HTTPPort    string
	DatabaseURL string
}

func Load() (Config, error) {
	httpPort := os.Getenv("LIBRARY_HTTP_PORT")
	if httpPort == "" {
		httpPort = "8081"
	}
	libURL := os.Getenv("LIBRARY_DATABASE_URL")

	if libURL == "" {
		return Config{}, errors.New("DB URL is incorrect")
	}

	return Config{
		HTTPPort:    httpPort,
		DatabaseURL: libURL,
	}, nil
}
