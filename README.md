## Run project:

docker compose up --build

## Stop project:

docker compose down

## Swagger:

http://localhost:8081/swagger/

## Reset database:

docker compose down -v

# MovieBase

MovieBase is a backend project for managing a personal movie library.

The project is built as a small microservice-based system with two Go services:

## Tech Stack

- Go
- net/http
- PostgreSQL
- pgx
- Docker Compose
- golang-migrate
- JWT
- bcrypt
- Swagger / OpenAPI

## Services

### Auth Service

Auth Service is responsible for user registration, login, logout and access token generation.

Swagger:

```text
http://localhost:8082/swagger/index.html
```

Endpoints:

```text
POST /auth/register
POST /auth/login
POST /auth/logout
GET  /health
GET  /ready
```

### Library Service

Library Service is responsible for managing the authenticated user's movie list.

Swagger:

```text
http://localhost:8081/swagger/index.html
```

Endpoints:

```text
POST   /movies
GET    /movies
GET    /movies/random
GET    /movies/{id}
DELETE /movies/{id}
PATCH  /movies/{id}/watched
PATCH  /movies/{id}/unwatched
GET    /health
GET    /ready
```

All `/movies` endpoints require authorization.

## Run

From the project root:

```bash
docker compose up --build -d
```

Check containers:

```bash
docker compose ps
```

Stop containers:

```bash
docker compose down
```

Stop containers and remove database volumes:

```bash
docker compose down -v
```

## Swagger

Auth Swagger:

To use protected Library endpoints in Swagger:

1. Open Auth Swagger.
2. Call `POST /auth/register`.
3. Call `POST /auth/login`.
4. Copy `access_token` from the login response.
5. Open Library Swagger.
6. Click `Authorize`.
7. Insert the token in this format:
