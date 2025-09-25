# TimeLocker Makefile
# Docker deployment and maintenance management tool

.PHONY: help build up down restart logs backup backup-physical restore-logic restore-physical clean \
        status health check-env dev-setup \
        db-connect db-size reset \
        update rebuild rebuild-backend \
        monitor tail-logs logs-backend logs-backup logs-list clear-logs \
        backup-list backup-cleanup \
        security-check permissions

# Version and configuration
VERSION ?= latest
ENV_FILE ?= .env
COMPOSE_FILE ?= docker-compose.yml
BACKUP_PREFIX ?= timelocker

# Color definition
GREEN = \033[0;32m
YELLOW = \033[1;33m
BLUE = \033[0;34m
RED = \033[0;31m
NC = \033[0m # No Color

# Default target
help:
	@echo ""
	@echo "$(BLUE)════════════════════════════════════════════════════════════════$(NC)"
	@echo "$(BLUE)                    TimeLocker Docker Management                   $(NC)"
	@echo "$(BLUE)════════════════════════════════════════════════════════════════$(NC)"
	@echo ""
	@echo "$(GREEN)🚀 Deployment management$(NC)"
	@echo "  make prod-setup       - 生产环境一键设置"
	@echo "  make build            - 构建Docker镜像"
	@echo "  make up               - 启动所有服务"
	@echo "  make down             - 停止所有服务"
	@echo "  make restart          - 重启所有服务"
	@echo ""
	@echo "$(GREEN)📊 监控管理$(NC)"
	@echo "  make status           - 显示服务状态"
	@echo "  make health           - 健康检查"
	@echo "  make logs             - 显示所有日志"
	@echo "  make tail-logs        - 实时查看日志"
	@echo "  make logs-backend     - 查看后端应用日志"
	@echo "  make logs-backup      - 查看备份调度器日志"
	@echo "  make logs-list        - 列出本地日志文件"
	@echo "  make monitor          - 系统监控面板"
	@echo ""
	@echo "$(GREEN)💾 备份管理$(NC)"
	@echo "  make backup                 - 创建数据库逻辑备份(PostgreSQL运行时)"
	@echo "  make backup-physical        - 创建数据库物理备份(PostgreSQL停止时)"
	@echo "  make backup-list            - 列出所有备份文件"
	@echo "  make backup-cleanup         - 清理旧备份文件"
	@echo "  make restore-logic FILE=    - 恢复数据库逻辑备份"
	@echo "  make restore-physical FILE= - 恢复数据库物理备份(PostgreSQL停止时)"
	@echo ""
	@echo "$(GREEN)🗄️  数据库管理$(NC)"
	@echo "  make db-connect       - 连接数据库"
	@echo "  make db-size          - 查看数据库大小"
	@echo "  make reset            - 重置数据库(危险)"
	@echo ""
	@echo "$(GREEN)🔧 维护管理$(NC)"
	@echo "  make update           - 更新服务"
	@echo "  make rebuild          - 重新构建所有服务"
	@echo "  make rebuild-backend  - 重新构建后端服务"
	@echo "  make clean            - 清理资源"
	@echo "  make clear-logs       - 清理日志文件"
	@echo "  make permissions      - 设置文件权限"
	@echo ""
	@echo "$(GREEN)🛠️  开发工具$(NC)"
	@echo "  make check-env        - 检查环境配置"
	@echo "  make security-check   - 安全检查"
	@echo ""
	@echo "$(YELLOW)📖 使用示例:$(NC)"
	@echo "  make prod-setup                                    # 生产环境一键部署"
	@echo "  make backup                                        # 创建逻辑备份"
	@echo "  make backup-physical                               # 创建物理备份"
	@echo "  make restore-logic FILE=backups/backup.sql         # 恢复逻辑备份"
	@echo "  make restore-physical FILE=backups/physical.tar    # 恢复物理备份"
	@echo "  make monitor                                       # 查看系统状态"
	@echo ""

# ================================
# 环境设置和检查
# ================================

check-env:
	@echo "$(BLUE)🔍 检查环境配置...$(NC)"
	@if [ ! -f "$(ENV_FILE)" ]; then \
		echo "$(RED)❌ 环境文件 $(ENV_FILE) 不存在$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)✅ 环境文件检查通过$(NC)"
	@docker --version
	@docker-compose --version

# ================================
# 一键部署
# ================================

prod-setup:
	@echo "$(BLUE)🚀 生产环境一键设置...$(NC)"
	@make build
	@make up
	@echo "$(GREEN)✅ 生产环境设置完成$(NC)"
	@make status

# ================================
# Docker 基础操作
# ================================

build:
	@echo "$(BLUE)🔨 构建Docker镜像...$(NC)"
	@docker-compose build --no-cache

up:
	@echo "$(BLUE)🚀 启动所有服务...$(NC)"
	@docker-compose up -d
	@echo "$(GREEN)✅ 服务启动完成$(NC)"

down:
	@echo "$(BLUE)🛑 停止所有服务...$(NC)"
	@docker-compose down
	@echo "$(GREEN)✅ 服务停止完成$(NC)"

restart:
	@echo "$(BLUE)🔄 重启所有服务...$(NC)"
	@make down
	@sleep 3
	@make up

# ================================
# 监控和日志
# ================================

status:
	@echo "$(BLUE)📊 服务状态概览$(NC)"
	@echo "$(YELLOW)═══════════════════════════════════════$(NC)"
	@docker-compose ps
	@echo ""
	@echo "$(BLUE)💾 备份文件状态$(NC)"
	@echo "$(YELLOW)═══════════════════════════════════════$(NC)"
	@if [ -d "backups" ]; then \
		echo "备份目录: $$(ls -la backups/ | wc -l) 个文件"; \
		echo "最新备份: $$(ls -t backups/*.tar.gz backups/timelocker_backup_* 2>/dev/null | head -1 | xargs basename 2>/dev/null || echo '无')"; \
		echo "目录大小: $$(du -sh backups/ 2>/dev/null | cut -f1 || echo '0B')"; \
	else \
		echo "$(RED)❌ 备份目录不存在$(NC)"; \
	fi

health:
	@echo "$(BLUE)🏥 健康检查...$(NC)"
	@docker-compose exec timelocker-backend wget --spider -q http://localhost:8080/api/v1/health && \
		echo "$(GREEN)✅ 后端服务健康$(NC)" || \
		echo "$(RED)❌ 后端服务异常$(NC)"
	@docker-compose exec postgres pg_isready -U timelocker -d timelocker_db && \
		echo "$(GREEN)✅ 数据库服务健康$(NC)" || \
		echo "$(RED)❌ 数据库服务异常$(NC)"
	@docker-compose exec redis redis-cli ping | grep -q PONG && \
		echo "$(GREEN)✅ Redis服务健康$(NC)" || \
		echo "$(RED)❌ Redis服务异常$(NC)"

logs:
	@echo "$(BLUE)📋 显示所有服务日志...$(NC)"
	@docker-compose logs --tail=100

tail-logs:
	@echo "$(BLUE)📋 实时查看所有服务日志...$(NC)"
	@docker-compose logs -f

logs-backend:
	@echo "$(BLUE)📋 查看后端应用日志...$(NC)"
	@if [ -f "logs/timelocker-backend/app.log" ]; then \
		tail -f logs/timelocker-backend/app.log; \
	elif [ -d "logs/timelocker-backend" ]; then \
		echo "$(YELLOW)⚠️  应用日志文件不存在，请重启服务: make restart$(NC)"; \
		find logs/timelocker-backend -name "*.log" -exec tail -f {} + 2>/dev/null || echo "$(YELLOW)⚠️  暂无其他日志文件$(NC)"; \
	else \
		echo "$(RED)❌ 日志目录不存在，请先运行: make permissions$(NC)"; \
	fi

logs-backup:
	@echo "$(BLUE)📋 查看备份调度器日志...$(NC)"
	@if [ -f "logs/backup-scheduler/backup.log" ]; then \
		tail -f logs/backup-scheduler/backup.log; \
	else \
		echo "$(YELLOW)⚠️  备份日志文件不存在$(NC)"; \
	fi

logs-list:
	@echo "$(BLUE)📋 本地日志文件列表$(NC)"
	@echo "$(YELLOW)═══════════════════════════════════════$(NC)"
	@if [ -d "logs" ]; then \
		find logs -name "*.log" -exec ls -lah {} \; 2>/dev/null || echo "$(YELLOW)⚠️  暂无日志文件$(NC)"; \
	else \
		echo "$(RED)❌ 日志目录不存在，请先运行: make permissions$(NC)"; \
	fi

monitor:
	@echo "$(BLUE)📊 系统监控面板$(NC)"
	@echo "$(YELLOW)═══════════════════════════════════════$(NC)"
	@make status
	@echo ""
	@echo "$(BLUE)💻 系统资源使用情况$(NC)"
	@echo "$(YELLOW)═══════════════════════════════════════$(NC)"
	@docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}"

# ================================
# 备份管理
# ================================

backup:
	@echo "$(BLUE)💾 创建数据库逻辑备份...$(NC)"
	@echo "$(YELLOW)⚠️  确保PostgreSQL正在运行$(NC)"
	@mkdir -p backups
	@if ! docker-compose ps postgres | grep -q "Up"; then \
		echo "$(RED)❌ PostgreSQL容器未运行，请先启动: make up$(NC)"; \
		exit 1; \
	fi
	@BACKUP_FILE="backups/logical_backup_$$(date +%Y%m%d_%H%M%S).sql"; \
	echo "$(YELLOW)备份文件: $$BACKUP_FILE$(NC)"; \
	if docker-compose exec -T postgres pg_dump -U timelocker -d timelocker_db > $$BACKUP_FILE; then \
		echo "$(GREEN)✅ 逻辑备份成功: $$BACKUP_FILE$(NC)"; \
		ls -lh $$BACKUP_FILE; \
	else \
		echo "$(RED)❌ 逻辑备份失败$(NC)"; \
		rm -f $$BACKUP_FILE; \
		exit 1; \
	fi

backup-physical:
	@echo "$(BLUE)💾 创建数据库物理备份...$(NC)"
	@echo "$(RED)⚠️  警告: 这将停止PostgreSQL服务进行物理备份$(NC)"
	@mkdir -p backups
	@if docker-compose ps postgres | grep -q "Up"; then \
		echo "$(YELLOW)正在停止PostgreSQL容器...$(NC)"; \
		docker-compose stop postgres; \
		sleep 3; \
	fi
	@BACKUP_FILE="backups/physical_backup_$$(date +%Y%m%d_%H%M%S).tar.gz"; \
	echo "$(YELLOW)物理备份文件: $$BACKUP_FILE$(NC)"; \
	if docker run --rm \
		-v $$(docker volume ls -q | grep postgres_data):/source:ro \
		-v $$(pwd)/backups:/backup \
		alpine:latest \
		tar -czf /backup/$$(basename $$BACKUP_FILE) -C /source .; then \
		echo "$(GREEN)✅ 物理备份成功: $$BACKUP_FILE$(NC)"; \
		ls -lh $$BACKUP_FILE; \
	else \
		echo "$(RED)❌ 物理备份失败$(NC)"; \
		rm -f $$BACKUP_FILE; \
		echo "$(BLUE)正在重新启动PostgreSQL容器...$(NC)"; \
		docker-compose start postgres; \
		exit 1; \
	fi

backup-list:
	@echo "$(BLUE)📋 备份文件列表$(NC)"
	@if [ -d "backups" ]; then \
		ls -la backups/; \
	else \
		echo "$(RED)❌ 备份目录不存在$(NC)"; \
	fi

backup-cleanup:
	@echo "$(BLUE)🧹 清理30天前的备份文件...$(NC)"
	@if [ -d "backups" ]; then \
		echo "清理前备份文件数: $$(find backups/ -name "*.sql" -o -name "*.dump" -o -name "*.tar.gz" | wc -l)"; \
		find backups/ -name "*.sql" -mtime +30 -delete 2>/dev/null || true; \
		find backups/ -name "*.dump" -mtime +30 -delete 2>/dev/null || true; \
		find backups/ -name "*.tar.gz" -mtime +30 -delete 2>/dev/null || true; \
		echo "清理后备份文件数: $$(find backups/ -name "*.sql" -o -name "*.dump" -o -name "*.tar.gz" | wc -l)"; \
		echo "$(GREEN)✅ 备份清理完成$(NC)"; \
	else \
		echo "$(YELLOW)⚠️  备份目录不存在$(NC)"; \
	fi

restore-logic:
	@if [ -z "$(FILE)" ]; then \
		echo "$(RED)❌ 请指定备份文件: make restore-logic FILE=backups/logical_backup.sql$(NC)"; \
		exit 1; \
	fi
	@if [ ! -f "$(FILE)" ]; then \
		echo "$(RED)❌ 备份文件不存在: $(FILE)$(NC)"; \
		exit 1; \
	fi
	@if ! docker-compose ps postgres | grep -q "Up"; then \
		echo "$(RED)❌ PostgreSQL容器未运行，请先启动: make up$(NC)"; \
		exit 1; \
	fi
	@echo "$(BLUE)🔄 恢复逻辑备份: $(FILE)$(NC)"
	@echo "$(RED)⚠️  警告: 这将覆盖现有数据$(NC)"
	@read -p "确认恢复? 输入 'YES' 继续: " confirm && [ "$$confirm" = "YES" ]
	@echo "$(BLUE)正在恢复数据库...$(NC)"
	@if docker-compose exec -T postgres psql -U timelocker -d timelocker_db < $(FILE); then \
		echo "$(GREEN)✅ 逻辑备份恢复成功$(NC)"; \
	else \
		echo "$(RED)❌ 逻辑备份恢复失败$(NC)"; \
		exit 1; \
	fi

restore-physical:
	@if [ -z "$(FILE)" ]; then \
		echo "$(RED)❌ 请指定物理备份文件: make restore-physical FILE=backups/physical_backup.tar.gz$(NC)"; \
		exit 1; \
	fi
	@if [ ! -f "$(FILE)" ]; then \
		echo "$(RED)❌ 物理备份文件不存在: $(FILE)$(NC)"; \
		exit 1; \
	fi
	@echo "$(BLUE)🔄 恢复物理备份: $(FILE)$(NC)"
	@echo "$(RED)⚠️  警告: 这将完全替换PostgreSQL数据目录$(NC)"
	@read -p "确认恢复? 输入 'YES' 继续: " confirm && [ "$$confirm" = "YES" ]
	@echo "$(BLUE)正在停止PostgreSQL容器...$(NC)"
	@docker-compose stop postgres
	@sleep 3
	@echo "$(BLUE)正在清空现有数据目录...$(NC)"
	@docker run --rm \
		-v $$(docker volume ls -q | grep postgres_data):/target \
		alpine:latest \
		sh -c "rm -rf /target/* /target/.*" 2>/dev/null || true
	@echo "$(BLUE)正在恢复物理备份...$(NC)"
	@if docker run --rm \
		-v $$(pwd)/$(FILE):/backup.tar.gz:ro \
		-v $$(docker volume ls -q | grep postgres_data):/target \
		alpine:latest \
		tar -xzf /backup.tar.gz -C /target; then \
		echo "$(GREEN)✅ 物理备份恢复成功$(NC)"; \
	else \
		echo "$(RED)❌ 物理备份恢复失败$(NC)"; \
		echo "$(BLUE)正在重新启动PostgreSQL容器...$(NC)"; \
		docker-compose start postgres; \
		exit 1; \
	fi

# ================================
# 数据库管理
# ================================

db-connect:
	@echo "$(BLUE)🔗 连接数据库...$(NC)"
	@docker-compose exec postgres psql -U timelocker -d timelocker_db

db-size:
	@echo "$(BLUE)📏 数据库大小信息$(NC)"
	@echo "$(YELLOW)═══════════════════════════════════════$(NC)"
	@docker-compose exec postgres psql -U timelocker -d timelocker_db -c "SELECT pg_size_pretty(pg_database_size('timelocker_db')) as database_size;"


reset:
	@echo "$(RED)⚠️  警告: 这将删除所有数据库数据!$(NC)"
	@read -p "确认删除所有数据? 输入 'RESET' 继续: " confirm && [ "$$confirm" = "RESET" ]
	@echo "$(BLUE)🗑️  重置数据库...$(NC)"
	@docker-compose down
	@docker volume rm $$(docker volume ls -q | grep -E "(postgres_data|redis_data)" || true) 2>/dev/null || true
	@docker-compose up -d
	@echo "$(GREEN)✅ 数据库重置完成$(NC)"

# ================================
# 维护管理
# ================================

update:
	@echo "$(BLUE)🔄 更新服务...$(NC)"
	@docker-compose pull
	@make restart

rebuild:
	@echo "$(BLUE)🔨 重新构建所有服务...$(NC)"
	@docker-compose build --no-cache
	@make restart

rebuild-backend:
	@echo "$(BLUE)🔨 重新构建后端服务...$(NC)"
	@docker-compose build --no-cache timelocker-backend
	@docker-compose restart timelocker-backend
	@echo "$(GREEN)✅ 后端服务重启完成$(NC)"
	@echo "$(BLUE)等待服务启动...$(NC)"
	@sleep 5
	@make health

clean:
	@echo "$(BLUE)🧹 清理Docker资源...$(NC)"
	@docker system prune -f
	@docker volume prune -f
	@echo "$(GREEN)✅ 清理完成$(NC)"

clear-logs:
	@echo "$(BLUE)🗑️  清理日志文件...$(NC)"
	@docker-compose exec timelocker-backend sh -c "rm -f /var/log/timelocker/*.log" 2>/dev/null || true
	@echo "$(GREEN)✅ 日志清理完成$(NC)"

permissions:
	@echo "$(BLUE)🔐 设置文件权限...$(NC)"
	@mkdir -p backups logs/timelocker-backend logs/backup-scheduler
	@echo "$(GREEN)✅ 目录创建完成$(NC)"

# ================================
# 开发工具
# ================================

security-check:
	@echo "$(BLUE)🔒 安全检查...$(NC)"
	@echo "$(YELLOW)═══════════════════════════════════════$(NC)"
	@if [ -f "$(ENV_FILE)" ]; then \
		echo "✅ 环境文件权限: $$(stat -f %A $(ENV_FILE) 2>/dev/null || stat -c %a $(ENV_FILE) 2>/dev/null || echo 'unknown')"; \
		if grep -q "your_.*_here" $(ENV_FILE); then \
			echo "$(RED)❌ 检测到默认密码，请修改$(NC)"; \
		else \
			echo "$(GREEN)✅ 未检测到默认密码$(NC)"; \
		fi \
	fi
	@echo "✅ Docker容器安全扫描..."
	@docker-compose config --quiet && echo "$(GREEN)✅ Docker配置验证通过$(NC)" || echo "$(RED)❌ Docker配置有误$(NC)"


# ================================
# 快速操作别名
# ================================

start: up
stop: down
ps: status
shell:
	@docker-compose exec timelocker-backend sh