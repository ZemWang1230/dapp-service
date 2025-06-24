-- TimeLocker 数据库初始化脚本
-- 执行前请确保数据库已创建

-- 删除已存在的表（按依赖关系逆序）
DROP TABLE IF EXISTS user_assets CASCADE;
DROP TABLE IF EXISTS chain_tokens CASCADE;
DROP TABLE IF EXISTS support_tokens CASCADE;
DROP TABLE IF EXISTS support_chains CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- 1. 用户表 (users) - 重构为以钱包地址为核心，chain_id为最后登录的链
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    wallet_address VARCHAR(42) NOT NULL UNIQUE, -- 钱包地址作为唯一标识
    chain_id INTEGER NOT NULL DEFAULT 1, -- 最后登录的链ID
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_login TIMESTAMP WITH TIME ZONE,
    preferences JSONB DEFAULT '{}',
    status INTEGER DEFAULT 1
);

-- 2. 支持的区块链表 (support_chains)
CREATE TABLE support_chains (
    id BIGSERIAL PRIMARY KEY,
    chain_id BIGINT NOT NULL UNIQUE,
    name VARCHAR(50) NOT NULL,
    symbol VARCHAR(10) NOT NULL,
    rpc_provider VARCHAR(20) NOT NULL DEFAULT 'alchemy',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 3. 支持的代币表 (support_tokens)
CREATE TABLE support_tokens (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(10) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    coingecko_id VARCHAR(50) NOT NULL UNIQUE,
    decimals INTEGER NOT NULL DEFAULT 18,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 4. 链代币关联表 (chain_tokens)
CREATE TABLE chain_tokens (
    id BIGSERIAL PRIMARY KEY,
    chain_id BIGINT NOT NULL REFERENCES support_chains(id) ON DELETE CASCADE,
    token_id BIGINT NOT NULL REFERENCES support_tokens(id) ON DELETE CASCADE,
    contract_address VARCHAR(42) DEFAULT '',
    is_native BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(chain_id, token_id)
);

-- 5. 用户资产表 (user_assets) - 重构为以wallet_address关联
CREATE TABLE user_assets (
    id BIGSERIAL PRIMARY KEY,
    wallet_address VARCHAR(42) NOT NULL REFERENCES users(wallet_address) ON DELETE CASCADE,
    chain_id BIGINT NOT NULL,
    token_id BIGINT NOT NULL REFERENCES support_tokens(id) ON DELETE CASCADE,
    balance VARCHAR(100) NOT NULL DEFAULT '0',
    balance_wei VARCHAR(100) NOT NULL DEFAULT '0',
    usd_value DECIMAL(20,8) DEFAULT 0,
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(wallet_address, chain_id, token_id)  -- 确保同一地址在同一链上的同一代币只有一条记录
);

-- 创建索引
CREATE INDEX idx_users_wallet_address ON users(wallet_address);
CREATE INDEX idx_users_chain_id ON users(chain_id);
CREATE INDEX idx_support_chains_chain_id ON support_chains(chain_id);
CREATE INDEX idx_support_tokens_symbol ON support_tokens(symbol);
CREATE INDEX idx_chain_tokens_chain_id ON chain_tokens(chain_id);
CREATE INDEX idx_chain_tokens_token_id ON chain_tokens(token_id);
CREATE INDEX idx_user_assets_token_id ON user_assets(token_id);
CREATE INDEX idx_user_assets_wallet_address ON user_assets(wallet_address);
CREATE INDEX idx_user_assets_chain_id ON user_assets(chain_id);

-- 插入初始链数据
INSERT INTO support_chains (chain_id, name, symbol, rpc_provider, is_active) VALUES
(1, 'Ethereum', 'ETH', 'alchemy', true),
(56, 'BSC', 'BNB', 'alchemy', true),
(137, 'Polygon', 'MATIC', 'alchemy', true),
(42161, 'Arbitrum One', 'ETH', 'alchemy', true),
(10, 'Optimism', 'ETH', 'alchemy', true),
(8453, 'Base', 'ETH', 'alchemy', true),
(11155111, 'Sepolia', 'ETH', 'alchemy', true);

-- 插入初始代币数据
INSERT INTO support_tokens (symbol, name, coingecko_id, decimals, is_active) VALUES
('ETH', 'Ethereum', 'ethereum', 18, true),
('BNB', 'BNB', 'binancecoin', 18, true),
('MATIC', 'Polygon', 'matic-network', 18, true),
('USDC', 'USD Coin', 'usd-coin', 6, true),
('USDT', 'Tether', 'tether', 6, true),
('WETH', 'Wrapped Ethereum', 'weth', 18, true),
('ARB', 'Arbitrum', 'arbitrum', 18, true),
('OP', 'Optimism', 'optimism', 18, true);

-- 插入链代币关联数据
-- Ethereum 主网 (chain_id = 1)
INSERT INTO chain_tokens (chain_id, token_id, contract_address, is_native, is_active) 
SELECT 
    sc.id, st.id, ct.contract_address, ct.is_native, true
FROM support_chains sc
CROSS JOIN (VALUES 
    ('ETH', '', true),
    ('USDC', '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', false),
    ('USDT', '0xdAC17F958D2ee523a2206206994597C13D831ec7', false)
) ct(symbol, contract_address, is_native)
JOIN support_tokens st ON st.symbol = ct.symbol
WHERE sc.chain_id = 1;

-- BSC 主网 (chain_id = 56)
INSERT INTO chain_tokens (chain_id, token_id, contract_address, is_native, is_active) 
SELECT 
    sc.id, st.id, ct.contract_address, ct.is_native, true
FROM support_chains sc
CROSS JOIN (VALUES 
    ('BNB', '', true),
    ('USDC', '0x8ac76a51cc950d9822d68b83fe1ad97b32cd580d', false),
    ('USDT', '0x55d398326f99059ff775485246999027b3197955', false),
    ('ETH', '0x2170ed0880ac9a755fd29b2688956bd959f933f8', false)
) ct(symbol, contract_address, is_native)
JOIN support_tokens st ON st.symbol = ct.symbol
WHERE sc.chain_id = 56;

-- Polygon 主网 (chain_id = 137)
INSERT INTO chain_tokens (chain_id, token_id, contract_address, is_native, is_active) 
SELECT 
    sc.id, st.id, ct.contract_address, ct.is_native, true
FROM support_chains sc
CROSS JOIN (VALUES 
    ('MATIC', '', true),
    ('USDC', '0x3c499c542cef5e3811e1192ce70d8cc03d5c3359', false),
    ('USDT', '0xc2132d05d31c914a87c6611c10748aeb04b58e8f', false)
) ct(symbol, contract_address, is_native)
JOIN support_tokens st ON st.symbol = ct.symbol
WHERE sc.chain_id = 137;

-- Arbitrum One (chain_id = 42161)
INSERT INTO chain_tokens (chain_id, token_id, contract_address, is_native, is_active) 
SELECT 
    sc.id, st.id, ct.contract_address, ct.is_native, true
FROM support_chains sc
CROSS JOIN (VALUES 
    ('ETH', '', true),
    ('USDC', '0xaf88d065e77c8cC2239327C5EDb3A432268e5831', false),
    ('USDT', '0xfd086bc7cd5c481dcc9c85ebe478a1c0b69fcbb9', false),
    ('ARB', '0x912CE59144191C1204E64559FE8253a0e49E6548', false)
) ct(symbol, contract_address, is_native)
JOIN support_tokens st ON st.symbol = ct.symbol
WHERE sc.chain_id = 42161; 

-- Optimism (chain_id = 10)
INSERT INTO chain_tokens (chain_id, token_id, contract_address, is_native, is_active) 
SELECT 
    sc.id, st.id, ct.contract_address, ct.is_native, true
FROM support_chains sc
CROSS JOIN (VALUES 
    ('ETH', '', true),
    ('USDC', '0x0b2c639c533813f4aa9d7837caf62653d097ff85', false),
    ('USDT', '0x94b008aa00579c1307b0ef2c499ad98a8ce58e58', false),
    ('OP', '0x4200000000000000000000000000000000000042', false)
) ct(symbol, contract_address, is_native)
JOIN support_tokens st ON st.symbol = ct.symbol
WHERE sc.chain_id = 10;

-- Base (chain_id = 8453)
INSERT INTO chain_tokens (chain_id, token_id, contract_address, is_native, is_active) 
SELECT 
    sc.id, st.id, ct.contract_address, ct.is_native, true
FROM support_chains sc
CROSS JOIN (VALUES 
    ('ETH', '', true),
    ('USDC', '0x833589fcd6edb6e08f4c7c32d4f71b54bda02913', false),
    ('USDT', '0xfde4c96c8593536e31f229ea8f37b2ada2699bb2', false)
) ct(symbol, contract_address, is_native)
JOIN support_tokens st ON st.symbol = ct.symbol
WHERE sc.chain_id = 8453;
