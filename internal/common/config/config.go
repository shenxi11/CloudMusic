package config

import (
	"fmt"
	"os"
	"strings"

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
	VideoHLSDir   string `yaml:"video_hls_dir"`
	FFmpegBinary  string `yaml:"ffmpeg_binary"`
	FFprobeBinary string `yaml:"ffprobe_binary"`
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

// JamendoExternalConfig configures the Jamendo external music provider.
type JamendoExternalConfig struct {
	Enabled      bool   `yaml:"enabled"`
	ClientID     string `yaml:"client_id"`
	BaseURL      string `yaml:"base_url"`
	TimeoutSec   int    `yaml:"timeout_sec"`
	DefaultLimit int    `yaml:"default_limit"`
}

// ExternalConfig configures third-party music providers.
type ExternalConfig struct {
	Jamendo JamendoExternalConfig `yaml:"jamendo"`
}

// AdminConfig 管理后台配置
type AdminConfig struct {
	Username          string `yaml:"username"`
	Password          string `yaml:"password"`
	SessionTTLMinutes int    `yaml:"session_ttl_minutes"`
}

// Config 总配置
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Event    EventConfig    `yaml:"event"`
	Schemas  SchemaConfig   `yaml:"schemas"`
	External ExternalConfig `yaml:"external"`
	Admin    AdminConfig    `yaml:"admin"`
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

// ResolveJamendoConfig applies defaults and environment overrides.
func ResolveJamendoConfig(cfg *Config) JamendoExternalConfig {
	jamendo := JamendoExternalConfig{
		Enabled:      true,
		BaseURL:      "https://api.jamendo.com/v3.0",
		TimeoutSec:   8,
		DefaultLimit: 20,
	}
	if cfg != nil {
		configured := cfg.External.Jamendo
		jamendo.Enabled = configured.Enabled
		if strings.TrimSpace(configured.ClientID) != "" {
			jamendo.ClientID = strings.TrimSpace(configured.ClientID)
		}
		if strings.TrimSpace(configured.BaseURL) != "" {
			jamendo.BaseURL = strings.TrimRight(strings.TrimSpace(configured.BaseURL), "/")
		}
		if configured.TimeoutSec > 0 {
			jamendo.TimeoutSec = configured.TimeoutSec
		}
		if configured.DefaultLimit > 0 {
			jamendo.DefaultLimit = configured.DefaultLimit
		}
	}

	if envClientID := strings.TrimSpace(os.Getenv("JAMENDO_CLIENT_ID")); envClientID != "" {
		jamendo.ClientID = envClientID
		jamendo.Enabled = true
	}

	return jamendo
}
