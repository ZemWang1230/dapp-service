-- TimeLocker 数据库初始化脚本
-- 执行前请确保数据库已创建

-- 删除已存在的表（按依赖关系逆序）
DROP TABLE IF EXISTS compound_timelocks CASCADE;
DROP TABLE IF EXISTS openzeppelin_timelocks CASCADE;
DROP TABLE IF EXISTS user_assets CASCADE;
DROP TABLE IF EXISTS support_chains CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- 1. 用户表 (users) - 以钱包地址为核心，支持chain_id
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    wallet_address VARCHAR(42) NOT NULL UNIQUE, -- 钱包地址作为唯一标识
    chain_id INTEGER NOT NULL DEFAULT 1,        -- 当前使用的链ID（用于timelock合约）
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_login TIMESTAMP WITH TIME ZONE,
    status INTEGER DEFAULT 1
);

-- 2. 支持的区块链表 (support_chains) - 简化设计
CREATE TABLE support_chains (
    id BIGSERIAL PRIMARY KEY,
    chain_name VARCHAR(50) NOT NULL UNIQUE, -- Covalent API的chainName，如 'eth-mainnet'
    display_name VARCHAR(100) NOT NULL,     -- 显示名称，如 'Ethereum Mainnet'
    chain_id BIGINT NOT NULL,               -- 链ID，如 1, 56
    native_token VARCHAR(10) NOT NULL,      -- 原生代币符号，如 'ETH', 'BNB'
    logo_url TEXT,                          -- 链的Logo URL
    is_testnet BOOLEAN NOT NULL DEFAULT false, -- 是否是测试网
    is_active BOOLEAN NOT NULL DEFAULT true,   -- 是否激活
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 3. 用户资产表 (user_assets) - 增加Logo信息和24h涨幅
CREATE TABLE user_assets (
    id BIGSERIAL PRIMARY KEY,
    wallet_address VARCHAR(42) NOT NULL REFERENCES users(wallet_address) ON DELETE CASCADE,
    chain_name VARCHAR(50) NOT NULL,       -- Covalent的chainName
    contract_address VARCHAR(42) NOT NULL DEFAULT '', -- 合约地址，原生代币为空字符串
    token_symbol VARCHAR(20) NOT NULL,
    token_name VARCHAR(100) NOT NULL,
    token_decimals INTEGER NOT NULL DEFAULT 18,
    balance VARCHAR(100) NOT NULL DEFAULT '0', -- 格式化的余额
    balance_wei VARCHAR(100) NOT NULL DEFAULT '0', -- Wei单位余额
    usd_value DECIMAL(20,8) DEFAULT 0,
    token_price DECIMAL(20,8) DEFAULT 0,
    price_change24h DECIMAL(10,4) DEFAULT 0, -- 24小时价格涨跌幅（%）
    is_native BOOLEAN NOT NULL DEFAULT false,
    token_logo_url TEXT,                    -- 代币Logo URL
    chain_logo_url TEXT,                    -- 链Logo URL
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(wallet_address, chain_name, contract_address)  -- 确保唯一性
);

-- 4. Compound标准Timelock合约表 (compound_timelocks)
CREATE TABLE compound_timelocks (
    id BIGSERIAL PRIMARY KEY,
    creator_address VARCHAR(42) NOT NULL REFERENCES users(wallet_address) ON DELETE CASCADE, -- 创建者/导入者地址
    chain_id INTEGER NOT NULL,
    chain_name VARCHAR(50) NOT NULL,
    contract_address VARCHAR(42) NOT NULL,
    tx_hash VARCHAR(66),                        -- 创建交易hash（创建时使用）
    min_delay BIGINT NOT NULL,                  -- 最小延迟时间（秒）
    admin VARCHAR(42) NOT NULL,                 -- 管理员地址
    pending_admin VARCHAR(42),                  -- 待定管理员地址
    remark VARCHAR(500) DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'deleted')),
    is_imported BOOLEAN NOT NULL DEFAULT false, -- 是否导入的合约
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(chain_id, contract_address)          -- 确保同一链和合约地址的唯一性
);

