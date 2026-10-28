# Инженерный аудит MovieTracker

Дата: 28 октября 2026. Область: пять Go-модулей, все SQL-миграции, Dockerfiles, три Compose-файла, Caddy, deploy/backup, GitHub Actions и README. Исходное рабочее дерево было чистым. Архитектура и публичные API сохранены; добавлена совместимая миграция Auth `000004`.

Приоритеты: **CRITICAL** — непосредственный массовый компромисс/необратимая потеря; **HIGH** — воспроизводимый обход защиты, потеря сообщения или нарушение целостности; **MEDIUM** — ограниченная уязвимость, надёжность и эксплуатационные риски; **LOW** — защитное улучшение/качество. Наличие механизма само по себе не считалось подтверждением гарантии.

Подтверждённых CRITICAL не найдено. Все перечисленные ниже подтверждённые HIGH исправлены. Это результат анализа и конкретных проверок, а не доказательство отсутствия любых уязвимостей.

## HIGH: подтверждённые дефекты

| ID | Исходная проблема и сценарий | Исправление и проверка |
| --- | --- | --- |
| G-01 | `gateway/.../rate_limit.go:clientIP` безусловно брал первый XFF. Один клиент менял заголовок и получал новый bucket для каждого login/register/refresh. | `forwarding.go`: по умолчанию доверенных прокси нет; явные CIDR, проверка непосредственного peer, проход по цепочке справа налево до первого недоверенного адреса. Malformed chain → peer. Unit-тест подмены и интеграционный тест настоящего Caddy с клиентами `127.0.0.2`/`.3`. |
| A-01 | Logout искал только текущий hash. Refresh успевал заменить его, после чего logout со старой cookie отвечал 204, оставляя новую сессию действительной. | Неизменяемая связь hash → session в `refresh_session_tokens`, атомарная запись при login/rotation, отзыв по стабильному session ID. PostgreSQL-тесты обеих очередностей и конкурентных гонок. |
| A-02 | Однократный backfill новой таблицы сам по себе не покрывает записи старого Auth между миграцией и рестартом. Это обнаружено независимым review исправления A-01. | Logout сначала разрешает стабильный ID через историю **или текущий hash**, затем отзывает по ID в транзакции. Rotation сохраняет также старый hash в той же транзакции. Проверены legacy sessions без истории, rotation и 20 конкурентных logout/refresh. Один `UPDATE ... OR current_hash` не использован: PostgreSQL EPQ и snapshot могли снова пропустить строку после concurrent rotation. |
| L-01 | Metadata безусловно заменяла `release_year`: неверное совпадение API меняло пользовательские данные; NULL → год мог конфликтовать с active unique index и постоянно откатывать событие. | Явный год пользователя сохраняется. Отсутствующий год заполняется; при конкретном `23505` identity index вся транзакция повторяется без изменения года. Metadata/genres/inbox остаются атомарны. Два PostgreSQL regression tests. |
| K-01 | `Consumer.Run` отбрасывал весь fetch при ошибке любой partition, включая полученные records другой partition. Poll уже двигал cursor; последующий commit более высокого offset мог навсегда пропустить record. | Полученные records обрабатываются независимо от fetch errors. Неуспех разрешения record останавливает consumer, не позволяя подтвердить более высокий offset. Unit regression содержит одновременно валидную запись и ошибку другой partition. |
| K-02 | `dlq-replay --event-id` commit-ил unmatched records в общей replay group. Обычный последующий replay уже не видел пропущенные события. | Фильтр выполняет независимое чтение DLQ с начала без group и commit. Обычный replay имеет durable offsets; commit только после Kafka ACK публикации оригинала. Проверено реальной Kafka: выборочный replay второго события, затем обычный replay обоих. |
| T-01 | Ошибки `http.Client.Do` в Telegram включали URL `/bot<TOKEN>/...`; логирование транспортной ошибки могло раскрыть bot token. | Клиент снимает оболочку `url.Error`, сохраняя underlying error и cancellation, без URL. Тест с фиктивным секретом проверяет отсутствие token в ошибке. |
| I-01 | `deploy.sh` выставлял секретам `0600`, тогда как приложения запускаются под UID 10001. File secrets Compose — bind mounts; исходный owner/mode сохраняется. После deploy приложения могли не прочитать секреты. | Каталог `0700`, файлы `0644`: доступ на хосте ограничен родительским каталогом, контейнер читает только явно выданные ему файлы. Модель и компромисс описаны в README. Никакие реальные секреты в ходе аудита не читались и не менялись. |
| I-02 | Dev Compose публиковал Kafka, БД и внутренние API на всех интерфейсах. Kafka PLAINTEXT без ACL позволяла любому доступному сетевому клиенту изменять/удалять topics и сообщения. | Все dev mappings привязаны к `127.0.0.1`. Production по-прежнему публикует только Caddy 80/443. Для Kafka dev добавлен отдельный external listener `localhost:9092`, внутренний `kafka:19092` сохранён. |

