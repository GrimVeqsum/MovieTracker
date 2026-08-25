FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 \
  GOOS=linux \
  go build \
  -trimpath \
  -ldflags="-s -w" \
  -o /server \
  ./cmd/enrichment

RUN CGO_ENABLED=0 \
  GOOS=linux \
  go build \
  -trimpath \
  -ldflags="-s -w" \
  -o /dlq-replay \
  ./cmd/dlq-replay


FROM alpine:3.23

RUN apk add --no-cache \
  ca-certificates \
  && addgroup \
  -S \
  appgroup \
  && adduser \
  -S \
  -D \
  -H \
  -u 10001 \
  -G appgroup \
  appuser

WORKDIR /app

COPY \
  --from=builder \
  --chown=appuser:appgroup \
  /server \
  /app/server

COPY \
  --from=builder \
  --chown=appuser:appgroup \
  /dlq-replay \
  /app/dlq-replay

USER appuser:appgroup

ENTRYPOINT ["/app/server"]