#!/bin/bash

# TimeLocker 数据库初始化脚本
# 使用方法: ./init_db.sh [数据库名] [用户名] [密码] [主机] [端口]

# 默认参数
DB_NAME=${1:-"timelocker_db"}
DB_USER=${2:-"timelocker"}
DB_PASSWORD=${3:-"timelocker_password"}
DB_HOST=${4:-"localhost"}
DB_PORT=${5:-"5432"}

echo "Initializing TimeLocker database..."
echo "Database: $DB_NAME"
echo "User: $DB_USER"
echo "Host: $DB_HOST:$DB_PORT"

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SQL_FILE="$SCRIPT_DIR/init.sql"

# 检查SQL文件是否存在
if [ ! -f "$SQL_FILE" ]; then
    echo "Error: Could not find initialization SQL file: $SQL_FILE"
    exit 1
fi

# 连接数据库并执行SQL脚本
echo "Executing initialization SQL file: $SQL_FILE"

PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$SQL_FILE"

if [ $? -eq 0 ]; then
    echo "✅ Database initialization completed successfully!"
    echo ""
    echo "Now you can start the TimeLocker backend service!"
else
    echo "❌ Database initialization failed!"
    echo "Please check:"
    echo "1. Database connection parameters"
    echo "2. User permissions"
    echo "3. Database existence"
    exit 1
fi 