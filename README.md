# web-config-guard

[![CI](https://github.com/minkinad/web-config-guard/actions/workflows/ci.yml/badge.svg)](https://github.com/minkin/web-config-guard/actions/workflows/ci.yml)
[![CodeQL](https://github.com/minkinad/web-config-guard/actions/workflows/codeql.yml/badge.svg)](https://github.com/minkin/web-config-guard/actions/workflows/codeql.yml)
![Go Version](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go&logoColor=white)
![Config Formats](https://img.shields.io/badge/config-YAML%20%7C%20JSON-blue)

`web-config-guard` - CLI-утилита на Go для анализа YAML/JSON-конфигураций веб-приложений и поиска потенциально опасных настроек.

## Возможности

- Принимает путь к конфигурационному файлу первым позиционным аргументом.
- Поддерживает YAML и JSON.
- Умеет читать конфигурацию из `stdin` через флаг `--stdin`.
- Завершается с ненулевым кодом, если найдена хотя бы одна проблема.
- Поддерживает `-s` / `--silent`, чтобы не возвращать ошибочный код при найденных проблемах.
- Рекурсивно анализирует директории с файлами `.json`, `.yaml` и `.yml`.
- Проверяет права доступа к файлам через `os.Stat`.
- Может запускаться как HTTP-сервер с REST API.

## Проверки

- Логирование в debug-режиме или включенный debug mode.
- Пароли, заданные в конфигурации открытым текстом.
- Использование `0.0.0.0` без явных ограничений доступа.
- Отключенный TLS/HTTPS или отключенная проверка сертификатов.
- Устаревшие или небезопасные алгоритмы, например `MD5`, `SHA-1`, `DES`, `3DES`, `RC4`.
- Слишком широкие права доступа к конфигурационным файлам.

Каждая найденная проблема содержит уровень критичности, путь до настройки, краткое объяснение и рекомендацию.

## Сборка

```bash
go mod download
go build -o web-config-guard ./cmd/web-config-guard
```

## Использование CLI

```bash
./web-config-guard ./config.yaml
./web-config-guard ./configs
cat config.yaml | ./web-config-guard --stdin
./web-config-guard --stdin --format json < config.yaml
./web-config-guard --silent ./config.yaml
```

Коды завершения:

- `0`: проблемы не найдены или использован флаг `--silent`.
- `1`: найдена хотя бы одна проблема.
- `2`: неверные аргументы, файл не читается или конфигурация не парсится.

Пример вывода:

```text
HIGH [config.yaml:storage.digest-algorithm]: используется устаревший или небезопасный алгоритм MD5. Замените алгоритм на современный вариант, например SHA-256/Argon2/bcrypt или актуальный TLS cipher suite по назначению
LOW [config.yaml:log.level]: логирование в debug-режиме. Поменяйте уровень логирования на info или выше для production-окружений
Found 2 problem(s).
```

## HTTP API

Запуск сервера:

```bash
./web-config-guard --serve --addr :8080
```

Проверка конфигурации:

```bash
curl -sS \
  -X POST \
  --data-binary @config.yaml \
  'http://localhost:8080/v1/check?filename=config.yaml'
```

Health check:

```bash
curl -sS http://localhost:8080/healthz
```

## Разработка

```bash
go test ./...
go test -race ./...
go vet ./...
```

## CI И CodeQL

В репозитории настроены GitHub Actions:

- `.github/workflows/ci.yml`: форматирование, `go vet`, `go test -race ./...` и сборка бинарника.
- `.github/workflows/codeql.yml`: статический security-анализ Go-кода через CodeQL.

CodeQL особенно уместен для этого проекта, потому что утилита сама относится к security tooling. Для публичных репозиториев GitHub code scanning доступен бесплатно; для приватных репозиториев может понадобиться включенный GitHub Advanced Security.

Код разделен по зонам ответственности:

- `internal/config`: парсинг и нормализация JSON/YAML.
- `internal/guard`: правила, модель проблемы, проверка прав доступа к файлам.
- `internal/runner`: проверка файлов, директорий, `stdin` и payload из API.
- `internal/server`: REST API.
- `internal/cli`: CLI, флаги и коды завершения.

Чтобы добавить новое правило, нужно реализовать интерфейс `guard.Rule` и добавить правило в `DefaultRules()`.

## Примечание

Проект выполнен как решение тестового задания.