## MEDIUM: исправлено

| ID | Проблема | Результат |
| --- | --- | --- |
| G-02 | Proxy принимал произвольный `X-Forwarded-Proto`; общая Docker-сеть не отделяла доверенный Caddy от backend peers. | Rewrite API формирует XFF/XFP/XFH заново; `Forwarded`/`X-Real-IP` удаляются; host берётся из фактического Host, XFP ограничен http/https и доверенным peer. Только Caddy и Gateway входят в production ingress, CIDR настраивается через `MOVIETRACKER_INGRESS_CIDR`. Dev доверие выключено. |
| G-03 | In-memory buckets не имели верхнего предела; большой поток разных IP увеличивал память. | До 10 000 buckets на limiter, дальнейшие новые адреса получают 429 до очистки. Существующие клиенты продолжают использовать свои лимиты. |
| G-04 | Не было общего ограничения forwarded request body и срока ожидания upstream headers. | Gateway ограничивает body 1 MiB, response headers — 10 s. Существующие header/read/write/idle timeouts сохранены. Auth и внутренние metadata handlers дополнительно ограничивают JSON. |
| A-03 | SameSite=Lax не изолирует соседние origins одного site. Cookie login/refresh/logout не проверяли Origin. | Точный `AUTH_PUBLIC_ORIGIN`, Fetch Metadata guard; production HTTPS и dev localhost настроены в Compose. Прямой Auth при отсутствии настройки использует реальный scheme/Host и не доверяет forwarded headers. Curl/Telegram без browser headers поддержаны. |
| A-04 | Поздний 401 проигравшего concurrent refresh очищал cookie успешного refresh. | Invalid refresh больше не отправляет Set-Cookie; явный logout очищает cookie. Проверено реальным PostgreSQL и HTTP handler. |
| L-02 | Batch из 20 последовательных Kafka отправок по 10 s мог пережить claim lease 1 min. Другой worker повторно захватывал ещё обрабатываемые записи. | Claim одной записи непосредственно перед публикацией; цикл до 20 сохраняется. Duplicate после ACK-before-mark остаётся штатным at-least-once сценарием. |
| L-03 | Shutdown не ожидал outbox worker; canceled context мешал записать уже полученный Kafka ACK, DB/producer закрывались конкурентно. | Отдельный bounded context для MarkPublished/MarkFailed и join worker до закрытия зависимостей. HTTP drain, затем принудительный Close при deadline. Тест отменяет worker сразу после реального Kafka ACK; `published_at` сохраняется. |
| K-03 | Rebalance мог сменить владельца во время обработки/commit; бесконечные DLQ/commit retries удерживали работу без верхней границы. | `BlockRebalanceOnPoll`, разрешение rebalance после record, 3 min record budget, 4 min rebalance timeout. Неисчерпанная запись не подтверждается; ошибка завершает процесс с exit 1, Compose перезапускает его. CloseAllowingRebalance снимает gate. |
| K-04 | HTTP-клиенты следовали redirects с чувствительными custom headers; error bodies внешнего сервера попадали в диагностику/DLQ; ответы не везде ограничивались. | PoiskKino/Library/Telegram/backend clients не следуют redirects. PoiskKino ≤2 MiB и полная JSON-валидация; Telegram ≤1 MiB и HTTP status check. Library error bodies не копируются в log/DLQ. Локальные HTTP-серверы проверяют 429/5xx, malformed/oversized response, cancellation и redirect. |
| P-01 | Ошибка pgx ParseConfig могла содержать malformed DSN с паролем. | В Auth/Library — постоянная диагностическая ошибка без DSN; regression tests. |
| S-01 | Config/listen failures могли выглядеть как exit 0, ожидать сигнала или обходить defer через Fatal из goroutine. | run() с cleanup и exit 1 для ошибок; bind error инициирует shutdown. Auth/Gateway drain до 30 s, Library до 15 s; Close при timeout. Compose даёт 45 s. |
| I-03 | Healthchecks приложений проверяли liveness, а Kafka clients не зависели от завершения topic initialization. | Healthchecks используют `/ready`; Library и Enrichment ждут `kafka-init`. Library readiness проверяет PostgreSQL, чтобы outage Kafka не блокировал запись в outbox. Compose не обещает автоматический рестарт только по unhealthy. |
| I-04 | Миграции подставляли raw DB password в URI; `@:/%?#` меняли смысл URL. | Пароль percent-encoded перед migrate. Наличие od/tr/sed и roundtrip фиктивного пароля с ASCII/UTF-8 special chars проверены в официальном migrate image. |
| V-01 | JWT subject не проверялся как UUID, внутренние metadata IDs могли доходить до PG как ошибочные типы. | UUID-проверки и negative tests. HS256, issuer/audience, exp, iat/nbf и leeway были корректны до аудита и сохранены. |

