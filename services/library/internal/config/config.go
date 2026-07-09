package config

import (
	"errors"
	"os"
)

type Config struct {
	HTTPPort    string
	DatabaseURL string
	JWTSecret   string
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

	return Config{
		HTTPPort:    httpPort,
		DatabaseURL: libURL,
		JWTSecret:   jwtSecret,
	}, nil
}
