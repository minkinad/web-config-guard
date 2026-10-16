# Разработка

Требуются Go 1.22+, `make`; Docker нужен только для сборки образа; build stage использует Go 1.27.2. Каждая директория проекта — отдельный Go-модуль, команды запускаются из неё.

```bash
make setup
make build
make run
make test
make fmt
make lint
make check
make docker-build
```

Примеры прямого запуска:

```bash
go run ./cmd/web-config-guard --stdin < config.yaml
go run ./cmd/web-config-guard --serve
```

`make lint` устанавливает закреплённую версию golangci-lint через Go modules при первом запуске. Go modules и контейнерные образы требуют доступа к registries. Не коммитьте `.env`, `bin/` и `coverage.out`.
