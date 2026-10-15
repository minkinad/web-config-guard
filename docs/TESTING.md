# Тестирование

Быстрая проверка: `go test ./...`; конкурентность: `go test -race ./...`; статический анализ: `go vet ./...`; покрытие: `go test -coverprofile=coverage.out ./...` и `go tool cover -func=coverage.out`. Форматирование: `test -z "$(gofmt -l $(find . -name '*.go' -not -path './vendor/*'))"`.

Unit и integration тесты не должны обращаться к внешнему интернету. Для HTTP используйте `httptest.Server` и локальные fixtures, для проверки времени — короткие, но устойчивые deadlines. Benchmarks запускайте отдельно `go test -bench=. ./...`, публикуя оборудование и команду. Искусственный порог coverage не установлен. CI повторяет основные команды и собирает бинарники; реальные GitHub Actions проверяются только после push/PR.
