# 多阶段构建 Dockerfile
# 第一阶段：构建阶段
FROM golang:1.23.10-alpine AS builder

# 安装必要的工具
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 复制go mod文件并下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用 timelocker-backend
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o timelocker-backend \
    ./cmd/server

# 第二阶段：运行阶段
FROM alpine:latest

# 安装ca-certificates for HTTPS requests and timezone data
RUN apk --no-cache add ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/timelocker-backend .

# 复制配置文件和其他必要文件
COPY config.yaml ./
COPY email_templates/ ./email_templates/

# 创建备份和日志目录
RUN mkdir -p /var/log/timelocker /var/log/caddy /app/backups

# 暴露端口
EXPOSE 8080

# 设置时区
ENV TZ=UTC

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

# 创建启动脚本
RUN echo '#!/bin/sh' > /app/start.sh && \
    echo 'exec ./timelocker-backend >> /var/log/timelocker/app.log 2>&1' >> /app/start.sh && \
    chmod +x /app/start.sh

# 启动应用
CMD ["./start.sh"]