-- 5. OpenZeppelin标准Timelock合约表 (openzeppelin_timelocks)
CREATE TABLE openzeppelin_timelocks (
    id BIGSERIAL PRIMARY KEY,
    creator_address VARCHAR(42) NOT NULL REFERENCES users(wallet_address) ON DELETE CASCADE, -- 创建者/导入者地址
    chain_id INTEGER NOT NULL,
    chain_name VARCHAR(50) NOT NULL,
    contract_address VARCHAR(42) NOT NULL,
    tx_hash VARCHAR(66),                        -- 创建交易hash（创建时使用）
    min_delay BIGINT NOT NULL,                  -- 最小延迟时间（秒）
    proposers TEXT NOT NULL,                    -- 提议者地址列表（JSON格式）
    executors TEXT NOT NULL,                    -- 执行者地址列表（JSON格式）
    cancellers TEXT NOT NULL,                   -- 取消者地址列表（JSON格式）
    remark VARCHAR(500) DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'deleted')),
    is_imported BOOLEAN NOT NULL DEFAULT false, -- 是否导入的合约
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(chain_id, contract_address)          -- 确保同一链和合约地址的唯一性
);

-- 创建索引
CREATE INDEX idx_users_wallet_address ON users(wallet_address);
CREATE INDEX idx_users_chain_id ON users(chain_id);
CREATE INDEX idx_support_chains_chain_name ON support_chains(chain_name);
CREATE INDEX idx_support_chains_chain_id ON support_chains(chain_id);
CREATE INDEX idx_support_chains_is_active ON support_chains(is_active);
CREATE INDEX idx_support_chains_is_testnet ON support_chains(is_testnet);
CREATE INDEX idx_user_assets_wallet_address ON user_assets(wallet_address);
CREATE INDEX idx_user_assets_chain_name ON user_assets(chain_name);
CREATE INDEX idx_user_assets_usd_value ON user_assets(usd_value DESC);

-- Compound timelock索引
CREATE INDEX idx_compound_timelocks_creator_address ON compound_timelocks(creator_address);
CREATE INDEX idx_compound_timelocks_chain_id ON compound_timelocks(chain_id);
CREATE INDEX idx_compound_timelocks_chain_name ON compound_timelocks(chain_name);
CREATE INDEX idx_compound_timelocks_contract_address ON compound_timelocks(contract_address);
CREATE INDEX idx_compound_timelocks_admin ON compound_timelocks(admin);
CREATE INDEX idx_compound_timelocks_pending_admin ON compound_timelocks(pending_admin);
CREATE INDEX idx_compound_timelocks_status ON compound_timelocks(status);

-- OpenZeppelin timelock索引
CREATE INDEX idx_openzeppelin_timelocks_creator_address ON openzeppelin_timelocks(creator_address);
CREATE INDEX idx_openzeppelin_timelocks_chain_id ON openzeppelin_timelocks(chain_id);
CREATE INDEX idx_openzeppelin_timelocks_chain_name ON openzeppelin_timelocks(chain_name);
CREATE INDEX idx_openzeppelin_timelocks_contract_address ON openzeppelin_timelocks(contract_address);
CREATE INDEX idx_openzeppelin_timelocks_status ON openzeppelin_timelocks(status);

-- 插入支持的链数据（包含主网和测试网）
INSERT INTO support_chains (chain_name, display_name, chain_id, native_token, is_testnet, is_active) VALUES
-- 主网
('eth-mainnet', 'Ethereum Mainnet', 1, 'ETH', false, true),
('matic-mainnet', 'Polygon Mainnet', 137, 'MATIC', false, true),
('avalanche-mainnet', 'Avalanche C-Chain', 43114, 'AVAX', false, true),
('bsc-mainnet', 'BNB Smart Chain', 56, 'BNB', false, true),
('arbitrum-mainnet', 'Arbitrum One', 42161, 'ETH', false, true),
('optimism-mainnet', 'Optimism', 10, 'ETH', false, true),
('base-mainnet', 'Base', 8453, 'ETH', false, true),
('fantom-mainnet', 'Fantom', 250, 'FTM', false, true),
('moonbeam-mainnet', 'Moonbeam', 1284, 'GLMR', false, true),

-- 测试网
('eth-sepolia', 'Ethereum Sepolia', 11155111, 'ETH', true, true),
('matic-mumbai', 'Polygon Mumbai', 80001, 'MATIC', true, true),
('avalanche-testnet', 'Avalanche Fuji', 43113, 'AVAX', true, true),
('bsc-testnet', 'BNB Smart Chain Testnet', 97, 'BNB', true, true),
('arbitrum-sepolia', 'Arbitrum Sepolia', 421614, 'ETH', true, true),
('optimism-sepolia', 'Optimism Sepolia', 11155420, 'ETH', true, true);
