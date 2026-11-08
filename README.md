# Анекдот Бот

Веб-приложение с анекдотами: категории, случайный анекдот из архива, случайный
анекдот с внешнего сайта, аккаунты читателей, избранное и свои анекдоты.

Проект вырос из Telegram-бота на Python: старый код больше не используется и
лежит в [`legacy/python`](legacy/python) для истории. Активный стек — Go на
бэкенде и React на фронтенде.

## Возможности

- Список категорий с количеством анекдотов в каждой.
- Случайный анекдот из архива, опционально в пределах категории.
- Случайный анекдот с `baneks.site` (парсинг HTML, ограничение размера ответа).
- Регистрация и вход по почте и паролю (`bcrypt`), сессии на непрозрачных токенах.
- Избранное: анекдот можно сохранить в аккаунт и убрать из него.
- Свои анекдоты: любой зарегистрированный читатель публикует анекдот в
  выбранной категории, дубликаты и слишком короткие тексты отклоняются.
- Проверка здоровья `/api/v1/health` с пингом базы.
- SPA отдаётся тем же бинарником: один процесс, один порт.

## Архитектура

Clean/hexagonal: домен не знает ни про HTTP, ни про PostgreSQL, ни про DI.

```mermaid
flowchart LR
    UI[React SPA] -->|HTTP JSON| HTTP[adapter/httpapi]
    HTTP --> UC[usecase]
    UC --> DOM[domain<br/>entities + ports + errors]
    UC -.->|реализует порты| PG[adapter/postgres]
    UC -.->|реализует порты| EXT[adapter/external<br/>baneks.site]
    PG --> DB[(PostgreSQL)]
    DI[di<br/>samber/do] -.-> HTTP
    DI -.-> UC
    DI -.-> PG
    DI -.-> EXT
```

```
backend/
  cmd/api       HTTP-сервер
  cmd/migrate   применение и откат миграций
  cmd/seed      импорт data/*.txt в PostgreSQL
  internal/
    domain/     сущности, порты, доменные ошибки
    usecase/    бизнес-логика (AnekdotService, AuthService)
    adapter/
      postgres/ репозитории на pgx/v5
      external/ клиент baneks.site
      httpapi/  роутер, middleware, обработчики, DTO
    config/     конфигурация из переменных окружения
    di/         сборка графа зависимостей
    app/        запуск и graceful shutdown
    logging/    настройка slog
    seed/       парсер и загрузчик архива
  migrations/   SQL-миграции, встроенные в бинарник через go:embed
frontend/
  src/          React + TypeScript SPA
data/           categories.txt и anek_data.txt (архив бота)
legacy/python/  старый Telegram-бот на Python
```

## Требования

- Go 1.24+
- Node.js 18+ и npm
- PostgreSQL 16 (локально или через `docker compose`)
- Docker — опционально, только для базы

## Быстрый старт

```bash
# 1. База
docker compose up -d

# 2. Зависимости и миграции
cd backend
make tidy
make migrate-up

# 3. Импорт архива (идемпотентно, можно запускать повторно)
make seed

# 4. Сборка фронтенда в backend/web
make frontend-build

# 5. Запуск
make run
```

Приложение: <http://localhost:8080>. Проверка: `curl localhost:8080/api/v1/health`.

Фронтенд для разработки запускается отдельно и проксирует `/api` на бэкенд:

```bash
make frontend-dev   # http://localhost:5173
```

## Переменные окружения

Переменные читаются из окружения. `backend/.env.example` — готовый список значений для
локального запуска; файл `.env` автоматически не загружается, поэтому
экспортируйте переменные (`set -a; source .env; set +a`) или передавайте их в
командной строке. `make run` и `make seed` уже подставляют `DATA_DIR=../data` и
`WEB_DIR=web`.

| Переменная | По умолчанию | Назначение |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | адрес HTTP-сервера |
| `DATABASE_URL` | `postgres://anekdot:anekdot@localhost:5432/anekdot?sslmode=disable` | строка подключения к PostgreSQL |
| `DB_MAX_CONNS` | `10` | максимум соединений в пуле |
| `DB_MIN_CONNS` | `2` | минимум соединений в пуле |
| `DB_MAX_CONN_LIFETIME` | `30m` | время жизни соединения |
| `DB_MAX_CONN_IDLE_TIME` | `5m` | время простоя соединения |
| `SESSION_TTL` | `720h` | время жизни сессии читателя; должен быть больше нуля |
| `CORS_ORIGINS` | `http://localhost:5173` | разрешённые источники через запятую или `*` |
| `TRUST_PROXY_HEADERS` | `false` | доверять `X-Forwarded-For` (только за своим прокси) |
| `WEB_DIR` | пусто | каталог с собранной SPA; пусто — только API (`make run` передаёт `web`) |
| `DATA_DIR` | `data` | каталог с `categories.txt` и `anek_data.txt` (`make seed` передаёт `../data`) |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | `json` или `text` |
| `EXTERNAL_ANEKDOT_URL` | `https://baneks.site/random` | источник внешних анекдотов |
| `REQUEST_TIMEOUT` | `10s` | таймаут внешнего HTTP-запроса |
| `SHUTDOWN_TIMEOUT` | `15s` | бюджет graceful shutdown |

## API

Все ответы — JSON. Ошибки в едином виде:

```json
{ "error": { "code": "not_found", "message": "Ничего не найдено." } }
```

