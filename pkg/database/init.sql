-- TimeLocker 数据库初始化脚本
-- 执行前请确保数据库已创建

-- =============================================================================
-- 第一步：删除已存在的表（按依赖关系逆序）
-- =============================================================================
DROP TABLE IF EXISTS emergency_notifications CASCADE;
DROP TABLE IF EXISTS email_send_logs CASCADE;
DROP TABLE IF EXISTS email_notifications CASCADE;
DROP TABLE IF EXISTS transactions CASCADE;
DROP TABLE IF EXISTS compound_timelocks CASCADE;
DROP TABLE IF EXISTS openzeppelin_timelocks CASCADE;
DROP TABLE IF EXISTS user_assets CASCADE;
DROP TABLE IF EXISTS abis CASCADE;
DROP TABLE IF EXISTS support_chains CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- =============================================================================
-- 第二步：创建表结构
-- =============================================================================

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
    alchemy_rpc_template TEXT,              -- Alchemy RPC URL模板，{API_KEY}为占位符
    infura_rpc_template TEXT,               -- Infura RPC URL模板，{API_KEY}为占位符
    custom_rpc_url TEXT,                    -- 自定义RPC URL（不需要API key的公共节点）
    rpc_enabled BOOLEAN NOT NULL DEFAULT true, -- 是否启用RPC功能
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

-- 4. ABI库表 (abis) - 支持用户自定义ABI和平台共享ABI
CREATE TABLE abis (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,                 -- ABI名称
    abi_content TEXT NOT NULL,                  -- ABI JSON内容
    owner VARCHAR(42) NOT NULL,                 -- 所有者地址，全0表示共享ABI
    description VARCHAR(500) DEFAULT '',        -- ABI描述
    is_shared BOOLEAN NOT NULL DEFAULT false,   -- 是否为共享ABI
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(name, owner)                         -- 同一所有者下名称唯一
);

-- 5. Compound标准Timelock合约表 (compound_timelocks)
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
    emergency_mode BOOLEAN NOT NULL DEFAULT false, -- 是否启用应急模式（针对此合约的所有交易）
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(chain_id, contract_address)          -- 确保同一链和合约地址的唯一性
);

-- 6. OpenZeppelin标准Timelock合约表 (openzeppelin_timelocks)
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
    emergency_mode BOOLEAN NOT NULL DEFAULT false, -- 是否启用应急模式（针对此合约的所有交易）
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(chain_id, contract_address)          -- 确保同一链和合约地址的唯一性
);

-- 7. 交易记录表 (transactions)
CREATE TABLE transactions (
    id BIGSERIAL PRIMARY KEY,
    creator_address VARCHAR(42) NOT NULL REFERENCES users(wallet_address) ON DELETE CASCADE, -- 交易创建者地址
    chain_id INTEGER NOT NULL,                     -- 链ID
    chain_name VARCHAR(50) NOT NULL,               -- 链名称
    timelock_address VARCHAR(42) NOT NULL,         -- Timelock合约地址
    timelock_standard VARCHAR(20) NOT NULL CHECK (timelock_standard IN ('compound', 'openzeppelin')), -- Timelock标准
    tx_hash VARCHAR(66) NOT NULL UNIQUE,           -- 交易哈希
    tx_data TEXT NOT NULL,                         -- 交易数据
    target VARCHAR(42) NOT NULL,                   -- 目标合约地址
    value VARCHAR(100) NOT NULL DEFAULT '0',       -- 转账金额(wei)
    function_sig VARCHAR(200),                     -- 函数签名
    eta BIGINT NOT NULL,                           -- 预计执行时间(Unix时间戳)
    queued_at TIMESTAMP WITH TIME ZONE,            -- 入队时间
    executed_at TIMESTAMP WITH TIME ZONE,          -- 执行时间
    canceled_at TIMESTAMP WITH TIME ZONE,          -- 取消时间
    status VARCHAR(20) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'ready', 'executed', 'expired', 'canceled')), -- 状态
    description VARCHAR(500) DEFAULT '',           -- 交易描述
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);



-- 插入支持的链数据（包含主网和测试网）
INSERT INTO support_chains (chain_name, display_name, chain_id, native_token, is_testnet, is_active, alchemy_rpc_template, infura_rpc_template, custom_rpc_url, rpc_enabled) VALUES
-- 主网
('eth-mainnet', 'Ethereum Mainnet', 1, 'ETH', false, true, 
 'https://eth-mainnet.g.alchemy.com/v2/{API_KEY}', 
 'https://mainnet.infura.io/v3/{API_KEY}', 
 NULL, true),
