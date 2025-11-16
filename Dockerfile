# =========================
# Build stage
# =========================
FROM golang:1.25-alpine3.21 AS build

RUN apk add --no-cache build-base git ca-certificates tzdata

WORKDIR /app

# Сначала зависимости — для кеша
COPY go.mod go.sum ./
RUN go mod download

# Затем всё остальное
COPY . .

# Сборка бинарника
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/bin/adserver ./cmd/adserver

# =========================
# Runtime stage
# =========================
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -g '' appuser

WORKDIR /app

# Бинарь
COPY --from=build /app/bin/adserver /app/adserver

# Конфиг файл configs/config.toml (ожидается что есть)
COPY configs/config.toml /app/configs/config.toml

USER appuser

EXPOSE 8443

ENTRYPOINT ["/app/adserver"]
