# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o nehuang-server ./cmd/server

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/nehuang-server .

# 启动脚本 — 运行时根据环境变量生成 MySQL 配置文件
RUN echo '#!/bin/sh' > /app/start.sh && \
    echo 'set -e' >> /app/start.sh && \
    echo 'mkdir -p /app/configs/mysql' >> /app/start.sh && \
    echo 'cat > /app/configs/mysql/nehuang_agent.cnf <<EOFCONF' >> /app/start.sh && \
    echo '[client]' >> /app/start.sh && \
    echo 'user=${NEHUANG_MYSQL_USER:-root}' >> /app/start.sh && \
    echo 'password=${NEHUANG_MYSQL_PASSWORD}' >> /app/start.sh && \
    echo 'host=${NEHUANG_MYSQL_HOST:-127.0.0.1}' >> /app/start.sh && \
    echo 'port=${NEHUANG_MYSQL_PORT:-3306}' >> /app/start.sh && \
    echo 'database=${NEHUANG_MYSQL_DATABASE:-nehuang_db}' >> /app/start.sh && \
    echo 'default-character-set=utf8mb4' >> /app/start.sh && \
    echo 'EOFCONF' >> /app/start.sh && \
    echo 'export NEHUANG_HTTP_ADDR=:${PORT:-8080}' >> /app/start.sh && \
    echo 'exec ./nehuang-server' >> /app/start.sh && \
    chmod +x /app/start.sh

EXPOSE 8080
CMD ["/app/start.sh"]
