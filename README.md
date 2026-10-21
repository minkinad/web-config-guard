# web-config-guard [![CI](https://github.com/minkinad/web-config-guard/actions/workflows/ci.yml/badge.svg)](https://github.com/minkinad/web-config-guard/actions/workflows/ci.yml) [![CodeQL](https://github.com/minkinad/web-config-guard/actions/workflows/codeql.yml/badge.svg)](https://github.com/minkinad/web-config-guard/actions/workflows/codeql.yml) [![Security](https://github.com/minkinad/web-config-guard/actions/workflows/security.yml/badge.svg)](https://github.com/minkinad/web-config-guard/actions/workflows/security.yml)

`web-config-guard` проверяет YAML/JSON настройки веб-приложений на типичные опасные решения. Существующие сценарии проверки файла, директории, stdin и HTTP payload сохранены.

**Статус:** действующий CLI и HTTP API с дополнительной настройкой порога критичности для CI. Сканер эвристический и не заменяет ручной аудит.

## Возможности

- Рекурсивный анализ `.yaml`, `.yml`, `.json`, скрытые директории пропускаются.
- Правила: debug, пароль открытым текстом, wildcard bind, отключённый TLS, слабый алгоритм, слишком широкие права файла.
- Вывод text или JSON; `--fail-on low|medium|high` управляет только кодом выхода, не скрывает находки.
- `--silent` сохраняет вывод, но возвращает 0 при находках.
- HTTP `POST /v1/check?filename=config.yaml`, совместимый `GET /healthz` и новые `/health/live`, `/health/ready`.

## Быстрый старт

```bash
make setup
make build
./bin/web-config-guard ./config.yaml
./bin/web-config-guard --format json ./configs
cat config.yaml | ./bin/web-config-guard --stdin --fail-on high
./bin/web-config-guard --serve --addr 127.0.0.1:8080
curl -X POST --data-binary @config.yaml 'http://127.0.0.1:8080/v1/check?filename=config.yaml'
```

Коды выхода: 0 — нет находок выше выбранного порога или `--silent`; 1 — есть находки; 2 — ошибка аргументов, чтения или разбора. HTTP API возвращает JSON с полем `problems`; при отсутствии находок текущий формат может содержать `null`.

## Конфигурация

Опциональный `--config configs/settings.yaml` задаёт адрес HTTP, формат/уровень логов и `fail_on`. Порядок: встроенные значения → YAML → `WCG_LISTEN`, `WCG_LOG_FORMAT`, `WCG_LOG_LEVEL`, `WCG_FAIL_ON` → флаги CLI. По умолчанию `:8080`, JSON, info, low. Не храните секреты в настройках утилиты.

### Безопасная эксплуатация HTTP API

Для локального использования запускайте сервер с `--addr 127.0.0.1:8080`. Для удалённых клиентов разместите его за доверенным reverse proxy с аутентификацией, TLS и ограничением запросов; ограничьте прямой доступ к порту firewall. Встроенной аутентификации нет, поэтому API нельзя публиковать напрямую. HTTP принимает только содержимое `POST /v1/check`; путь к локальному файлу или директории передать ему нельзя. Проверка локальных файлов доступна только через CLI.

Сервер допускает до 64 соединений и четырёх одновременных проверок. Максимальный body — 10 MiB. Входные JSON/YAML документы ограничены глубиной 64, числом узлов 10 000 и длиной пути настройки 512 байт; YAML aliases отклоняются. Эти лимиты действуют также для CLI. Заголовки ограничены 16 KiB; `ReadHeaderTimeout` — 5 с, `ReadTimeout` — 15 с, `WriteTimeout` — 30 с, `IdleTimeout` — 60 с. При SIGINT/SIGTERM сервер перестаёт принимать новые соединения и ждёт активные запросы до 10 с.

Ошибки API возвращаются в JSON с полем `error`: 400 — некорректный документ или превышение его структурных лимитов; 413 — body больше 10 MiB; 503 — все слоты проверки заняты (`Retry-After: 1`). При отмене запроса клиентом обработка прекращается, поэтому ответ может не дойти до клиента. `GET /healthz`, `/health/live`, `/health/ready` остаются доступными при занятых слотах.

## Разработка

```bash
go test ./...
go test -race ./...
go vet ./...
make lint
make check
make docker-build
```

Пакеты `internal/config`, `guard`, `runner`, `server`, `cli` сохраняют прежние границы. Аудит и дальнейшие решения находятся в [архитектуре](docs/ARCHITECTURE.md), задачи — в [roadmap](docs/ROADMAP.md).

## Ограничения

Правила используют эвристики и могут давать false positive и false negative. Нет SARIF, baseline/diff или пользовательских правил. Лицензия в репозитории пока не оформлена.
