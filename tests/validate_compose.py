#!/usr/bin/env python3
"""Check network/security invariants without starting the deployment."""
import json
import os
import subprocess


def config(overlay):
    env = dict(os.environ, MOVIETRACKER_DOMAIN="example.test", MOVIETRACKER_INGRESS_CIDR="172.30.253.0/29")
    raw = subprocess.check_output(["docker", "compose", "-f", "docker-compose.yml", "-f", overlay, "config", "--format", "json"], env=env, text=True)
    return json.loads(raw)


prod = config("docker-compose.prod.yml")
services = prod["services"]
assert {name for name, service in services.items() if service.get("ports")} == {"caddy"}, "production exposes a backend port"
assert {(str(port["published"]), port.get("protocol", "tcp")) for port in services["caddy"]["ports"]} == {("80", "tcp"), ("443", "tcp"), ("443", "udp")}
assert {name for name, service in services.items() if "ingress" in service.get("networks", {})} == {"caddy", "gateway-app"}, "trusted ingress includes an unexpected peer"
assert services["gateway-app"]["environment"]["GATEWAY_TRUSTED_PROXIES"] == "172.30.253.0/29"
assert services["auth-app"]["environment"]["AUTH_COOKIE_SECURE"] == "true"
assert services["auth-app"]["environment"]["AUTH_PUBLIC_ORIGIN"] == "https://example.test"
for name in ("library-app", "enrichment-app"):
    assert services[name]["depends_on"]["kafka-init"]["condition"] == "service_completed_successfully"
for name in ("auth-app", "library-app", "gateway-app"):
    assert "/ready" in " ".join(services[name]["healthcheck"]["test"])
assert all(not volume.get("external", False) for volume in prod["volumes"].values()), "unexpected external volume"

dev = config("docker-compose.dev.yml")
for name, service in dev["services"].items():
    assert all(port.get("host_ip") == "127.0.0.1" for port in service.get("ports", [])), "non-loopback dev port: " + name
assert "EXTERNAL://localhost:9092" in dev["services"]["kafka"]["environment"]["KAFKA_ADVERTISED_LISTENERS"]
assert not dev["services"]["gateway-app"]["environment"].get("GATEWAY_TRUSTED_PROXIES"), "dev unexpectedly trusts forwarded headers"
print("PASS: Compose port exposure, proxy trust, cookies, readiness and topic initialization")