('avalanche-mainnet', 'Avalanche C-Chain', 43114, 'AVAX', false, true, 
 'https://avax-mainnet.g.alchemy.com/v2/{API_KEY}', 
 'https://avalanche-mainnet.infura.io/v3/{API_KEY}', 
 NULL, true),
('bsc-mainnet', 'BNB Smart Chain', 56, 'BNB', false, true, 
 'https://bnb-mainnet.g.alchemy.com/v2/{API_KEY}', 
 'https://bsc-mainnet.infura.io/v3/{API_KEY}', 
 NULL, true),
('arbitrum-mainnet', 'Arbitrum One', 42161, 'ETH', false, true, 
 'https://arb-mainnet.g.alchemy.com/v2/{API_KEY}', 
 'https://arbitrum-mainnet.infura.io/v3/{API_KEY}', 
 NULL, true),
('optimism-mainnet', 'Optimism', 10, 'ETH', false, true, 
 'https://opt-mainnet.g.alchemy.com/v2/{API_KEY}', 
 'https://optimism-mainnet.infura.io/v3/{API_KEY}', 
 NULL, true),
('base-mainnet', 'Base', 8453, 'ETH', false, true, 
 'https://base-mainnet.g.alchemy.com/v2/{API_KEY}', 
 'https://base-mainnet.infura.io/v3/{API_KEY}', 
 NULL, true),
('fantom-mainnet', 'Fantom', 250, 'FTM', false, true, 
 'https://fantom-mainnet.g.alchemy.com/v2/{API_KEY}', 
 'https://fantom-mainnet.infura.io/v3/{API_KEY}', 
 NULL, true),

-- 测试网
('eth-sepolia', 'Ethereum Sepolia', 11155111, 'ETH', true, true, 
 'https://eth-sepolia.g.alchemy.com/v2/{API_KEY}', 
 'https://sepolia.infura.io/v3/{API_KEY}', 
 NULL, true),
('avalanche-testnet', 'Avalanche Fuji', 43113, 'AVAX', true, true, 
 'https://avax-fuji.g.alchemy.com/v2/{API_KEY}', 
 'https://avax-fuji.infura.io/v3/{API_KEY}', 
 NULL, true),
('bsc-testnet', 'BNB Smart Chain Testnet', 97, 'BNB', true, true, 
 'https://bnb-testnet.g.alchemy.com/v2/{API_KEY}', 
 'https://bsc-testnet.infura.io/v3/{API_KEY}', 
 NULL, true),
('arbitrum-sepolia', 'Arbitrum Sepolia', 421614, 'ETH', true, true, 
 'https://arb-sepolia.g.alchemy.com/v2/{API_KEY}', 
 'https://arbitrum-sepolia.infura.io/v3/{API_KEY}', 
 NULL, true),
('optimism-sepolia', 'Optimism Sepolia', 11155420, 'ETH', true, true, 
 'https://opt-sepolia.g.alchemy.com/v2/{API_KEY}', 
 'https://optimism-sepolia.infura.io/v3/{API_KEY}', 
 NULL, true),
 ('monad-testnet', 'Monad Testnet', 10143, 'MONAD', true, true, 
 'https://monad-testnet.g.alchemy.com/v2/{API_KEY}', 
 'https://monad-testnet.infura.io/v3/{API_KEY}', 
 NULL, true);

-- 8. 邮件通知配置表 (email_notifications)
CREATE TABLE email_notifications (
    id BIGSERIAL PRIMARY KEY,
    wallet_address VARCHAR(42) NOT NULL REFERENCES users(wallet_address) ON DELETE CASCADE, -- 用户钱包地址
    email VARCHAR(255) NOT NULL,                    -- 邮箱地址
    email_remark VARCHAR(200) DEFAULT '',           -- 邮箱备注
    timelock_contracts TEXT NOT NULL DEFAULT '[]', -- 监听的timelock合约地址列表（JSON格式）
    is_verified BOOLEAN NOT NULL DEFAULT false,    -- 是否已验证邮箱
    verification_code VARCHAR(6),                   -- 验证码
    verification_expires_at TIMESTAMP WITH TIME ZONE, -- 验证码过期时间
    is_active BOOLEAN NOT NULL DEFAULT true,        -- 是否激活
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(wallet_address, email)                   -- 确保同一用户不能重复添加相同邮箱
);

