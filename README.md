# MovieTracker

[![CI](https://github.com/GrimVeqsum/MovieTracker/actions/workflows/ci.yml/badge.svg)](https://github.com/GrimVeqsum/MovieTracker/actions/workflows/ci.yml)

MovieTracker — сервис для ведения личной библиотеки фильмов.

Пользователь может добавлять фильмы, отмечать их просмотренными, ставить оценки, оставлять отзывы и получать случайный фильм из списка непросмотренных. После добавления информация о фильме дополняется асинхронно. Библиотекой также можно управлять через Telegram-бота.

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
                              API метаданных фильмов

Telegram API ◄────► Telegram Service ───► Auth / Library
```

### Сервисы

- **Gateway** — единая публичная точка входа, маршрутизация запросов и ограничение частоты запросов.
- **Auth** — регистрация, вход, JWT-токены, refresh-сессии и привязка Telegram.
- **Library** — хранение фильмов пользователя, статусов просмотра, оценок, отзывов и метаданных.
- **Enrichment** — обработка событий из Kafka и получение данных о фильмах из внешнего API.
- **Telegram** — управление библиотекой через Telegram Bot API.

Auth и Library используют отдельные базы PostgreSQL.

При создании фильма Library сохраняет событие в transactional outbox. Событие публикуется в Kafka и обрабатывается Enrichment-сервисом. Необработанные после повторных попыток сообщения отправляются в DLQ.

## Стек

- Go 1.25
- `net/http`
- PostgreSQL
- pgx
- Apache Kafka
- Docker
- Docker Compose
- golang-migrate
- JWT
- bcrypt
- Caddy
- Telegram Bot API
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

Каждый файл содержит только значение соответствующего секрета.

Каталог `.secrets/` исключён из Git.

### Запуск

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.dev.yml \
  up -d --build
```

Проверить состояние контейнеров:

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.dev.yml \
  ps
```

Gateway будет доступен по адресу:

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

## API

Основное API доступно через Gateway:

```text
/auth/*
/movies
/movies/*
```

Основные возможности:

- регистрация и вход;
- обновление access-токена;
- выход из аккаунта;
- привязка Telegram;
- добавление и удаление фильмов;
- получение списка фильмов;
- получение фильма по идентификатору;
- выбор случайного непросмотренного фильма;
- отметка фильма просмотренным или непросмотренным;
- оценка и отзыв.

Маршруты `/movies` требуют авторизации:

```text
Authorization: Bearer <access_token>
```

Служебные маршруты Auth и Library используются только для взаимодействия между сервисами и не публикуются через Gateway.

## Локальная разработка

В конфигурации для разработки наружу дополнительно открыты:

```text
Gateway   http://localhost:8080
Library   http://localhost:8081
Auth      http://localhost:8082
```

Swagger:

```text
http://localhost:8081/swagger/index.html
http://localhost:8082/swagger/index.html
```

Основной `docker-compose.yml` не публикует внутренние сервисы наружу. Дополнительные порты подключаются через `docker-compose.dev.yml`.

## Авторизация

После входа Auth выдаёт короткоживущий JWT access token.

Refresh token хранится в cookie и используется для получения нового access token. При обновлении refresh token заменяется новым, а при выходе соответствующая сессия отзывается.

Library проверяет JWT самостоятельно и использует идентификатор пользователя из токена для доступа только к его фильмам.

## Обработка событий

Создание фильма не требует ожидания внешнего API.

Library записывает фильм и событие в одной транзакции PostgreSQL:

```text
Library
   │
   ├── movies
   │
   └── outbox
          │
          ▼
        Kafka
          │
          ▼
     Enrichment
          │
          ▼
  API метаданных
          │
          ▼
       Library
```

Это позволяет сохранить событие даже при временной недоступности Kafka.

Повторная обработка одного события не приводит к повторному изменению уже обработанных метаданных.

## Миграции

Миграции Auth и Library выполняются автоматически перед запуском приложений.

Используется `golang-migrate`.

Миграции находятся в каталогах:

```text
services/auth/migrations
services/library/migrations
```

## Рабочее окружение

Для рабочего запуска используется:

```text
docker-compose.yml
docker-compose.prod.yml
```

Необходимо задать домен:

```bash
export MOVIETRACKER_DOMAIN=example.com
```

Запуск:

```bash
docker compose \
  -f docker-compose.yml \
  -f docker-compose.prod.yml \
  up -d --build
```

Caddy принимает HTTP/HTTPS-запросы и передаёт их в Gateway.

Наружу публикуются только:

```text
80
443
```

PostgreSQL, Kafka, Auth, Library, Enrichment и Telegram-сервис остаются внутри сети Docker.

## Резервное копирование

Скрипт резервного копирования:

```text
deploy/backup.sh
```

Он создаёт архивы обеих PostgreSQL-баз через `pg_dump`.

Резервные копии сохраняются в:

```text
backups/
```

По умолчанию файлы старше 7 дней удаляются.

Каталог `backups/` исключён из Git.

## CI

GitHub Actions запускается при:

- push в `main`;
- pull request в `main`.

Каждый Go-сервис проверяется отдельно:

```text
auth
library
gateway
enrichment
telegram
```

Для каждого выполняются:

```text
go mod download
go mod verify
go vet ./...
go test ./...
go build ./...
```

## Проверка работы

После запуска Gateway доступен по адресу:

````text
http://localhost:8080

## Назначение секретов:

```text
auth_db_password                    пароль базы Auth
library_db_password                 пароль базы Library
jwt_secret                          ключ подписи JWT
telegram_bot_token                  токен Telegram-бота
telegram_auth_service_token         ключ Telegram Service → Auth
movie_api_key                       ключ API метаданных фильмов
enrichment_library_service_token    ключ Enrichment → Library
````
