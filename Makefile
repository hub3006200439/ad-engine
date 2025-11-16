.PHONY: docker-up docker-down docker-down run migrate-run migrate build fmt test test-integration test-all

APP_NAME=adengine
CMD=./cmd/adserver
CONFIG=./configs/config.toml

# -----------------------------
# Tests
# -----------------------------
# Обычные unit-тесты (без integration)
test:
	@echo ">> Running unit tests"
	go test ./... -count=1 -run . -v -tags ""

# Интеграционные тесты:
# Требуется ENV TEST_PG_DSN="postgres://..."
test-integration:
ifndef TEST_PG_DSN
	$(error TEST_PG_DSN is not set. Example: export TEST_PG_DSN="postgres://adengine_user:password@localhost:5432/adengine?sslmode=disable")
endif
	@echo ">> Running integration tests"
	go test ./... -count=1 -run . -v -tags=integration

# Unit + integration подряд
test-all: test test-integration
	@echo ">> All tests finished"

# -----------------------------
# Docker commands
# -----------------------------

docker-up:
	docker compose up -d --build --force-recreate --remove-orphans

docker-down:
	docker compose down

# -----------------------------
# Local run
# -----------------------------

run:
	go run $(CMD) -config $(CONFIG)

migrate-run:
	MIGRATE=1 go run $(CMD) -config $(CONFIG)

migrate:
	MIGRATE=1 go run $(CMD) -config $(CONFIG) --migrate-only

# -----------------------------
# Build / Format
# -----------------------------

build:
	go build -o $(APP_NAME) $(CMD)

fmt:
	go fmt ./...
	go vet ./...
