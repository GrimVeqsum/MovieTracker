FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/auth

FROM alpine:latest

WORKDIR /app

COPY --from=builder /server /app/server

EXPOSE 8082

CMD ["/app/server"]