LOW исправления: одинаковый порядок блокировок genres при concurrent upsert (сортируется копия списка); no-store для служебных Telegram token/link-code; запрет второго JSON object внутренних Telegram запросов; отменяемый shutdown и явные exit codes. Внешний API URL опционально настраивается для локального fixture; default endpoint сохранён.

## Проверенные существовавшие гарантии

- **Refresh concurrency:** условный UPDATE текущего hash в транзакции PostgreSQL уже давал ровно одного победителя: блокировка строки и повторная проверка predicate после concurrent commit. Это подтверждено 16 конкурентными запросами; механизм не заменяли предположением о его некорректности.
- **JWT:** подпись только HS256, ожидаемые issuer `movietracker-auth` и audience `movietracker-api`, обязательный exp, проверки nbf/iat, leeway 30 s. Добавлены отрицательные тесты алгоритма, подписи, claims и UUID.
- **Секреты Auth:** bcrypt, случайный refresh 256 bit, хранится только SHA-256; cookie HttpOnly, host-only, Path=/auth, SameSite=Lax, production Secure. Service token comparisons — constant time. Telegram link consumption защищён транзакцией и unique constraint.
- **Изоляция Library:** публичные read/update/delete используют одновременно movie ID, user ID, deleted_at IS NULL. Чужая запись даёт NotFound; изменение не создаёт outbox event. Проверены реальные репозитории, а не mock с той же ожидаемой логикой.
- **Гонки:** UPDATE/DELETE — единичные SQL statements с scoped predicate, не схема read-check-write. Конкурентный update/delete не воскрешает soft-deleted строку.
- **Soft delete:** partial unique index с NULLS NOT DISTINCT защищает неизвестный год и исключает удалённые строки. Повторное добавление после soft delete допустимо.
- **Transactional outbox:** приложение использует TransactionalRepository для Create/Delete/UpdateStatus. Данные фильма и событие пишутся одной pgx-транзакцией. Инъекция ошибки INSERT outbox откатывает каждую мутацию. Context cancellation не допускает частичный commit.
- **Metadata inbox:** success marker `(consumer,event_id)` и movie/genres пишутся одной транзакцией. Восемь конкурентных deliveries дают один marker/effect. Failed не резервирует marker; replay того же event_id может стать ready; поздний failed не понижает ready.
- **Kafka:** outbox публикует синхронно с AllISRAcks; только ACK разрешает MarkPublished. Failed publication оставляет durable intent с backoff; lease/lock_id защищают владение. DisableAutoCommit включён. Consumer commit следует принятому результату Library либо acknowledged DLQ copy.
- **DLQ:** envelope сохраняет исходные key/value bytes через base64 и source topic/partition/offset. Replay публикует оригинал до commit; malformed envelope останавливает replay без удаления данных.
- **SQL:** проверены ошибки PG и rows.Err, ограниченные SQL contexts, уникальные индексы. Up/down/up миграции проверяются в отдельных схемах PostgreSQL; новая Auth migration дополнительно проверяет backfill и rollback при ошибке записи истории.