| Метод | Путь | Описание |
| --- | --- | --- |
| `GET` | `/api/v1/health` | статус сервиса и базы |
| `GET` | `/api/v1/categories` | категории с количеством анекдотов |
| `GET` | `/api/v1/anekdots/random` | случайный анекдот из архива |
| `GET` | `/api/v1/anekdots/random?category_id=3` | случайный анекдот в категории |
| `GET` | `/api/v1/anekdots/external` | случайный анекдот с `baneks.site` |
| `POST` | `/api/v1/anekdots` | опубликовать свой анекдот (нужен токен) |
| `GET` | `/api/v1/anekdots/mine` | мои анекдоты (нужен токен) |
| `GET` | `/api/v1/favorites` | избранное (нужен токен) |
| `PUT` | `/api/v1/favorites/{id}` | добавить в избранное (нужен токен) |
| `DELETE` | `/api/v1/favorites/{id}` | убрать из избранного (нужен токен) |
| `POST` | `/api/v1/auth/register` | регистрация |
| `POST` | `/api/v1/auth/login` | вход |
| `POST` | `/api/v1/auth/logout` | выход (нужен токен) |
| `GET` | `/api/v1/auth/me` | текущий пользователь (нужен токен) |

### Примеры

```bash
curl localhost:8080/api/v1/categories

curl 'localhost:8080/api/v1/anekdots/random?category_id=3'

curl localhost:8080/api/v1/anekdots/external

# Регистрация: в ответе token, его и передаём дальше
curl -X POST localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"reader@example.com","password":"password123","display_name":"Читатель"}'

curl -X POST localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"reader@example.com","password":"password123"}'

curl -X POST localhost:8080/api/v1/anekdots \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"category_id":3,"text":"Анекдот собственного сочинения"}'

curl localhost:8080/api/v1/anekdots/mine -H "Authorization: Bearer $TOKEN"

curl -X PUT localhost:8080/api/v1/favorites/42 -H "Authorization: Bearer $TOKEN"

curl localhost:8080/api/v1/favorites -H "Authorization: Bearer $TOKEN"
```

Токен передаётся в `Authorization: Bearer <token>`; выдаётся при регистрации и
входе, живёт `SESSION_TTL`.

Ограничения: тело запроса на вход и регистрацию — до 8 КиБ, пароль — 8–72
символа, почта — до 254 символов, отображаемое имя — до 100 символов. Тело
нового анекдота — до 16 КиБ, текст — 10–4000 символов; дубликат текста в
категории возвращает `409 conflict`. Регистрация и вход ограничены 10 запросами в
минуту с одного IP.

## База данных и миграции

`backend/migrations/000001_init.up.sql` создаёт:

- `categories` — категории (`id`, `name`);
- `anekdots` — анекдоты (`id`, `category_id` → `categories`, `text`) с
  уникальным индексом `(category_id, md5(text))`, что делает импорт идемпотентным.

`backend/migrations/000002_users.up.sql` добавляет аккаунты и удаляет обращения:

- `users` — читатели (`email` в нижнем регистре и уникальный, `password_hash`,
  `display_name`);
- `sessions` — сессии (`token_hash` — SHA-256 от токена, `user_id`, `expires_at`);
- `anekdots.author_id` → `users` — автор пользовательского анекдота (NULL для
  импортированных);
- `favorites` — избранное (`user_id`, `anekdot_id`, первичный ключ по паре).

Миграции встроены в бинарник через `go:embed`, поэтому `migrate` не зависит от
файловой системы:

```bash
cd backend
make migrate-up     # применить все
make migrate-down   # откатить все
```

## Импорт архива

`make seed` читает `data/categories.txt` и `data/anek_data.txt`, нормализует
текст (`\n` → перевод строки, HTML-сущности, обрезка пробелов) и загружает
данные пачками по 500 строк в транзакции. Повторный запуск не создаёт дубликатов.
Строки, не прошедшие разбор, пропускаются и попадают в счётчики статистики.

## Разработка

```bash
cd backend
make build   # bin/api, bin/migrate, bin/seed
make test    # go test ./...
make vet
make fmt
make lint    # нужен golangci-lint
```

Тесты не требуют базы: репозитории подменены фейками на уровне портов, парсер
проверяется на временных файлах. Конфигурация линтера — `backend/.golangci.yml`.

## Безопасность

- Пароли хранятся только как `bcrypt`-хеши (cost 12), токены сессий — только как
  SHA-256-хеши в базе.
- CSP, `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy` для всех ответов.
- Лимит запросов на `POST /api/v1/auth/register`, `POST /api/v1/auth/login` и
  `POST /api/v1/anekdots` по IP.
- Строгий разбор JSON и ограничение размера тела запроса.
- Внешний HTML читается с лимитом 1 МиБ и превращается в текст.
- SQL — только параметризованные запросы через `pgx`.

## Контент

Анекдоты не проходят предварительную модерацию и могут содержать ненормативную
лексику. Внешние анекдоты приходят с `baneks.site` и не контролируются проектом.

## Старый Telegram-бот

Каталог `legacy/python` содержит исходный бот (`bot/anekdot_bot.py`), модели
SQLAlchemy и `requirements.txt`. Он не собирается и не запускается в рамках
проекта; сохранён, чтобы можно было свериться с исходной логикой и текстами меню.

