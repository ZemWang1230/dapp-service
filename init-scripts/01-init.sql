-- TimeLocker 数据库初始化脚本
-- 这个脚本会在PostgreSQL容器首次启动时执行

-- 创建必要的扩展
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS "btree_gin";

-- 设置时区
SET timezone = 'UTC';

-- 配置数据库参数优化性能
ALTER DATABASE timelocker_db SET timezone = 'UTC';
ALTER DATABASE timelocker_db SET log_statement = 'none';
ALTER DATABASE timelocker_db SET log_min_duration_statement = 1000;

-- 创建应用专用schema（可选，如果需要更好的组织）
-- CREATE SCHEMA IF NOT EXISTS timelocker;
-- GRANT ALL PRIVILEGES ON SCHEMA timelocker TO timelocker;

-- 输出初始化完成信息
DO $$ 
BEGIN 
    RAISE NOTICE 'TimeLocker database initialized successfully';
    RAISE NOTICE 'Extensions created: uuid-ossp, pg_trgm, btree_gin';
    RAISE NOTICE 'Database tables will be created automatically by the Go application';
END $$;
