# MovieTracker

[![CI](https://github.com/GrimVeqsum/MovieTracker/actions/workflows/ci.yml/badge.svg)](https://github.com/GrimVeqsum/MovieTracker/actions/workflows/ci.yml)

MovieTracker — сервис для ведения личной библиотеки фильмов.

Можно добавлять фильмы, отмечать их просмотренными, ставить оценки, оставлять отзывы и выбирать случайный фильм из непросмотренных. Метаданные фильма загружаются асинхронно после создания. Библиотекой также можно управлять через Telegram-бота.

## Архитектура

```text
                         ┌──────────────┐
Интернет ───────────────►│    Caddy     │
                         └──────┬───────┘
                                │
                         ┌──────▼───────┐
                         │   Gateway    │
                         └──────┬───────┘
                            ┌───┴───┐
                            │       │
                     ┌──────▼──┐ ┌──▼─────────┐
                     │  Auth   │ │  Library   │
                     └────┬────┘ └─────┬──────┘
                          │            │
                     PostgreSQL   PostgreSQL
                                       │
                                     Outbox
                                       │
                                       ▼
                                     Kafka
                                       │
                              ┌────────▼─────────┐
                              │   Enrichment     │
                              └────────┬─────────┘
                                       │
                                  PoiskKino API

Telegram API ◄────► Telegram Service ───► Auth / Library
```

### Сервисы

- **Gateway** — публичная точка входа и ограничение частоты запросов.
- **Auth** — регистрация, вход, JWT, refresh-сессии и привязка Telegram.
- **Library** — фильмы, статусы просмотра, оценки, отзывы и метаданные.
- **Enrichment** — обработка событий Kafka и получение метаданных фильмов.
- **Telegram** — управление библиотекой через Telegram Bot API.

Auth и Library используют отдельные PostgreSQL-базы.

События создания фильмов записываются через transactional outbox и публикуются в Kafka. После обработки Enrichment обновляет данные фильма в Library. Для сообщений, которые не удалось обработать после повторных попыток, используется DLQ.

## Стек

- Go 1.25
- `net/http`
- PostgreSQL
- pgx
- Apache Kafka
- Docker / Docker Compose
- golang-migrate
- JWT / bcrypt
- Caddy
- Telegram Bot API
- PoiskKino API
- Swagger / OpenAPI
- GitHub Actions

## Быстрый запуск

### Требования

- Docker
- Docker Compose

### Секреты

Создайте каталог:

```text
.secrets/
```

В нём должны находиться:

```text
.secrets/
├── auth_db_password
├── library_db_password
├── jwt_secret
├── telegram_bot_token
├── telegram_auth_service_token
├── movie_api_key
└── enrichment_library_service_token
```

Назначение файлов:

```text
auth_db_password                    пароль базы Auth
library_db_password                 пароль базы Library
jwt_secret                          ключ подписи JWT
telegram_bot_token                  токен Telegram-бота
telegram_auth_service_token         ключ Telegram Service → Auth
movie_api_key                       ключ PoiskKino API
enrichment_library_service_token    ключ Enrichment → Library
```

Каждый файл должен содержать только значение секрета.

Каталог `.secrets/` исключён из Git.

### Запуск

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.dev.yml \
  up -d --build
```

Проверить контейнеры:

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.dev.yml \
  ps
```

Gateway:

```text
http://localhost:8080
```

Проверка готовности:

```text
http://localhost:8080/ready
```

Остановка:

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.dev.yml \
  down
```

## Проверка работы

Зарегистрировать пользователя:

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'
```

Войти:

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'
```

В ответе будет `access_token`.

Добавить фильм:

```bash
curl -X POST http://localhost:8080/movies \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"title":"Inception","release_year":2010}'
```

Получить библиотеку:

```bash
curl http://localhost:8080/movies \
  -H "Authorization: Bearer <access_token>"
```

Получить случайный непросмотренный фильм:

```bash
curl http://localhost:8080/movies/random \
  -H "Authorization: Bearer <access_token>"
```

Метаданные добавленного фильма загружаются асинхронно, поэтому могут появиться через некоторое время после создания записи.

## API

Основные маршруты доступны через Gateway:

```text
/auth/*
/movies
/movies/*
```

Маршруты `/movies` требуют:

```text
Authorization: Bearer <access_token>
```

Служебные маршруты Auth и Library используются только между сервисами и через Gateway не публикуются.

В режиме разработки Swagger доступен по адресам:

```text
http://localhost:8081/swagger/index.html
http://localhost:8082/swagger/index.html
```

## Telegram

После привязки аккаунта библиотекой можно управлять через Telegram.

Основные команды:

```text
/add НАЗВАНИЕ
/movies
/random
/watched N RATING
/unwatched N
/delete N
/link CODE
/help
```

Код привязки создаётся через MovieTracker и передаётся боту командой:

```text
/link CODE
```

## Авторизация

После входа Auth выдаёт короткоживущий JWT access token.

Refresh token хранится в HTTP-only cookie. При обновлении сессии refresh token заменяется новым. При выходе соответствующая refresh-сессия отзывается.

Library самостоятельно проверяет JWT и получает идентификатор пользователя из токена.

## Миграции

Auth и Library имеют независимые миграции:

```text
services/auth/migrations
services/library/migrations
```

Используется `golang-migrate`.

При развёртывании миграции выполняются перед запуском новой версии приложений.

## Развёртывание

Для рабочего окружения используются:

```text
docker-compose.yml
docker-compose.prod.yml
```

Необходимо задать домен:

```bash
export MOVIETRACKER_DOMAIN=example.com
```

Развёртывание:

```bash
./deploy/deploy.sh
```

Caddy принимает внешние HTTP/HTTPS-запросы и передаёт их в Gateway.

Снаружи публикуются только:

```text
80
443
```

PostgreSQL, Kafka и внутренние сервисы остаются внутри Docker-сети.

## Резервное копирование

Скрипт:

```text
deploy/backup.sh
```

создаёт резервные копии Auth и Library через `pg_dump`.

По умолчанию файлы сохраняются в:

```text
backups/
```

и удаляются через 7 дней.

Каталог `backups/` исключён из Git.

Для рабочего окружения рекомендуется дополнительно хранить копии резервных файлов вне сервера.

## CI

GitHub Actions запускается при push и pull request в `main`.

Для каждого Go-сервиса выполняются:

```text
go mod download
go mod verify
go vet ./...
go test ./...
go build ./...
```

Дополнительно проверяются Docker Compose, Dockerfile, Caddyfile и shell-скрипты развёртывания.
