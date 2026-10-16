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

Опциональный `--config configs/settings.yaml` задаёт адрес HTTP, формат/уровень логов и `fail_on`. Порядок: встроенные значения → YAML → `WCG_LISTEN`, `WCG_LOG_FORMAT`, `WCG_LOG_LEVEL`, `WCG_FAIL_ON` → флаги CLI. По умолчанию `:8080`, JSON, info, low. Образ запускает HTTP режим; без доверенного reverse proxy не публикуйте его наружу. Не храните секреты в настройках утилиты.

## Разработка

```bash
go test ./...
go test -race ./...
go vet ./...
make check
make docker-build
```

Пакеты `internal/config`, `guard`, `runner`, `server`, `cli` сохраняют прежние границы. Аудит и дальнейшие решения находятся в [архитектуре](docs/ARCHITECTURE.md), задачи — в [roadmap](docs/ROADMAP.md).

## Ограничения

Правила используют эвристики и могут давать false positive и false negative. Нет SARIF, baseline/diff или пользовательских правил. HTTP API не имеет аутентификации и ограничивает payload 10 MiB. Для CLI такие же ограничения действуют на файлы и stdin. Лицензия в репозитории пока не оформлена.