## Реальные гарантии доставки и окна сбоев

| Момент сбоя | Восстановление |
| --- | --- |
| До commit movie+outbox | Ни фильм, ни intent не записываются; HTTP success не возвращается. |
| После commit, Kafka недоступна | Фильм доступен; outbox остаётся pending и повторяет отправку. |
| После Kafka ACK, до published_at | Kafka может получить дубликат после истечения lease; event_id остаётся прежним. |
| После metadata commit, до consumer offset commit | Consumer получает событие повторно; Library inbox исключает повторное изменение metadata/genres. |
| Ошибка PoiskKino | Повторные попытки для временных ошибок; not found сохраняет failed как терминальный результат; исчерпанные/постоянные ошибки сохраняются в DLQ. |
| DLQ produce не подтверждён | Source offset не commit-ится. При исчерпании record budget consumer завершается; следующий запуск снова читает source. |
| Replay produce не подтверждён | DLQ offset не commit-ится. |
| Replay produce подтверждён, commit DLQ не выполнен | Повторный replay допустимо публикует дубликат с тем же event_id. |
| SIGTERM во время publish/processing | Неподтверждённый record остаётся для повторения. ACK outbox сохраняется bounded независимым context; worker завершается до закрытия БД. |

Это **at-least-once**, не exactly-once. Kafka ACK с replication factor 1 не означает защиту от потери диска. Конечная гарантия зависит от сохранности PostgreSQL/Kafka storage и retention. Никакой тест не доказывает невозможность всех сетевых/аппаратных сбоев.

## Остаточные проблемы и компромиссы

| Приоритет | Ограничение | Причина/следующий шаг |
| --- | --- | --- |
| MEDIUM | Один Kafka broker, PostgreSQL без failover, PLAINTEXT Kafka внутри сети, общие HS256/service secrets. | Сохранён текущий deployment. Для реальной эксплуатации отдельно нужны backup/restore drills, least privilege DB roles, Kafka ACL/TLS и политика ротации ключей; новые системы не вводились. |
| MEDIUM | Нет автоматического retention/purge outbox/inbox, refresh history/sessions, link codes. | Удаление inbox должно соответствовать максимальному Kafka/DLQ replay window, иначе вернутся повторные effects. Нужна явная эксплуатационная политика. |
| MEDIUM | List без пагинации, genres N+1; random загружает кандидатов. | Сохранён API. Следующая небольшая итерация — batch genres и согласованная cursor pagination. |
| MEDIUM | Нет строгого порядка outbox при retry/нескольких workers. | Текущий Enrichment обрабатывает MovieCreated. Новым consumers status history понадобится version/order contract. |
| MEDIUM | Access JWT остаётся действителен до 15 min + leeway после logout; отзывается одна refresh-сессия. | Немедленный отзыв access token требует проверки состояния. API общего logout всех устройств отсутствует. |
| MEDIUM | Telegram offsets только в памяти, нет durable command inbox. | После сбоя возможны повторные команды; при обработанной ошибке команда подтверждается следующей polling итерацией. Outbox-гарантии не распространяются на Telegram delivery. |
| MEDIUM | Нет интегрированных metrics/alerts для backlog/lag/DLQ. | Логи и SQL/CLI позволяют инспекцию, но production наблюдение потребует отдельной настройки. |
| LOW | Rate limiting только в памяти одной реплики, fixed window, общий bucket для NAT. | Рестарт сбрасывает лимиты; несколько реплик независимы. Redis не добавлялся. |
| LOW | Один точный browser origin Auth; dev Swagger на другом origin может требовать конфигурации. | Использовать canonical Gateway origin и MOVIETRACKER_PUBLIC_ORIGIN; прямые curl-запросы работают. |
| LOW | `--event-id` replay сканирует DLQ с начала без продвижения обычной очереди; malformed envelope прерывает scan. | Консервативная политика: оператор исправляет/инспектирует запись, события автоматически не удаляются. Deleted movie 404 тоже сохраняется в DLQ. |
| LOW | PoiskKino matching эвристический; внешние quotas/Retry-After не моделируются полностью. | Тесты используют локальные ответы; реальные провайдеры и ключи не вызывались. |
| LOW | Старые nontransactional Repository mutation methods остались, хотя application их не вызывает. | Удаление не требовалось; публичные service routes проверены на использование TransactionalRepository. |

