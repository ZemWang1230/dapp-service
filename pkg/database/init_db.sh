#!/bin/bash

# TimeLocker 数据库初始化脚本
# 使用方法: ./init_db.sh [数据库名] [用户名] [密码] [主机] [端口]

# 默认参数
DB_NAME=${1:-"timelocker_db"}
DB_USER=${2:-"timelocker"}
DB_PASSWORD=${3:-"timelocker_password"}
DB_HOST=${4:-"localhost"}
DB_PORT=${5:-"5432"}

echo "正在初始化TimeLocker数据库..."
echo "数据库: $DB_NAME"
echo "用户: $DB_USER"
echo "主机: $DB_HOST:$DB_PORT"

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SQL_FILE="$SCRIPT_DIR/init.sql"

# 检查SQL文件是否存在
if [ ! -f "$SQL_FILE" ]; then
    echo "错误: 找不到初始化SQL文件: $SQL_FILE"
    exit 1
fi

# 连接数据库并执行SQL脚本
echo "正在执行数据库初始化脚本..."

PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$SQL_FILE"

if [ $? -eq 0 ]; then
    echo "✅ 数据库初始化成功!"
    echo ""
    echo "数据库表结构:"
    echo "- users (用户表)"
    echo "- support_chains (支持的区块链表)"
    echo "- support_tokens (支持的代币表)"
    echo "- chain_tokens (链代币关联表)"
    echo "- user_assets (用户资产表)"
    echo ""
    echo "已初始化的数据:"
    echo "- 4条区块链记录 (Ethereum, BSC, Polygon, Arbitrum)"
    echo "- 10种代币配置"
    echo "- 链与代币的关联配置"
    echo ""
    echo "现在可以启动TimeLocker后端服务了!"
else
    echo "❌ 数据库初始化失败!"
    echo "请检查:"
    echo "1. 数据库连接参数是否正确"
    echo "2. 用户是否有足够的权限"
    echo "3. 数据库是否已创建"
    exit 1
fi 