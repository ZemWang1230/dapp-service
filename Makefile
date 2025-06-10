.PHONY: build run test clean demo deps docker-build docker-run

# 应用名称
APP_NAME=timelocker-backend
VERSION=v1.0.0

# 构建应用
build:
	@echo "Building $(APP_NAME)..."
	go build -o bin/$(APP_NAME) cmd/server/main.go

# 运行应用
run:
	@echo "Running $(APP_NAME)..."
	go run cmd/server/main.go

# 运行测试
test:
	@echo "Running tests..."
	go test -v ./...

# 运行测试并生成覆盖率报告
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# 运行演示
demo:
	@echo "Running wallet authentication demo..."
	go run examples/wallet_auth_demo.go

# 安装依赖
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# 清理构建文件
clean:
	@echo "Cleaning up..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# 格式化代码
fmt:
	@echo "Formatting code..."
	go fmt ./...

# 代码检查
lint:
	@echo "Running linter..."
	golangci-lint run

# 初始化配置文件
init-config:
	@if [ ! -f config.yaml ]; then \
		echo "Creating config.yaml from example..."; \
		cp config.yaml.example config.yaml; \
		echo "Please edit config.yaml to set your database connection details"; \
	else \
		echo "config.yaml already exists"; \
	fi

# 数据库迁移（需要先启动数据库）
migrate:
	@echo "Running database migration..."
	go run cmd/server/main.go --migrate-only

# Docker构建
docker-build:
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):$(VERSION) .
	docker tag $(APP_NAME):$(VERSION) $(APP_NAME):latest

# Docker运行
docker-run:
	@echo "Running with Docker Compose..."
	docker-compose up -d

# Docker停止
docker-stop:
	@echo "Stopping Docker containers..."
	docker-compose down

# 开发环境设置
dev-setup: deps init-config
	@echo "Development environment setup complete!"
	@echo "Next steps:"
	@echo "1. Start PostgreSQL database"
	@echo "2. Edit config.yaml with your database settings"
	@echo "3. Run 'make run' to start the server"

# 生产环境构建
prod-build:
	@echo "Building for production..."
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/$(APP_NAME) cmd/server/main.go

# 帮助信息
help:
	@echo "Available commands:"
	@echo "  build         - Build the application"
	@echo "  run           - Run the application"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  demo          - Run wallet authentication demo"
	@echo "  deps          - Install dependencies"
	@echo "  clean         - Clean build files"
	@echo "  fmt           - Format code"
	@echo "  lint          - Run linter"
	@echo "  init-config   - Initialize config file"
	@echo "  migrate       - Run database migration"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run with Docker Compose"
	@echo "  docker-stop   - Stop Docker containers"
	@echo "  dev-setup     - Setup development environment"
	@echo "  prod-build    - Build for production"
	@echo "  help          - Show this help message" 