## Проверка и её границы

Тестовые зависимости из `tests/integration/compose.yml` отделены от production: случайный project `movietracker-it-*`, только новые project-scoped disposable volumes (не external), временные схемы/topics, отдельные auth_test/library_test. Cleanup использует все profiles и удаляет **только созданные этим запуском тестовые ресурсы**. Existing user volumes/production credentials не использовались; prune, git commit/push/PR и изменение history не выполнялись.

Итоговые локальные результаты — **PASS**:

| Проверка | Реально выполнено |
| --- | --- |
| Форматирование | `gofmt -l .` пусто во всех пяти модулях |
| Зависимости | `go mod verify` во всех пяти модулях |
| Статический анализ | `go vet ./...` во всех пяти модулях |
| Unit/security | `go test ./...` во всех пяти модулях |
| Race detector | `go test -race ./...` во всех пяти модулях |
| Сборка Go | `go build ./...` во всех пяти модулях |
| Полный integration runner | `./tests/integration/run.sh`, exit 0: все модули с `-tags=integration -race -count=1 -timeout=5m`, настоящие PostgreSQL 18, Kafka 4.3.1 и Caddy 2.11.4 |
| Реальные процессы и storage | Контейнеры Auth/Library/Gateway/Enrichment, локальный PoiskKino fixture; Kafka stop → HTTP movie ACK → services stop → PostgreSQL/Kafka restart → refresh/movie/outbox recovery → duplicate через Kafka/Enrichment/Library → один PG inbox marker и неизменный updated_at |
| Docker build | Все пять Dockerfiles собраны; приложения lifecycle сценария реально запущены. Telegram и Enrichment также собраны через production Compose без запуска production stack |
| Compose | Dev/prod `config --quiet`, test Compose validation, `python3 tests/validate_compose.py`: опубликованные порты, ingress membership/trust, secure cookies/origin, readiness, topic init |
| Caddy | Официальный `caddy validate` для production Caddyfile — Valid configuration |
| Shell/Python | `sh -n` deploy/backup/runner; Python scripts исполнялись в успешных проверках |
| File secrets | Фиктивный `/tmp` файл, официальный Alpine, UID 10001: 0600 недоступен, 0644 читается; исходные реальные секреты не использовались |
| Миграционные URL | od/tr/sed и percent-encoding фиктивного special-character пароля проверены в migrate v4.19.1 |
| Рабочее дерево | `git diff --check` без ошибок |

Хост Go 1.26.8; Docker builds используют существующие Go 1.25 образы. Итоговый успешный runner project: `movietracker-it-tpwjrh71`; его контейнеры, сеть и два вновь созданных тестовых volumes удалены cleanup. Отдельный первоначальный audit project также остановлен и удалён.

Ранние прогоны полного runner были неуспешны и использовались для исправления самого harness: права нового Kafka volume, повторное получение динамического Gateway port после restart, cleanup всех profiles, явное создание lifecycle topics (как в production) и ожидание recovery старого consumer member. Итоговый PASS относится к повторному полному прогону после этих исправлений. При broker restart HTTP readiness не означает готовность consumer assignment; тест допускает 90 s на metadata recovery, не меняет offsets и не исключает проверку результата.

Обязательные сценарии привязаны к тестам:

| Требование | Проверка |
| --- | --- |
| XFF не обходит лимит | `TestForwardedIPCannotBypassRateLimit`, forged prefix test |
| Разные реальные клиенты за Caddy | `TestRealCaddyClientIPAndForwarding`: настоящий Caddy, два TCP source IP |
| Concurrent refresh | PostgreSQL session integration: 16 rotations, обе последовательности logout, конкурентные races и migration gap |
| Нельзя изменить чужой фильм | PostgreSQL repository integration: чужие read/update/delete и отсутствие outbox effect |
| Movie/outbox atomic | Forced outbox failure для Create/Delete/UpdateStatus, cancellation rollback |
| Kafka outage сохраняет событие | Реальная PG row + неуспех подключения, затем публикация в реальный broker; полный контейнерный outage/restart сценарий |
| Duplicate не меняет результат | PostgreSQL concurrent metadata tests, Kafka consumer redelivery, сквозной real Enrichment→Library→PG сценарий |
| Crash перед commit offset | Реальная Kafka: side effect выполнен, commit намеренно отменён, новый consumer той же group получает record |
| DLQ/replay не теряет события | Реальная Kafka: injected publish error, crash после produce/до commit, filtered→normal replay |
| Shutdown сохраняет подтверждённое | Реальный ACK worker cancellation, HTTP timeout/Close test, сохранённые movie/session после реальных service/PG/Kafka restarts |

