package config

import (
	"errors"
	"time"
	"timelocker-backend/pkg/logger"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Price    PriceConfig    `mapstructure:"price"`
	RPC      RPCConfig      `mapstructure:"rpc"`
	Asset    AssetConfig    `mapstructure:"asset"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret        string        `mapstructure:"secret"`
	AccessExpiry  time.Duration `mapstructure:"access_expiry"`
	RefreshExpiry time.Duration `mapstructure:"refresh_expiry"`
}

type PriceConfig struct {
	Provider       string        `mapstructure:"provider"`
	APIKey         string        `mapstructure:"api_key"`
	BaseURL        string        `mapstructure:"base_url"`
	UpdateInterval time.Duration `mapstructure:"update_interval"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
	CachePrefix    string        `mapstructure:"cache_prefix"`
}

type RPCProviderConfig struct {
	APIKey   string `mapstructure:"api_key"`
	Ethereum string `mapstructure:"ethereum"`
	BSC      string `mapstructure:"bsc"`
	Polygon  string `mapstructure:"polygon"`
	Arbitrum string `mapstructure:"arbitrum"`
}

type RPCConfig struct {
	Provider       string            `mapstructure:"provider"`
	Alchemy        RPCProviderConfig `mapstructure:"alchemy"`
	Infura         RPCProviderConfig `mapstructure:"infura"`
	RequestTimeout time.Duration     `mapstructure:"request_timeout"`
}

type AssetConfig struct {
	UpdateInterval time.Duration `mapstructure:"update_interval"`
	BatchSize      int           `mapstructure:"batch_size"`
	CachePrefix    string        `mapstructure:"cache_prefix"`
	MaxRetry       int           `mapstructure:"max_retry"`
	RetryDelay     time.Duration `mapstructure:"retry_delay"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Set defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "timelocker")
	viper.SetDefault("database.password", "timelocker")
	viper.SetDefault("database.dbname", "timelocker_db")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("jwt.secret", "timelocker-jwt-secret-v1")
	viper.SetDefault("jwt.access_expiry", time.Hour*24)
	viper.SetDefault("jwt.refresh_expiry", time.Hour*24*7)
	viper.SetDefault("price.provider", "coingecko")
	viper.SetDefault("price.api_key", "")
	viper.SetDefault("price.base_url", "https://api.coingecko.com/api/v3")
	viper.SetDefault("price.update_interval", time.Second*30)
	viper.SetDefault("price.request_timeout", time.Second*10)
	viper.SetDefault("price.cache_prefix", "price:")

	// RPC defaults
	viper.SetDefault("rpc.provider", "alchemy")
	viper.SetDefault("rpc.request_timeout", time.Second*30)
	viper.SetDefault("rpc.alchemy.api_key", "")
	viper.SetDefault("rpc.alchemy.ethereum", "https://eth-mainnet.g.alchemy.com/v2/")
	viper.SetDefault("rpc.alchemy.bsc", "https://bnb-mainnet.g.alchemy.com/v2/")
	viper.SetDefault("rpc.alchemy.polygon", "https://polygon-mainnet.g.alchemy.com/v2/")
	viper.SetDefault("rpc.alchemy.arbitrum", "https://arb-mainnet.g.alchemy.com/v2/")
	viper.SetDefault("rpc.infura.api_key", "")
	viper.SetDefault("rpc.infura.ethereum", "https://mainnet.infura.io/v3/")
	viper.SetDefault("rpc.infura.bsc", "https://bsc-dataseed.binance.org/")
	viper.SetDefault("rpc.infura.polygon", "https://polygon-mainnet.infura.io/v3/")
	viper.SetDefault("rpc.infura.arbitrum", "https://arbitrum-mainnet.infura.io/v3/")

	// Asset defaults
	viper.SetDefault("asset.update_interval", time.Second*30)
	viper.SetDefault("asset.batch_size", 10)
	viper.SetDefault("asset.cache_prefix", "asset:")
	viper.SetDefault("asset.max_retry", 3)
	viper.SetDefault("asset.retry_delay", time.Second*5)

	// Read environment variables
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			logger.Error("LoadConfig Error: ", errors.New("config file not found"), "error: ", err)
			return nil, err
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		logger.Error("LoadConfig Error: ", errors.New("failed to unmarshal config"), "error: ", err)
		return nil, err
	}

	logger.Info("LoadConfig: ", "load config success")
	return &config, nil
}
