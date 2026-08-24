package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	HTTPPort            string
	AuthServiceURL      string
	LibraryServiceURL   string
	TelegramBotUsername string
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

	telegramBotUsername := strings.TrimSpace(
		os.Getenv("TELEGRAM_BOT_USERNAME"),
	)

	if telegramBotUsername == "" {
		return Config{}, errors.New(
			"TELEGRAM_BOT_USERNAME is empty",
		)
	}

	telegramBotUsername = strings.TrimPrefix(
		telegramBotUsername,
		"@",
	)

	return Config{
		HTTPPort:            httpPort,
		AuthServiceURL:      authServiceURL,
		LibraryServiceURL:   libraryServiceURL,
		TelegramBotUsername: telegramBotUsername,
	}, nil
}
