# Инструкции для агентов

- Сохраните существующие CLI флаги, positional path, exit codes 0/1/2, HTTP `/v1/check` и `/healthz`.
- Проверяемый документ парсится в `internal/config/parser.go`; operational YAML — в `settings.go`; правила — `internal/guard`; обход файлов — `runner`; HTTP — `server`; CLI — `cli`.
- Правила реализуют `guard.Rule`; новые правила должны иметь таблицы позитивных и негативных примеров. Не меняйте JSON `Problem` без явного плана совместимости.
- Не логируйте содержимое конфигов и секреты. Ограничивайте размер входа. HTTP API нельзя считать публичным без auth proxy.
- Проверка: `make setup`, `make check`, `make build`, `make docker-build` при доступном Docker.
- Definition of Done: старые сценарии проходят, новый кейс протестирован, документация совпадает с поведением, `go vet` и race detector проходят.
