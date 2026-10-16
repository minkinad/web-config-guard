.PHONY: setup build run test lint fmt check docker-build
setup:
	go mod download
build:
	go build -o bin/web-config-guard ./cmd/web-config-guard
run:
	go run ./cmd/web-config-guard --serve
test:
	go test ./...
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...
fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')
check:
	go vet ./...
	go test -race ./...
docker-build:
	docker build -t web-config-guard:dev .
