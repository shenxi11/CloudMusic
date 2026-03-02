package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// ServerConfig 服务器配置
type ServerConfig struct {
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"`
	PublicHost    string `yaml:"public_host"`
	PublicPort    int    `yaml:"public_port"`
	PublicBaseURL string `yaml:"public_base_url"`
	UploadDir     string `yaml:"upload_dir"`
	StaticDir     string `yaml:"static_dir"`
	VideoDir      string `yaml:"video_dir"`
	// TLS/HTTPS 配置
	EnableTLS bool   `yaml:"enable_tls"` // 是否启用 HTTPS
	CertFile  string `yaml:"cert_file"`  // TLS 证书文件路径
	KeyFile   string `yaml:"key_file"`   // TLS 私钥文件路径
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
}

// EventOutboxConfig 事件 outbox 配置
type EventOutboxConfig struct {
	PollIntervalMs   int `yaml:"poll_interval_ms"`
	BatchSize        int `yaml:"batch_size"`
	MaxRetry         int `yaml:"max_retry"`
	RetryBaseDelayMs int `yaml:"retry_base_delay_ms"`
}

// EventWorkerConfig 事件消费器配置
type EventWorkerConfig struct {
	MaxRetry     int `yaml:"max_retry"`
	RetryDelayMs int `yaml:"retry_delay_ms"`
}

// EventBusConfig 事件总线配置
type EventBusConfig struct {
	Stream           string `yaml:"stream"`
	Group            string `yaml:"group"`
	Consumer         string `yaml:"consumer"`
	PendingMinIdleMs int    `yaml:"pending_min_idle_ms"`
	PendingBatchSize int    `yaml:"pending_batch_size"`
}

// EventConfig 事件系统配置
type EventConfig struct {
	Bus    EventBusConfig    `yaml:"bus"`
	Outbox EventOutboxConfig `yaml:"outbox"`
	Worker EventWorkerConfig `yaml:"worker"`
}

// SchemaConfig 领域数据 schema 配置
type SchemaConfig struct {
	Profile string `yaml:"profile"`
	Catalog string `yaml:"catalog"`
	Media   string `yaml:"media"`
}

// Config 总配置
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Event    EventConfig    `yaml:"event"`
	Schemas  SchemaConfig   `yaml:"schemas"`
}

var globalConfig *Config

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	globalConfig = &cfg
	return &cfg, nil
}

// Get 获取全局配置
func Get() *Config {
	return globalConfig
}

// MustLoad 加载配置，失败则panic
func MustLoad(configPath string) *Config {
	cfg, err := Load(configPath)
	if err != nil {
		panic(err)
	}
	return cfg
}