-- 9. 邮件发送记录表 (email_send_logs)
CREATE TABLE email_send_logs (
    id BIGSERIAL PRIMARY KEY,
    email_notification_id BIGINT NOT NULL REFERENCES email_notifications(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,                    -- 接收邮箱
    timelock_address VARCHAR(42) NOT NULL,          -- 相关timelock合约地址
    transaction_hash VARCHAR(66),                   -- 相关交易hash
    event_type VARCHAR(50) NOT NULL,                -- 事件类型：proposal_created, proposal_canceled, ready_to_execute, executed, expired
    subject VARCHAR(500) NOT NULL,                  -- 邮件主题
    content TEXT NOT NULL,                          -- 邮件内容
    is_emergency BOOLEAN NOT NULL DEFAULT false,   -- 是否为应急邮件
    emergency_reply_token VARCHAR(64),              -- 应急邮件回复token
    is_replied BOOLEAN NOT NULL DEFAULT false,      -- 是否已回复（仅应急邮件）
    replied_at TIMESTAMP WITH TIME ZONE,            -- 回复时间
    send_status VARCHAR(20) NOT NULL DEFAULT 'pending', -- 发送状态：pending, sent, failed
    send_attempts INTEGER NOT NULL DEFAULT 0,       -- 发送尝试次数
    error_message TEXT,                             -- 错误信息
    sent_at TIMESTAMP WITH TIME ZONE,               -- 发送时间
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 10. 应急通知追踪表 (emergency_notifications) - 简化版本
CREATE TABLE emergency_notifications (
    id BIGSERIAL PRIMARY KEY,
    timelock_address VARCHAR(42) NOT NULL,          -- timelock合约地址
    transaction_hash VARCHAR(66) NOT NULL,          -- 交易hash
    event_type VARCHAR(50) NOT NULL,                -- 事件类型：proposal_created, proposal_canceled, ready_to_execute, executed, expired
    replied_emails INTEGER NOT NULL DEFAULT 0,      -- 已回复邮箱数量
    is_completed BOOLEAN NOT NULL DEFAULT false,    -- 是否完成（至少一个邮箱回复）
    next_send_at TIMESTAMP WITH TIME ZONE,          -- 下次发送时间
    send_count INTEGER NOT NULL DEFAULT 1,          -- 发送次数
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(timelock_address, transaction_hash, event_type) -- 确保同一交易的同一事件只有一个记录
);

-- =============================================================================
-- 第三步：创建索引
-- =============================================================================

-- 用户表索引
CREATE INDEX idx_users_wallet_address ON users(wallet_address);
CREATE INDEX idx_users_chain_id ON users(chain_id);

-- 支持链表索引
CREATE INDEX idx_support_chains_chain_name ON support_chains(chain_name);
CREATE INDEX idx_support_chains_chain_id ON support_chains(chain_id);
CREATE INDEX idx_support_chains_is_active ON support_chains(is_active);
CREATE INDEX idx_support_chains_is_testnet ON support_chains(is_testnet);

-- 用户资产表索引
CREATE INDEX idx_user_assets_wallet_address ON user_assets(wallet_address);
CREATE INDEX idx_user_assets_chain_name ON user_assets(chain_name);
CREATE INDEX idx_user_assets_usd_value ON user_assets(usd_value DESC);

-- ABI表索引
CREATE INDEX idx_abis_owner ON abis(owner);
CREATE INDEX idx_abis_name ON abis(name);
CREATE INDEX idx_abis_is_shared ON abis(is_shared);
CREATE INDEX idx_abis_created_at ON abis(created_at DESC);

-- Compound timelock索引
CREATE INDEX idx_compound_timelocks_creator_address ON compound_timelocks(creator_address);
CREATE INDEX idx_compound_timelocks_chain_id ON compound_timelocks(chain_id);
CREATE INDEX idx_compound_timelocks_chain_name ON compound_timelocks(chain_name);
CREATE INDEX idx_compound_timelocks_contract_address ON compound_timelocks(contract_address);
CREATE INDEX idx_compound_timelocks_admin ON compound_timelocks(admin);
CREATE INDEX idx_compound_timelocks_pending_admin ON compound_timelocks(pending_admin);
CREATE INDEX idx_compound_timelocks_status ON compound_timelocks(status);
CREATE INDEX idx_compound_timelocks_emergency_mode ON compound_timelocks(emergency_mode);

-- OpenZeppelin timelock索引
CREATE INDEX idx_openzeppelin_timelocks_creator_address ON openzeppelin_timelocks(creator_address);
CREATE INDEX idx_openzeppelin_timelocks_chain_id ON openzeppelin_timelocks(chain_id);
CREATE INDEX idx_openzeppelin_timelocks_chain_name ON openzeppelin_timelocks(chain_name);
CREATE INDEX idx_openzeppelin_timelocks_contract_address ON openzeppelin_timelocks(contract_address);
CREATE INDEX idx_openzeppelin_timelocks_status ON openzeppelin_timelocks(status);
CREATE INDEX idx_openzeppelin_timelocks_emergency_mode ON openzeppelin_timelocks(emergency_mode);

-- 交易记录表索引
CREATE INDEX idx_transactions_creator_address ON transactions(creator_address);
CREATE INDEX idx_transactions_chain_id ON transactions(chain_id);
CREATE INDEX idx_transactions_timelock_address ON transactions(timelock_address);
CREATE INDEX idx_transactions_timelock_standard ON transactions(timelock_standard);
CREATE INDEX idx_transactions_tx_hash ON transactions(tx_hash);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_eta ON transactions(eta);
CREATE INDEX idx_transactions_created_at ON transactions(created_at DESC);
CREATE INDEX idx_transactions_updated_at ON transactions(updated_at DESC);

-- 邮件通知配置表索引
CREATE INDEX idx_email_notifications_wallet_address ON email_notifications(wallet_address);
CREATE INDEX idx_email_notifications_email ON email_notifications(email);
CREATE INDEX idx_email_notifications_is_verified ON email_notifications(is_verified);
CREATE INDEX idx_email_notifications_is_active ON email_notifications(is_active);

-- 邮件发送记录表索引
CREATE INDEX idx_email_send_logs_email_notification_id ON email_send_logs(email_notification_id);
CREATE INDEX idx_email_send_logs_timelock_address ON email_send_logs(timelock_address);
CREATE INDEX idx_email_send_logs_transaction_hash ON email_send_logs(transaction_hash);
CREATE INDEX idx_email_send_logs_event_type ON email_send_logs(event_type);
CREATE INDEX idx_email_send_logs_is_emergency ON email_send_logs(is_emergency);
CREATE INDEX idx_email_send_logs_is_replied ON email_send_logs(is_replied);
CREATE INDEX idx_email_send_logs_send_status ON email_send_logs(send_status);
CREATE INDEX idx_email_send_logs_sent_at ON email_send_logs(sent_at DESC);

-- 应急通知追踪表索引
CREATE INDEX idx_emergency_notifications_timelock_address ON emergency_notifications(timelock_address);
CREATE INDEX idx_emergency_notifications_transaction_hash ON emergency_notifications(transaction_hash);
CREATE INDEX idx_emergency_notifications_is_completed ON emergency_notifications(is_completed);
CREATE INDEX idx_emergency_notifications_next_send_at ON emergency_notifications(next_send_at);

-- =============================================================================
-- 第四步：插入初始数据
-- =============================================================================

-- 插入共享ABI数据
INSERT INTO abis (name, abi_content, owner, description, is_shared) VALUES
('ERC20 Token', '[{"inputs":[{"internalType":"address","name":"spender","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"approve","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"account","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"decimals","outputs":[{"internalType":"uint8","name":"","type":"uint8"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"name","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"symbol","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"totalSupply","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"transfer","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"from","type":"address"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"transferFrom","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"owner","type":"address"},{"indexed":true,"internalType":"address","name":"spender","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}],"name":"Approval","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}],"name":"Transfer","type":"event"}]', '0x0000000000000000000000000000000000000000', 'Standard ERC-20 Token interface with basic functions for transferring tokens and checking balances.', true),

('ERC721 NFT', '[{"inputs":[{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"approve","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"getApproved","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"owner","type":"address"},{"internalType":"address","name":"operator","type":"address"}],"name":"isApprovedForAll","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"name","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"ownerOf","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"from","type":"address"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"safeTransferFrom","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"from","type":"address"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"},{"internalType":"bytes","name":"data","type":"bytes"}],"name":"safeTransferFrom","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"operator","type":"address"},{"internalType":"bool","name":"approved","type":"bool"}],"name":"setApprovalForAll","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes4","name":"interfaceId","type":"bytes4"}],"name":"supportsInterface","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"symbol","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"tokenURI","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"from","type":"address"},{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"transferFrom","outputs":[],"stateMutability":"nonpayable","type":"function"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"owner","type":"address"},{"indexed":true,"internalType":"address","name":"approved","type":"address"},{"indexed":true,"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"Approval","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"owner","type":"address"},{"indexed":true,"internalType":"address","name":"operator","type":"address"},{"indexed":false,"internalType":"bool","name":"approved","type":"bool"}],"name":"ApprovalForAll","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":true,"internalType":"uint256","name":"tokenId","type":"uint256"}],"name":"Transfer","type":"event"}]', '0x0000000000000000000000000000000000000000', 'Standard ERC-721 Non-Fungible Token interface with functions for managing unique tokens.', true),

('Uniswap V2 Pair', '[{"inputs":[],"name":"DOMAIN_SEPARATOR","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"MINIMUM_LIQUIDITY","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"PERMIT_TYPEHASH","outputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"to","type":"address"}],"name":"burn","outputs":[{"internalType":"uint256","name":"amount0","type":"uint256"},{"internalType":"uint256","name":"amount1","type":"uint256"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"decimals","outputs":[{"internalType":"uint8","name":"","type":"uint8"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"factory","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getReserves","outputs":[{"internalType":"uint112","name":"_reserve0","type":"uint112"},{"internalType":"uint112","name":"_reserve1","type":"uint112"},{"internalType":"uint32","name":"_blockTimestampLast","type":"uint32"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"_token0","type":"address"},{"internalType":"address","name":"_token1","type":"address"}],"name":"initialize","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"kLast","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"to","type":"address"}],"name":"mint","outputs":[{"internalType":"uint256","name":"liquidity","type":"uint256"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"name","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"","type":"address"}],"name":"nonces","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"price0CumulativeLast","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"price1CumulativeLast","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"to","type":"address"}],"name":"skim","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"amount0Out","type":"uint256"},{"internalType":"uint256","name":"amount1Out","type":"uint256"},{"internalType":"address","name":"to","type":"address"},{"internalType":"bytes","name":"data","type":"bytes"}],"name":"swap","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"symbol","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"sync","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"token0","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"token1","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"totalSupply","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]', '0x0000000000000000000000000000000000000000', 'Uniswap V2 trading pair contract interface for decentralized token swapping.', true),

('OpenZeppelin TimelockController', '[{"inputs":[{"internalType":"uint256","name":"minDelay","type":"uint256"},{"internalType":"address[]","name":"proposers","type":"address[]"},{"internalType":"address[]","name":"executors","type":"address[]"},{"internalType":"address","name":"admin","type":"address"}],"stateMutability":"nonpayable","type":"constructor"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"},{"internalType":"address","name":"account","type":"address"}],"name":"grantRole","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes32","name":"role","type":"bytes32"},{"internalType":"address","name":"account","type":"address"}],"name":"hasRole","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"isOperation","outputs":[{"internalType":"bool","name":"pending","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"isOperationDone","outputs":[{"internalType":"bool","name":"done","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"isOperationPending","outputs":[{"internalType":"bool","name":"pending","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"isOperationReady","outputs":[{"internalType":"bool","name":"ready","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"target","type":"address"},{"internalType":"uint256","name":"value","type":"uint256"},{"internalType":"bytes","name":"data","type":"bytes"},{"internalType":"bytes32","name":"predecessor","type":"bytes32"},{"internalType":"bytes32","name":"salt","type":"bytes32"}],"name":"execute","outputs":[],"stateMutability":"payable","type":"function"},{"inputs":[{"internalType":"address","name":"target","type":"address"},{"internalType":"uint256","name":"value","type":"uint256"},{"internalType":"bytes","name":"data","type":"bytes"},{"internalType":"bytes32","name":"predecessor","type":"bytes32"},{"internalType":"bytes32","name":"salt","type":"bytes32"},{"internalType":"uint256","name":"delay","type":"uint256"}],"name":"schedule","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"cancel","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"getMinDelay","outputs":[{"internalType":"uint256","name":"duration","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"getTimestamp","outputs":[{"internalType":"uint256","name":"timestamp","type":"uint256"}],"stateMutability":"view","type":"function"}]', '0x0000000000000000000000000000000000000000', 'OpenZeppelin TimelockController contract for time-delayed execution of governance proposals.', true);
