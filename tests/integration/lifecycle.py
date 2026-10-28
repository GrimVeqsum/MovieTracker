#!/usr/bin/env python3
"""Exercise acknowledged HTTP writes across real process and dependency restarts."""
import http.cookiejar
import json
import os
import pathlib
import subprocess
import time
import urllib.error
import urllib.request
import uuid

ROOT = pathlib.Path(__file__).resolve().parents[2]
PROJECT = os.environ["COMPOSE_PROJECT_NAME"]
assert PROJECT.startswith("movietracker-it-"), "isolated runner project required"
COMPOSE = ["docker", "compose", "-p", PROJECT, "-f", str(ROOT / "tests/integration/compose.yml")]


def compose(*args, input_data=None):
    return subprocess.check_output(COMPOSE + list(args), text=True, input=input_data, timeout=120).strip()


address = compose("port", "gateway", "8080")
base_url = "http://" + address
jar = http.cookiejar.CookieJar()
client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))


def request(path, method="GET", body=None, token=None, expected=200):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(base_url + path, data=json.dumps(body).encode() if body is not None else None, headers=headers, method=method)
    with client.open(req, timeout=10) as response:
        assert response.status == expected, (path, response.status)
        raw = response.read()
        return json.loads(raw) if raw else None


def ready():
    deadline = time.monotonic() + 60
    while time.monotonic() < deadline:
        try:
            request("/ready")
            return
        except (urllib.error.URLError, TimeoutError, ConnectionError):
            time.sleep(0.2)
    raise AssertionError("services did not recover")


email = "lifecycle-" + uuid.uuid4().hex + "@example.test"
credentials = {"email": email, "password": "integration-only-password123"}
request("/auth/register", "POST", credentials, expected=201)
access = request("/auth/login", "POST", credentials)["access_token"]
# Force a real broker outage after readiness; HTTP writes must remain durable.
compose("stop", "kafka")
movie = request("/movies", "POST", {"title": "Lifecycle Test", "release_year": 2010}, access, expected=201)
assert movie["id"]
# Stop the real processes while delivery is unavailable. Only HTTP-acknowledged
# data is asserted, since a disconnect before an HTTP response is ambiguous.
compose("stop", "gateway", "auth", "library", "enrichment")
# The random test project owns disposable persistent volumes, allowing real
# dependency restarts without touching any pre-existing user volumes.
compose("restart", "postgres", "kafka")
compose("up", "-d", "--wait", "--wait-timeout", "90", "auth", "library", "gateway", "enrichment")
# Docker may allocate a new ephemeral published port when starting containers.
base_url = "http://" + compose("port", "gateway", "8080")
ready()
access = request("/auth/refresh", "POST")["access_token"]
movies = request("/movies", token=access)
assert sum(m["id"] == movie["id"] for m in movies) == 1, "acknowledged movie lost or duplicated"
# Verify both committed DB intent and eventual Kafka delivery after recovery.
query = "SELECT count(*) FROM outbox_events WHERE aggregate_id = '%s' AND published_at IS NOT NULL;" % movie["id"]
deadline = time.monotonic() + 30
while time.monotonic() < deadline:
    count = compose("exec", "-T", "postgres", "psql", "-U", "integration", "-d", "library_test", "-Atc", query)
    if count == "1":
        break
    time.sleep(0.2)
else:
    diagnostic = compose("exec", "-T", "postgres", "psql", "-U", "integration", "-d", "library_test", "-Atc", "SELECT attempts, last_error FROM outbox_events WHERE published_at IS NULL;")
    raise AssertionError("outbox event did not publish after Kafka recovery: " + diagnostic)
# A real Kafka record must be readable, not just a published_at timestamp.
record = compose("exec", "-T", "kafka", "/opt/kafka/bin/kafka-console-consumer.sh", "--bootstrap-server", "kafka:19092", "--topic", "lifecycle.events", "--from-beginning", "--max-messages", "1", "--timeout-ms", "10000")
assert json.loads(record)["movie_id"] == movie["id"], "wrong or missing Kafka event"
event = json.loads(record)
# Cross-service duplicate delivery: real Kafka -> real Enrichment -> real
# Library -> real PostgreSQL inbox. The local fixture replaces only PoiskKino.
def movie_state():
    return request("/movies/" + movie["id"], token=access)

# Broker restart can retain the old group member until session timeout.
# HTTP readiness does not imply that a consumer has received its assignment.
deadline = time.monotonic() + 90
while time.monotonic() < deadline:
    enriched = movie_state()
    if enriched["metadata_status"] == "ready":
        break
    time.sleep(0.2)
else:
    raise AssertionError("real enrichment did not apply metadata")
assert enriched["external_id"] == "7" and len(enriched["genres"]) == 1
compose("exec", "-T", "kafka", "/opt/kafka/bin/kafka-console-producer.sh", "--bootstrap-server", "kafka:19092", "--topic", "lifecycle.events", input_data=json.dumps(event) + "\n")
# Wait until the real consumer commits both original and duplicate records.
deadline = time.monotonic() + 30
while time.monotonic() < deadline:
    offsets = compose("exec", "-T", "kafka", "/opt/kafka/bin/kafka-consumer-groups.sh", "--bootstrap-server", "kafka:19092", "--describe", "--group", "lifecycle-enrichment")
    rows = [line.split() for line in offsets.splitlines() if line.startswith("lifecycle-enrichment ")]
    if rows and all(row[5] == "0" for row in rows) and sum(int(row[3]) for row in rows) >= 2:
        break
    time.sleep(0.2)
else:
    raise AssertionError("duplicate event was not consumed and committed")
assert movie_state()["updated_at"] == enriched["updated_at"], "duplicate metadata changed movie again"
inbox_query = "SELECT count(*) FROM processed_events WHERE consumer = 'enrichment' AND event_id = '%s';" % event["event_id"]
assert compose("exec", "-T", "postgres", "psql", "-U", "integration", "-d", "library_test", "-Atc", inbox_query) == "1"
print("PASS: acknowledged movie/session survive service and DB restarts; Kafka outage recovers; real Kafka/Enrichment/Library/PostgreSQL deduplicate metadata")
