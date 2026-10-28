#!/bin/sh
set -eu

PROJECT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$PROJECT_DIR"
RUN_DIR=$(mktemp -d "${TMPDIR:-/tmp}/movietracker-it-XXXXXXXX")
COMPOSE_PROJECT_NAME=$(basename "$RUN_DIR" | tr '[:upper:]' '[:lower:]')
export COMPOSE_PROJECT_NAME
TEST_KAFKA_PORT=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')
export TEST_KAFKA_PORT
compose() {
    docker compose -p "$COMPOSE_PROJECT_NAME" -f tests/integration/compose.yml "$@"
}
cleanup() {
    result=$?
    trap - EXIT INT TERM
    if [ "$result" -ne 0 ]; then compose --profile "*" logs --no-color >&2 || true; fi
    # Remove only this random project's disposable volumes; none are external.
    case "$COMPOSE_PROJECT_NAME" in movietracker-it-*) compose --profile "*" down --volumes --remove-orphans || true ;; esac
    rm -rf "$RUN_DIR"
    exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
compose config --quiet
compose up -d --wait --wait-timeout 180
PG_ADDRESS=$(compose port postgres 5432)
AUTH_TEST_DATABASE_URL="postgres://integration:integration-only-password@$PG_ADDRESS/integration?sslmode=disable"
LIBRARY_TEST_DATABASE_URL=$AUTH_TEST_DATABASE_URL
TEST_KAFKA_BROKER="127.0.0.1:$TEST_KAFKA_PORT"
TEST_KAFKA_TOPIC_PREFIX=$COMPOSE_PROJECT_NAME
export AUTH_TEST_DATABASE_URL LIBRARY_TEST_DATABASE_URL TEST_KAFKA_BROKER TEST_KAFKA_TOPIC_PREFIX
for service in auth library enrichment gateway telegram; do
    echo "Integration tests: $service"
    (cd "services/$service" && go test -tags=integration -race -count=1 -timeout=5m ./...)
done

# Match production topic initialization: Go clients deliberately do not rely
# on broker auto-creation, even when the test broker permits it.
for topic in lifecycle.events lifecycle.events.dlq; do
    compose exec -T kafka /opt/kafka/bin/kafka-topics.sh \
        --bootstrap-server kafka:19092 --create --if-not-exists \
        --topic "$topic" --partitions 1 --replication-factor 1
done

echo "Build and start real services for lifecycle test"
compose --profile apps up -d --build --wait --wait-timeout 180 auth library gateway enrichment
python3 tests/integration/lifecycle.py
