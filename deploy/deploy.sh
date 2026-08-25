#!/bin/sh

set -eu


SCRIPT_DIR="$(
    CDPATH= cd -- "$(dirname -- "$0")" &&
    pwd
)"

PROJECT_DIR="$(
    dirname "$SCRIPT_DIR"
)"

cd "$PROJECT_DIR"


if [ -z "${MOVIETRACKER_DOMAIN:-}" ]; then
    echo "MOVIETRACKER_DOMAIN is not set" >&2
    exit 1
fi


if ! command -v docker >/dev/null 2>&1; then
    echo "Docker is not installed" >&2
    exit 1
fi


if ! docker compose version >/dev/null 2>&1; then
    echo "Docker Compose is not available" >&2
    exit 1
fi


SECRETS_DIR="$PROJECT_DIR/.secrets"

REQUIRED_SECRETS="
auth_db_password
library_db_password
jwt_secret
telegram_bot_token
telegram_auth_service_token
movie_api_key
enrichment_library_service_token
"


if [ ! -d "$SECRETS_DIR" ]; then
    echo "Secrets directory does not exist: $SECRETS_DIR" >&2
    exit 1
fi


for secret in $REQUIRED_SECRETS
do
    secret_path="$SECRETS_DIR/$secret"

    if [ ! -s "$secret_path" ]; then
        echo "Secret is missing or empty: $secret" >&2
        exit 1
    fi
done


chmod 700 "$SECRETS_DIR"

for secret in $REQUIRED_SECRETS
do
    chmod 600 "$SECRETS_DIR/$secret"
done


compose() {
    docker compose \
        -f docker-compose.yml \
        -f docker-compose.prod.yml \
        "$@"
}


echo "Validating Docker Compose configuration..."

compose config >/dev/null


echo "Building application images..."

compose build \
    --pull \
    auth-app \
    library-app \
    gateway-app \
    enrichment-app \
    telegram-app


echo "Starting databases and Kafka..."

compose up \
    -d \
    --wait \
    library-postgres \
    auth-postgres \
    kafka


echo "Running Library migrations..."

compose run \
    --rm \
    library-migrations


echo "Running Auth migrations..."

compose run \
    --rm \
    auth-migrations


echo "Ensuring Kafka topics exist..."

compose run \
    --rm \
    kafka-init


echo "Starting MovieTracker..."

compose up \
    -d \
    --remove-orphans


echo "Current containers:"

compose ps


echo "Deployment completed."

echo "URL: https://$MOVIETRACKER_DOMAIN"