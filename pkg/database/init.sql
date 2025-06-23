-- TimeLocker 数据库初始化脚本
-- 执行前请确保数据库已创建

-- 删除已存在的表（按依赖关系逆序）
DROP TABLE IF EXISTS user_assets CASCADE;
DROP TABLE IF EXISTS chain_tokens CASCADE;
DROP TABLE IF EXISTS support_tokens CASCADE;
DROP TABLE IF EXISTS support_chains CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- 1. 用户表 (users)
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    wallet_address VARCHAR(42) NOT NULL UNIQUE,
    chain_id INTEGER NOT NULL DEFAULT 1,
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

-- 5. 用户资产表 (user_assets) - 唯一约束确保不重复
CREATE TABLE user_assets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wallet_address VARCHAR(42) NOT NULL,
    chain_id BIGINT NOT NULL,
    token_id BIGINT NOT NULL REFERENCES support_tokens(id) ON DELETE CASCADE,
    balance VARCHAR(100) NOT NULL DEFAULT '0',
    balance_wei VARCHAR(100) NOT NULL DEFAULT '0',
    usd_value DECIMAL(20,8) DEFAULT 0,
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, chain_id, token_id)  -- 确保用户在同一链上的同一代币只有一条记录
);

-- 创建索引
CREATE INDEX idx_users_wallet_address ON users(wallet_address);
CREATE INDEX idx_users_chain_id ON users(chain_id);
CREATE INDEX idx_support_chains_chain_id ON support_chains(chain_id);
CREATE INDEX idx_support_tokens_symbol ON support_tokens(symbol);
CREATE INDEX idx_chain_tokens_chain_id ON chain_tokens(chain_id);
CREATE INDEX idx_chain_tokens_token_id ON chain_tokens(token_id);
CREATE INDEX idx_user_assets_user_id ON user_assets(user_id);
CREATE INDEX idx_user_assets_wallet_address ON user_assets(wallet_address);
CREATE INDEX idx_user_assets_chain_id ON user_assets(chain_id);

-- 插入初始链数据
INSERT INTO support_chains (chain_id, name, symbol, rpc_provider, is_active) VALUES
(1, 'Ethereum', 'ETH', 'alchemy', true),
(56, 'BSC', 'BNB', 'alchemy', true),
(137, 'Polygon', 'MATIC', 'alchemy', true),
(42161, 'Arbitrum One', 'ETH', 'alchemy', true);

-- 插入初始代币数据
INSERT INTO support_tokens (symbol, name, coingecko_id, decimals, is_active) VALUES
('ETH', 'Ethereum', 'ethereum', 18, true),
('BNB', 'BNB', 'binancecoin', 18, true),
('MATIC', 'Polygon', 'matic-network', 18, true),
('USDC', 'USD Coin', 'usd-coin', 6, true),
('USDT', 'Tether', 'tether', 6, true),
('WETH', 'Wrapped Ethereum', 'weth', 18, true),
('DAI', 'Dai Stablecoin', 'dai', 18, true),
('UNI', 'Uniswap', 'uniswap', 18, true),
('LINK', 'Chainlink', 'chainlink', 18, true),
('AAVE', 'Aave', 'aave', 18, true);

-- 插入链代币关联数据
-- Ethereum 主网 (chain_id = 1)
INSERT INTO chain_tokens (chain_id, token_id, contract_address, is_native, is_active) 
SELECT 
    sc.id, st.id, ct.contract_address, ct.is_native, true
FROM support_chains sc
CROSS JOIN (VALUES 
    ('ETH', '', true),
    ('USDC', '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', false),
    ('USDT', '0xdAC17F958D2ee523a2206206994597C13D831ec7', false),
    ('DAI', '0x6B175474E89094C44Da98b954EedeAC495271d0F', false),
    ('UNI', '0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984', false),
    ('LINK', '0x514910771AF9Ca656af840dff83E8264EcF986CA', false),
    ('AAVE', '0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9', false)
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
    ('USDC', '0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d', false),
    ('USDT', '0x55d398326f99059fF775485246999027B3197955', false),
    ('ETH', '0x2170Ed0880ac9A755fd29B2688956BD959F933F8', false)
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
    ('USDC', '0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174', false),
    ('USDT', '0xc2132D05D31c914a87C6611C10748AEb04B58e8F', false),
    ('DAI', '0x8f3Cf7ad23Cd3CaDbD9735AFf958023239c6A063', false),
    ('WETH', '0x7ceB23fD6bC0adD59E62ac25578270cFf1b9f619', false),
    ('AAVE', '0xD6DF932A45C0f255f85145f286eA0b292B21C90B', false)
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
    ('USDC', '0xFF970A61A04b1cA14834A43f5dE4533eBDDB5CC8', false),
    ('USDT', '0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9', false),
    ('DAI', '0xDA10009cBd5D07dd0CeCc66161FC93D7c9000da1', false),
    ('UNI', '0xFa7F8980b0f1E64A2062791cc3b0871572f1F7f0', false),
    ('LINK', '0xf97f4df75117a78c1A5a0DBb814Af92458539FB4', false)
) ct(symbol, contract_address, is_native)
JOIN support_tokens st ON st.symbol = ct.symbol
WHERE sc.chain_id = 42161; 