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

	TelegramServiceSecret string
}

func Load() (Config, error) {
	httpPort := os.Getenv("AUTH_HTTP_PORT")
	if httpPort == "" {
		httpPort = "8082"
	}

	databaseURL := os.Getenv("AUTH_DATABASE_URL")
	if databaseURL == "" {
		return Config{},
			errors.New("AUTH_DATABASE_URL is empty")
	}

	jwtSecret, err := loadSecret(
		"AUTH_JWT_SECRET",
		"AUTH_JWT_SECRET_FILE",
	)
	if err != nil {
		return Config{}, err
	}

	telegramServiceSecret, err := loadSecret(
		"AUTH_TELEGRAM_SERVICE_SECRET",
		"AUTH_TELEGRAM_SERVICE_SECRET_FILE",
	)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPPort:              httpPort,
		DatabaseURL:           databaseURL,
		JWTSecret:             jwtSecret,
		TelegramServiceSecret: telegramServiceSecret,
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
