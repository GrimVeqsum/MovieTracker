#!/usr/bin/env python3
"""Local PoiskKino fixture; no real API credentials or external network calls."""
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

MOVIE = {"id": 7, "name": "Lifecycle Test", "alternativeName": "Lifecycle Test", "description": "Local integration fixture", "year": 2010, "movieLength": 120, "poster": {"url": "https://example.test/poster.jpg"}, "genres": [{"name": "drama"}]}


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        path = urlparse(self.path).path
        if path == "/health":
            status, body = 200, {"status": "ok"}
        elif self.headers.get("X-API-KEY") != "integration-only-api-key":
            status, body = 401, {}
        elif path == "/v1.4/movie/search":
            status, body = 200, {"docs": [MOVIE]}
        elif path == "/v1.4/movie/7":
            status, body = 200, MOVIE
        else:
            status, body = 404, {}
        raw = json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def log_message(self, *_):
        pass


ThreadingHTTPServer(("0.0.0.0", 8085), Handler).serve_forever()