Граница инъекций: сбой перед offset commit моделируется намеренной ошибкой broker boundary при реальном producer/consumer; Library sink в этом тесте локальный HTTP fixture, а реальная PG дедупликация дополнительно проверяется отдельно и сквозным контейнерным сценарием. Не моделировались host power loss/fsync failure, многоузловые partitions, потеря диска, нагрузка длительностью дни, Telegram/PoiskKino production API. GitHub Actions удалённо не запускался; локальные команды воспроизводят его проверки.

## Изменённые файлы

Изменено или добавлено 68 файлов; удаления пользовательских файлов нет. Полный список:

```text
.github/workflows/ci.yml
README.md
deploy/deploy.sh
docker-compose.dev.yml
docker-compose.prod.yml
docker-compose.yml
docs/AUDIT.md
services/auth/cmd/auth/main.go
services/auth/internal/config/config.go
services/auth/internal/platform/postgres/postgres.go
services/auth/internal/platform/postgres/postgres_test.go
services/auth/internal/users/csrf.go
services/auth/internal/users/handler.go
services/auth/internal/users/security_test.go
services/auth/internal/users/service.go
services/auth/internal/users/session_integration_test.go
services/auth/internal/users/session_repository.go
services/auth/internal/users/telegram_handler.go
services/auth/internal/users/telegram_repository.go
services/auth/migrations/000004_refresh_session_tokens.down.sql
services/auth/migrations/000004_refresh_session_tokens.up.sql
services/enrichment/cmd/dlq-replay/main.go
services/enrichment/cmd/dlq-replay/main_test.go
services/enrichment/cmd/dlq-replay/replay_integration_test.go
services/enrichment/cmd/enrichment/main.go
services/enrichment/internal/config/config.go
services/enrichment/internal/kafka/consumer.go
services/enrichment/internal/kafka/consumer_integration_test.go
services/enrichment/internal/kafka/consumer_test.go
services/enrichment/internal/library/client.go
services/enrichment/internal/library/client_test.go
services/enrichment/internal/poiskkino/client.go
services/enrichment/internal/poiskkino/client_test.go
services/enrichment/internal/testkafka/kafka.go
services/gateway/cmd/gateway/main.go
services/gateway/internal/config/config.go
services/gateway/internal/transport/http/caddy_integration_test.go
services/gateway/internal/transport/http/forwarding.go
services/gateway/internal/transport/http/forwarding_test.go
services/gateway/internal/transport/http/proxy.go
services/gateway/internal/transport/http/rate_limit.go
services/gateway/internal/transport/http/rate_limit_test.go
services/library/cmd/library/main.go
services/library/cmd/library/main_test.go
services/library/internal/auth/middleware.go
services/library/internal/auth/middleware_test.go
services/library/internal/movies/metadata_repository.go
services/library/internal/movies/repository_integration_test.go
services/library/internal/outbox/kafka_integration_test.go
services/library/internal/outbox/worker.go
services/library/internal/outbox/worker_test.go
services/library/internal/platform/postgres/postgres.go
services/library/internal/platform/postgres/postgres_test.go
services/library/internal/testutil/postgres_integration.go
services/library/internal/testutil/postgres_integration_test.go
services/library/internal/transport/http/handler.go
services/telegram/cmd/telegram/main.go
services/telegram/internal/backend/client.go
services/telegram/internal/backend/client_test.go
services/telegram/internal/telegram/client.go
services/telegram/internal/telegram/client_test.go
tests/integration/Caddyfile
tests/integration/compose.yml
tests/integration/databases.sql
tests/integration/lifecycle.py
tests/integration/movie_api.py
tests/integration/run.sh
tests/validate_compose.py
```
