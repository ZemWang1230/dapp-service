package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	RPC      RPCConfig      `mapstructure:"rpc"`
	Email    EmailConfig    `mapstructure:"email"`
	Scanner  ScannerConfig  `mapstructure:"scanner"`
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

// RPCConfig RPC配置
type RPCConfig struct {
	AlchemyAPIKey   string `mapstructure:"alchemy_api_key"`
	InfuraAPIKey    string `mapstructure:"infura_api_key"`
	Provider        string `mapstructure:"provider"`
	IncludeTestnets bool   `mapstructure:"include_testnets"`
}

// EmailConfig 邮件配置
type EmailConfig struct {
	SMTPHost               string        `mapstructure:"smtp_host"`
	SMTPPort               int           `mapstructure:"smtp_port"`
	SMTPUsername           string        `mapstructure:"smtp_username"`
	SMTPPassword           string        `mapstructure:"smtp_password"`
	FromName               string        `mapstructure:"from_name"`
	FromEmail              string        `mapstructure:"from_email"`
	VerificationCodeExpiry time.Duration `mapstructure:"verification_code_expiry"`
	EmailURL               string        `mapstructure:"email_url"`
}

// ScannerConfig 扫链配置
type ScannerConfig struct {
	// RPC配置
	RPCTimeout    time.Duration `mapstructure:"rpc_timeout"`
	RPCRetryMax   int           `mapstructure:"rpc_retry_max"`
	RPCRetryDelay time.Duration `mapstructure:"rpc_retry_delay"`

	// 扫块配置
	ScanBatchSize     int           `mapstructure:"scan_batch_size"`
	ScanInterval      time.Duration `mapstructure:"scan_interval"`
	ScanIntervalSlow  time.Duration `mapstructure:"scan_interval_slow"`
	ScanConfirmations int           `mapstructure:"scan_confirmations"`

	// Flow refresher config
	FlowRefreshInterval  time.Duration `mapstructure:"flow_refresh_interval"`
	FlowRefreshBatchSize int           `mapstructure:"flow_refresh_batch_size"`
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

	// Email defaults
	viper.SetDefault("email.smtp_host", "smtp.gmail.com")
	viper.SetDefault("email.smtp_port", 587)
	viper.SetDefault("email.smtp_username", "")
	viper.SetDefault("email.smtp_password", "")
	viper.SetDefault("email.from_name", "TimeLocker Notification")
	viper.SetDefault("email.from_email", "")
	viper.SetDefault("email.verification_code_expiry", time.Minute*10)
	viper.SetDefault("email.email_url", "http://localhost:8080")

	// Scanner defaults
	viper.SetDefault("scanner.rpc_timeout", time.Second*10)
	viper.SetDefault("scanner.rpc_retry_max", 3)
	viper.SetDefault("scanner.rpc_retry_delay", time.Second*1)
	viper.SetDefault("scanner.scan_batch_size", 500)
	viper.SetDefault("scanner.scan_interval", time.Second*5)
	viper.SetDefault("scanner.scan_interval_slow", time.Second*30)
	viper.SetDefault("scanner.scan_confirmations", 12)
	viper.SetDefault("scanner.flow_refresh_interval", time.Second*60)

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

// GetRPCURL 根据链RPC信息获取RPC URL
func (c *Config) GetRPCURL(chainInfo *types.ChainRPCInfo) (string, error) {
	if !chainInfo.RPCEnabled {
		return "", errors.New("RPC disabled for chain: " + chainInfo.ChainName)
	}

	// 根据配置的提供商选择RPC
	switch c.RPC.Provider {
	case "alchemy":
		if chainInfo.AlchemyRPCTemplate == nil {
			logger.Error("GetRPCURL error: ", fmt.Errorf("alchemy RPC not supported for chain: %s", chainInfo.ChainName))
			return "", errors.New("alchemy RPC not supported for chain: " + chainInfo.ChainName)
		}
		if c.RPC.AlchemyAPIKey == "" || c.RPC.AlchemyAPIKey == "YOUR_ALCHEMY_API_KEY" {
			logger.Error("GetRPCURL error: ", fmt.Errorf("alchemy API key not configured"))
			return "", errors.New("alchemy API key not configured")
		}
		return strings.Replace(*chainInfo.AlchemyRPCTemplate, "{API_KEY}", c.RPC.AlchemyAPIKey, 1), nil

	case "infura":
		if chainInfo.InfuraRPCTemplate == nil {
			logger.Error("GetRPCURL error: ", fmt.Errorf("infura RPC not supported for chain: %s", chainInfo.ChainName))
			return "", errors.New("infura RPC not supported for chain: " + chainInfo.ChainName)
		}
		if c.RPC.InfuraAPIKey == "" || c.RPC.InfuraAPIKey == "YOUR_INFURA_API_KEY" {
			logger.Error("GetRPCURL error: ", fmt.Errorf("infura API key not configured"))
			return "", errors.New("infura API key not configured")
		}
		return strings.Replace(*chainInfo.InfuraRPCTemplate, "{API_KEY}", c.RPC.InfuraAPIKey, 1), nil

	default:
		logger.Error("GetRPCURL error: ", fmt.Errorf("unsupported RPC provider: %s", c.RPC.Provider))
		return "", fmt.Errorf("unsupported RPC provider: %s", c.RPC.Provider)
	}
}
