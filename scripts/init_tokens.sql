-- 初始化支持的代币数据
-- 这个脚本用于插入常见的代币信息到support_tokens表
-- run了服务之后用（服务会自动创建表）

INSERT INTO support_tokens (symbol, name, coingecko_id, decimals, is_active, created_at, updated_at) VALUES
    ('BTC', 'Bitcoin', 'bitcoin', 8, true, NOW(), NOW()),
    ('ETH', 'Ethereum', 'ethereum', 18, true, NOW(), NOW()),
    ('USDC', 'USD Coin', 'usd-coin', 6, true, NOW(), NOW()),
    ('USDT', 'Tether USD', 'tether', 6, true, NOW(), NOW()),
    ('BNB', 'Binance Coin', 'binancecoin', 18, true, NOW(), NOW()),
    ('MATIC', 'Polygon', 'matic-network', 18, true, NOW(), NOW()),
    ('LINK', 'Chainlink', 'chainlink', 18, true, NOW(), NOW()),
    ('UNI', 'Uniswap', 'uniswap', 18, true, NOW(), NOW()),
    ('WETH', 'Wrapped Ethereum', 'weth', 18, true, NOW(), NOW()),
    ('DAI', 'Dai Stablecoin', 'dai', 18, true, NOW(), NOW()),
    ('AAVE', 'Aave', 'aave', 18, true, NOW(), NOW()),
    ('CRV', 'Curve DAO Token', 'curve-dao-token', 18, true, NOW(), NOW()),
    ('COMP', 'Compound', 'compound-governance-token', 18, true, NOW(), NOW()),
    ('MKR', 'Maker', 'maker', 18, true, NOW(), NOW()),
    ('SNX', 'Synthetix', 'havven', 18, true, NOW(), NOW())
ON CONFLICT (symbol) DO UPDATE SET
    name = EXCLUDED.name,
    coingecko_id = EXCLUDED.coingecko_id,
    decimals = EXCLUDED.decimals,
    updated_at = NOW(); 