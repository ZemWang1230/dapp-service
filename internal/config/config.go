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

// bindEnvKeys 把所有支持环境变量覆盖的 key 都显式绑定一次。
// 这样的好处：1) 不依赖 yaml 中是否出现该字段；2) 在文档/代码里集中可见。
func bindEnvKeys() {
	keys := []string{
		// server
		"server.port", "server.mode",
		// database
		"database.host", "database.port", "database.user", "database.password", "database.dbname", "database.sslmode",
		// redis
		"redis.host", "redis.port", "redis.password", "redis.db",
		// jwt
		"jwt.secret", "jwt.access_expiry", "jwt.refresh_expiry",
		// rpc
		"rpc.alchemy_api_key", "rpc.infura_api_key", "rpc.provider", "rpc.include_testnets",
		// email
		"email.smtp_host", "email.smtp_port", "email.smtp_username", "email.smtp_password",
		"email.from_name", "email.from_email", "email.verification_code_expiry", "email.email_url",
		// timelock 调度
		"timelock.refresh_interval", "timelock.refresh_concurrency",
		// goldsky 调度
		"goldsky.sync_interval", "goldsky.status_check_interval", "goldsky.sync_page_size",
		// notification worker 池
		"notification.worker_count", "notification.queue_buffer",
	}
	for _, k := range keys {
		_ = viper.BindEnv(k)
	}
}

// Config 应用配置
type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Redis        RedisConfig        `mapstructure:"redis"`
	JWT          JWTConfig          `mapstructure:"jwt"`
	RPC          RPCConfig          `mapstructure:"rpc"`
	Email        EmailConfig        `mapstructure:"email"`
	Timelock     TimelockConfig     `mapstructure:"timelock"`
	Goldsky      GoldskyConfig      `mapstructure:"goldsky"`
	Notification NotificationConfig `mapstructure:"notification"`
}

// TimelockConfig Timelock 刷新任务相关配置
type TimelockConfig struct {
	// 定时全量刷新链上 Timelock 元数据的间隔
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
	// 刷新时每个链上最多的并发 RPC 调用数
	RefreshConcurrency int `mapstructure:"refresh_concurrency"`
}

// GoldskyConfig Goldsky 同步 / 状态检查相关配置
type GoldskyConfig struct {
	// 从 Goldsky subgraph 全量拉取 flow 的轮询间隔
	SyncInterval time.Duration `mapstructure:"sync_interval"`
	// 本地基于 eta/expired_at 推进 flow 状态的轮询间隔
	StatusCheckInterval time.Duration `mapstructure:"status_check_interval"`
	// 单次同步 flow 时分页大小
	SyncPageSize int `mapstructure:"sync_page_size"`
}

// NotificationConfig 通知发送相关配置
type NotificationConfig struct {
	// 状态变化通知 worker 数量
	WorkerCount int `mapstructure:"worker_count"`
	// 状态变化通知队列 buffer 大小
	QueueBuffer int `mapstructure:"queue_buffer"`
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
	viper.SetDefault("email.from_name", "Timelock Notification")
	viper.SetDefault("email.from_email", "")
	viper.SetDefault("email.verification_code_expiry", time.Minute*10)
	viper.SetDefault("email.email_url", "http://localhost:8080")

	// Timelock refresh defaults
	viper.SetDefault("timelock.refresh_interval", 2*time.Hour)
	viper.SetDefault("timelock.refresh_concurrency", 5)

	// Goldsky defaults
	viper.SetDefault("goldsky.sync_interval", 10*time.Minute)
	viper.SetDefault("goldsky.status_check_interval", 30*time.Second)
	viper.SetDefault("goldsky.sync_page_size", 500)

	// Notification defaults
	viper.SetDefault("notification.worker_count", 4)
	viper.SetDefault("notification.queue_buffer", 1024)

	// 让嵌套 key 能从环境变量读取：database.host -> DATABASE_HOST 等。
	// 这样 .env / docker-compose 注入的环境变量会自动覆盖 config.yaml 里的同名字段。
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// AutomaticEnv 对未在 yaml 里出现的字段不会兜底，所以对所有结构体字段
	// 显式 BindEnv 一次，确保即使 yaml 缺这一项，env 仍然能注入进来。
	bindEnvKeys()

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
