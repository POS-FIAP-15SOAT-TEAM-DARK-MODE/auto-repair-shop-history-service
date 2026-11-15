-include .env
export

run:
	go run cmd/service/main.go

test:
	LOG_LEVEL=PANIC go test ./... --race -v

test-integration:
	LOG_LEVEL=PANIC go test -tags=integration ./internal/integration/... -v

coverage:
	LOG_LEVEL=PANIC go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

up:
	docker compose up --build

down:
	docker compose down -v

.PHONY: run test test-integration coverage up down
