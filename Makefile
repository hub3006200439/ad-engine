.PHONY: docker-up docker-down docker-down run migrate-run migrate build fmt

APP_NAME=adengine
CMD=./cmd/adserver
CONFIG=./configs/config.toml

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
