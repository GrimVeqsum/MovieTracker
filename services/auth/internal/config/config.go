package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	defaultJWTIssuer   = "movietracker-auth"
	defaultJWTAudience = "movietracker-api"

	minJWTSecretLength = 32

	defaultDatabaseHost    = "auth-postgres"
	defaultDatabasePort    = "5432"
	defaultDatabaseUser    = "auth_user"
	defaultDatabaseName    = "auth_db"
	defaultDatabaseSSLMode = "disable"
)

type Config struct {
	HTTPPort string

	DatabaseURL string

	JWTSecret string

	JWTIssuer string

	JWTAudience string

	TelegramServiceSecret string

	CookieSecure bool
}

func Load() (Config, error) {
	httpPort :=
		strings.TrimSpace(
			os.Getenv(
				"AUTH_HTTP_PORT",
			),
		)

	if httpPort == "" {
		httpPort = "8082"
	}

	databaseURL, err :=
		loadDatabaseURL()

	if err != nil {
		return Config{}, err
	}

	jwtSecret, err :=
		loadSecret(
			"AUTH_JWT_SECRET",
			"AUTH_JWT_SECRET_FILE",
		)

	if err != nil {
		return Config{}, err
	}

	if len(jwtSecret) <
		minJWTSecretLength {

		return Config{},
			fmt.Errorf(
				"JWT secret must contain at least %d characters",
				minJWTSecretLength,
			)
	}

	jwtIssuer :=
		strings.TrimSpace(
			os.Getenv(
				"AUTH_JWT_ISSUER",
			),
		)

	if jwtIssuer == "" {
		jwtIssuer =
			defaultJWTIssuer
	}

	jwtAudience :=
		strings.TrimSpace(
			os.Getenv(
				"AUTH_JWT_AUDIENCE",
			),
		)

	if jwtAudience == "" {
		jwtAudience =
			defaultJWTAudience
	}

	telegramServiceSecret, err :=
		loadSecret(
			"AUTH_TELEGRAM_SERVICE_SECRET",
			"AUTH_TELEGRAM_SERVICE_SECRET_FILE",
		)

	if err != nil {
		return Config{}, err
	}

	cookieSecure, err :=
		loadBool(
			"AUTH_COOKIE_SECURE",
			false,
		)

	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPPort: httpPort,

		DatabaseURL: databaseURL,

		JWTSecret: jwtSecret,

		JWTIssuer: jwtIssuer,

		JWTAudience: jwtAudience,

		TelegramServiceSecret: telegramServiceSecret,

		CookieSecure: cookieSecure,
	}, nil
}

func loadDatabaseURL() (string, error) {
	directURL :=
		strings.TrimSpace(
			os.Getenv(
				"AUTH_DATABASE_URL",
			),
		)

	if directURL != "" {
		return directURL, nil
	}

	host :=
		envOrDefault(
			"AUTH_DATABASE_HOST",
			defaultDatabaseHost,
		)

	port :=
		envOrDefault(
			"AUTH_DATABASE_PORT",
			defaultDatabasePort,
		)

	user :=
		envOrDefault(
			"AUTH_DATABASE_USER",
			defaultDatabaseUser,
		)

	databaseName :=
		envOrDefault(
			"AUTH_DATABASE_NAME",
			defaultDatabaseName,
		)

	sslMode :=
		envOrDefault(
			"AUTH_DATABASE_SSLMODE",
			defaultDatabaseSSLMode,
		)

	password, err :=
		loadSecret(
			"AUTH_DATABASE_PASSWORD",
			"AUTH_DATABASE_PASSWORD_FILE",
		)

	if err != nil {
		return "",
			fmt.Errorf(
				"load database password: %w",
				err,
			)
	}

	databaseURL :=
		&url.URL{
			Scheme: "postgres",

			User: url.UserPassword(
				user,
				password,
			),

			Host: net.JoinHostPort(
				host,
				port,
			),

			Path: "/" + databaseName,
		}

	query :=
		databaseURL.Query()

	query.Set(
		"sslmode",
		sslMode,
	)

	databaseURL.RawQuery =
		query.Encode()

	return databaseURL.String(), nil
}

func loadSecret(
	envName string,
	fileEnvName string,
) (string, error) {
	if value :=
		strings.TrimSpace(
			os.Getenv(
				envName,
			),
		); value != "" {

		return value, nil
	}

	filePath :=
		strings.TrimSpace(
			os.Getenv(
				fileEnvName,
			),
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
			string(
				data,
			),
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

func envOrDefault(
	name string,
	defaultValue string,
) string {
	value :=
		strings.TrimSpace(
			os.Getenv(
				name,
			),
		)

	if value == "" {
		return defaultValue
	}

	return value
}

func loadBool(
	name string,
	defaultValue bool,
) (bool, error) {
	raw :=
		strings.TrimSpace(
			os.Getenv(
				name,
			),
		)

	if raw == "" {
		return defaultValue,
			nil
	}

	value, err :=
		strconv.ParseBool(
			raw,
		)

	if err != nil {
		return false,
			fmt.Errorf(
				"%s must be boolean: %w",
				name,
				err,
			)
	}

	return value, nil
}
