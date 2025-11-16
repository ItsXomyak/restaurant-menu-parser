# ---------
# Stage 1: Build
# ---------
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Кэшируем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь исходный код
COPY . .

# Создаем отдельную папку для бинарников
RUN mkdir -p /app/bin

# Собираем API (исправлен путь)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/api ./cmd/api

# Собираем Worker (исправлен путь)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/worker ./cmd/worker

# ---------
# Stage 2: Release
# ---------
FROM alpine:latest

# Добавляем сертификаты и wget для healthcheck
RUN apk add --no-cache ca-certificates wget

# Создаем непривилегированного пользователя
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

WORKDIR /home/appuser

# Копируем бинарники из builder
COPY --from=builder /app/bin/api .
COPY --from=builder /app/bin/worker .

# CMD не задаем, docker-compose будет указывать что запускать