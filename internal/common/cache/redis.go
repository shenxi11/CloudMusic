package cache

import (
	"context"
	"fmt"
	"time"

	"music-platform/internal/common/config"

	"github.com/go-redis/redis/v8"
)

var (
	rdb *redis.Client
	ctx = context.Background()
)

// Cache TTL常量
const (
	TTLShort  = 5 * time.Minute  // 短期缓存
	TTLMedium = 30 * time.Minute // 中期缓存
	TTLLong   = 2 * time.Hour    // 长期缓存
)

// Cache key前缀
const (
	PrefixUser  = "user:"
	PrefixMusic = "music:"
	PrefixStats = "stats:"
)

// Init 初始化Redis连接
func Init(cfg *config.RedisConfig) error {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	rdb = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.Password,
		DB:       0,
	})

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("Redis连接失败: %w", err)
	}

	return nil
}

// GetClient 获取Redis客户端
func GetClient() *redis.Client {
	return rdb
}

// GetContext 获取全局context
func GetContext() context.Context {
	return ctx
}

// Close 关闭Redis连接
func Close() error {
	if rdb != nil {
		return rdb.Close()
	}
	return nil
}

// Set 设置缓存
func Set(key string, value interface{}, ttl time.Duration) error {
	return rdb.Set(ctx, key, value, ttl).Err()
}

// Get 获取缓存
func Get(key string) (string, error) {
	return rdb.Get(ctx, key).Result()
}

// Del 删除缓存
func Del(keys ...string) error {
	return rdb.Del(ctx, keys...).Err()
}

// FlushAll 清空所有缓存
func FlushAll() error {
	return rdb.FlushAll(ctx).Err()
}
