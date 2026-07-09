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
	httpPort := os.Getenv("AUTH_HTTP_PORT")
	if httpPort == "" {
		httpPort = "8082"
	}

	databaseURL := os.Getenv("AUTH_DATABASE_URL")
	if databaseURL == "" {
		return Config{}, errors.New("AUTH_DATABASE_URL is empty")
	}

	jwtSecret := os.Getenv("AUTH_JWT_SECRET")
	if jwtSecret == "" {
		return Config{}, errors.New("AUTH_JWT_SECRET is empty")
	}

	return Config{
		HTTPPort:    httpPort,
		DatabaseURL: databaseURL,
		JWTSecret:   jwtSecret,
	}, nil
}
