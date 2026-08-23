package config

import (
	"errors"
	"os"
)

type Config struct {
	BotToken   string
	AuthURL    string
	LibraryURL string
}

func Load() (Config, error) {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		return Config{},
			errors.New("TELEGRAM_BOT_TOKEN is empty")
	}

	authURL := os.Getenv("TELEGRAM_AUTH_URL")
	if authURL == "" {
		return Config{},
			errors.New("TELEGRAM_AUTH_URL is empty")
	}

	libraryURL := os.Getenv("TELEGRAM_LIBRARY_URL")
	if libraryURL == "" {
		return Config{},
			errors.New("TELEGRAM_LIBRARY_URL is empty")
	}

	return Config{
		BotToken:   botToken,
		AuthURL:    authURL,
		LibraryURL: libraryURL,
	}, nil
}
