package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	defaultJWTIssuer   = "movietracker-auth"
	defaultJWTAudience = "movietracker-api"
	minJWTSecretLength = 32
)

type Config struct {
	HTTPPort    string
	DatabaseURL string

	JWTSecret   string
	JWTIssuer   string
	JWTAudience string

	KafkaBroker string
	KafkaTopic  string

	EnrichmentServiceSecret string
}

func Load() (Config, error) {
	httpPort :=
		strings.TrimSpace(
			os.Getenv(
				"LIBRARY_HTTP_PORT",
			),
		)

	if httpPort == "" {
		httpPort = "8081"
	}

	databaseURL :=
		strings.TrimSpace(
			os.Getenv(
				"LIBRARY_DATABASE_URL",
			),
		)

	if databaseURL == "" {
		return Config{},
			errors.New(
				"LIBRARY_DATABASE_URL is empty",
			)
	}

	jwtSecret, err :=
		loadSecret(
			"LIBRARY_JWT_SECRET",
			"LIBRARY_JWT_SECRET_FILE",
		)
	if err != nil {
		return Config{}, err
	}

	if len(jwtSecret) < minJWTSecretLength {
		return Config{},
			fmt.Errorf(
				"JWT secret must contain at least %d characters",
				minJWTSecretLength,
			)
	}

	jwtIssuer :=
		strings.TrimSpace(
			os.Getenv(
				"LIBRARY_JWT_ISSUER",
			),
		)

	if jwtIssuer == "" {
		jwtIssuer =
			defaultJWTIssuer
	}

	jwtAudience :=
		strings.TrimSpace(
			os.Getenv(
				"LIBRARY_JWT_AUDIENCE",
			),
		)

	if jwtAudience == "" {
		jwtAudience =
			defaultJWTAudience
	}

	kafkaBroker :=
		strings.TrimSpace(
			os.Getenv(
				"LIBRARY_KAFKA_BROKER",
			),
		)

	if kafkaBroker == "" {
		return Config{},
			errors.New(
				"LIBRARY_KAFKA_BROKER is empty",
			)
	}

	kafkaTopic :=
		strings.TrimSpace(
			os.Getenv(
				"LIBRARY_KAFKA_TOPIC",
			),
		)

	if kafkaTopic == "" {
		return Config{},
			errors.New(
				"LIBRARY_KAFKA_TOPIC is empty",
			)
	}

	enrichmentServiceSecret, err :=
		loadSecret(
			"LIBRARY_ENRICHMENT_SERVICE_SECRET",
			"LIBRARY_ENRICHMENT_SERVICE_SECRET_FILE",
		)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPPort:    httpPort,
		DatabaseURL: databaseURL,

		JWTSecret:   jwtSecret,
		JWTIssuer:   jwtIssuer,
		JWTAudience: jwtAudience,

		KafkaBroker: kafkaBroker,
		KafkaTopic:  kafkaTopic,

		EnrichmentServiceSecret: enrichmentServiceSecret,
	}, nil
}

func loadSecret(
	envName string,
	fileEnvName string,
) (string, error) {
	if value :=
		strings.TrimSpace(
			os.Getenv(envName),
		); value != "" {

		return value, nil
	}

	filePath :=
		strings.TrimSpace(
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

	data, err :=
		os.ReadFile(
			filePath,
		)
	if err != nil {
		return "",
			fmt.Errorf(
				"read secret file %s: %w",
				filePath,
				err,
			)
	}

	value :=
		strings.TrimSpace(
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
