# MovieBase

MovieBase — backend-проект для управления личной библиотекой фильмов.

Проект построен как небольшая микросервисная система на Go:

- Gateway Service — единая точка входа в API
- Auth Service — регистрация, вход, выход и выдача JWT
- Library Service — управление списком фильмов авторизованного пользователя

## Стек

- Go
- net/http
- PostgreSQL
- pgx
- Docker Compose
- golang-migrate
- JWT
- bcrypt
- Swagger / OpenAPI

## Сервисы

### Gateway Service

Gateway Service — основная точка входа в проект.

Base URL:

```text
http://localhost:8080
```

Маршрутизация:

```text
/auth/*   -> Auth Service
/movies*  -> Library Service
```

Основные endpoints через Gateway:

```text
POST   /auth/register
POST   /auth/login
POST   /auth/logout

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

### Auth Service

Auth Service отвечает за регистрацию пользователей, вход, выход и выдачу access token.

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

Library Service отвечает за управление списком фильмов авторизованного пользователя.

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

Все `/movies` endpoints требуют авторизацию.

## Запуск проекта

```bash
docker compose up --build
```

## Остановка проекта

```bash
docker compose down
```

## Проверка контейнеров

```bash
docker compose ps
```

## Сброс базы данных

```bash
docker compose down -v
```

## Swagger

Auth Swagger:

```text
http://localhost:8082/swagger/index.html
```

Library Swagger:

```text
http://localhost:8081/swagger/index.html
```

Чтобы использовать защищённые endpoints Library Service через Swagger:

1. Открыть Auth Swagger.
2. Вызвать `POST /auth/register`.
3. Вызвать `POST /auth/login`.
4. Скопировать `access_token` из ответа.
5. Открыть Library Swagger.
6. Нажать `Authorize`.
7. Вставить токен в формате:

```text
Bearer <access_token>
```

## Пример использования через Gateway

Регистрация:

```http
POST http://localhost:8080/auth/register
Content-Type: application/json

{
  "email": "test@example.com",
  "password": "password123"
}
```

Вход:

```http
POST http://localhost:8080/auth/login
Content-Type: application/json

{
  "email": "test@example.com",
  "password": "password123"
}
```

После входа в ответе приходит `access_token`.

Для запросов к фильмам нужно передавать токен:

```text
Authorization: Bearer <access_token>
```

Создать фильм:

```http
POST http://localhost:8080/movies
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "title": "Inception",
  "release_year": 2010
}
```

Получить список фильмов:

```http
GET http://localhost:8080/movies
Authorization: Bearer <access_token>
```

Получить случайный фильм:

```http
GET http://localhost:8080/movies/random
Authorization: Bearer <access_token>
```

Отметить фильм просмотренным:

```http
PATCH http://localhost:8080/movies/{id}/watched
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "rating": 9,
  "review": "Great movie"
}
```

Отметить фильм непросмотренным:

```http
PATCH http://localhost:8080/movies/{id}/unwatched
Authorization: Bearer <access_token>
```

Удалить фильм:

```http
DELETE http://localhost:8080/movies/{id}
Authorization: Bearer <access_token>
```

Выйти:

```http
POST http://localhost:8080/auth/logout
Authorization: Bearer <access_token>
```

## Авторизация

В проекте используется JWT-авторизация.

После успешного входа Auth Service создаёт JWT access token.

Library Service защищает все `/movies` endpoints и проверяет токен из заголовка:

```text
Authorization: Bearer <access_token>
```

ID пользователя хранится внутри JWT и используется в Library Service, чтобы пользователь мог работать только со своими фильмами.

## Особенности

- Gateway Service используется как единая точка входа.
- Auth Service и Library Service используют отдельные базы PostgreSQL.
- Миграции применяются автоматически через Docker Compose.
- Swagger-документация доступна для Auth Service и Library Service.
- Logout реализован как client-side token invalidation.
