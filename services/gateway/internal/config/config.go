package config

import (
	"errors"
	"os"
)

type Config struct {
	HTTPPort          string
	AuthServiceURL    string
	LibraryServiceURL string
}

func Load() (Config, error) {
	httpPort := os.Getenv("GATEWAY_HTTP_PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	authServiceURL := os.Getenv("AUTH_SERVICE_URL")
	if authServiceURL == "" {
		return Config{}, errors.New("AUTH_SERVICE_URL is empty")
	}

	libraryServiceURL := os.Getenv("LIBRARY_SERVICE_URL")
	if libraryServiceURL == "" {
		return Config{}, errors.New("LIBRARY_SERVICE_URL is empty")
	}

	return Config{
		HTTPPort:          httpPort,
		AuthServiceURL:    authServiceURL,
		LibraryServiceURL: libraryServiceURL,
	}, nil
}
