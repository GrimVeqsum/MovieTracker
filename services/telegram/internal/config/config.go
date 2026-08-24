package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	BotToken   string
	AuthURL    string
	LibraryURL string

	AuthServiceSecret string
}

func Load() (Config, error) {
	botToken, err := loadSecret(
		"TELEGRAM_BOT_TOKEN",
		"TELEGRAM_BOT_TOKEN_FILE",
	)
	if err != nil {
		return Config{}, err
	}

	authURL := os.Getenv(
		"TELEGRAM_AUTH_URL",
	)
	if authURL == "" {
		return Config{},
			errors.New(
				"TELEGRAM_AUTH_URL is empty",
			)
	}

	libraryURL := os.Getenv(
		"TELEGRAM_LIBRARY_URL",
	)
	if libraryURL == "" {
		return Config{},
			errors.New(
				"TELEGRAM_LIBRARY_URL is empty",
			)
	}

	authServiceSecret, err := loadSecret(
		"TELEGRAM_AUTH_SERVICE_SECRET",
		"TELEGRAM_AUTH_SERVICE_SECRET_FILE",
	)
	if err != nil {
		return Config{}, err
	}

	return Config{
		BotToken:          botToken,
		AuthURL:           authURL,
		LibraryURL:        libraryURL,
		AuthServiceSecret: authServiceSecret,
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
