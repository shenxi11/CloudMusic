package eventbus

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"music-platform/internal/common/config"

	"github.com/go-redis/redis/v8"
)

const (
	// DefaultChannel 兼容旧命名，实际用于 Redis Stream 名称
	DefaultChannel = "music.domain.events.v1"
	// DefaultConsumerGroup 默认消费者组
	DefaultConsumerGroup = "music-domain-events-group"
	// DefaultReadCount 单次读取条数
	DefaultReadCount = int64(50)
	// DefaultReadBlock 阻塞读取时间
	DefaultReadBlock = 2 * time.Second
	// DefaultPendingMinIdle 自动接管 pending 的最小空闲时间
	DefaultPendingMinIdle = 30 * time.Second
	// DefaultPendingBatchSize 单次接管 pending 数量
	DefaultPendingBatchSize = int64(50)
)

var eventSeq uint64

// Event 领域事件结构
type Event struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Source    string          `json:"source"`
	Version   int             `json:"version"`
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// Publisher 事件发布接口
type Publisher interface {
	Publish(ctx context.Context, evt *Event) error
	Close() error
}

// Subscriber 事件订阅接口
type Subscriber interface {
	Consume(ctx context.Context, handler func(*Event) error) error
	Close() error
}

// RedisPublisher Redis Stream 发布实现
type RedisPublisher struct {
	client *redis.Client
	stream string
}

// RedisSubscriber Redis Stream 订阅实现
type RedisSubscriber struct {
	client   *redis.Client
	stream   string
	group    string
	consumer string

	claimMinIdle time.Duration
	claimCount   int64
}

func newRedisClient(cfg *config.RedisConfig) *redis.Client {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.Password,
		DB:       0,
	})
}

func normalizeStream(stream string) string {
	if strings.TrimSpace(stream) == "" {
		return DefaultChannel
	}
	return stream
}

func normalizeGroup(group string) string {
	if strings.TrimSpace(group) == "" {
		return DefaultConsumerGroup
	}
	return group
}

func newConsumerName(name string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}

	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "unknown-host"
	}
	return fmt.Sprintf("%s-%d-%d", host, os.Getpid(), time.Now().UnixNano())
}

// NewEvent 创建标准领域事件
func NewEvent(eventType, source string, payload interface{}) (*Event, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化事件数据失败: %w", err)
	}

	now := time.Now().UnixMilli()
	evt := &Event{
		ID:        newEventID(eventType),
		Type:      eventType,
		Source:    source,
		Version:   1,
		Timestamp: now,
		Data:      data,
	}
	return evt, nil
}

// NewRedisPublisher 创建 Redis Stream 发布器
func NewRedisPublisher(cfg *config.RedisConfig, stream string) (*RedisPublisher, error) {
	client := newRedisClient(cfg)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("Redis连接失败: %w", err)
	}

	return &RedisPublisher{
		client: client,
		stream: normalizeStream(stream),
	}, nil
}

// Publish 发布事件到 Redis Stream
func (p *RedisPublisher) Publish(ctx context.Context, evt *Event) error {
	if evt == nil {
		return fmt.Errorf("事件不能为空")
	}
	if evt.ID == "" {
		evt.ID = newEventID(evt.Type)
	}
	if evt.Version == 0 {
		evt.Version = 1
	}
	if evt.Timestamp == 0 {
		evt.Timestamp = time.Now().UnixMilli()
	}

	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("序列化事件失败: %w", err)
	}

	args := &redis.XAddArgs{
		Stream: p.stream,
		Values: map[string]interface{}{
			"event": string(payload),
		},
	}
	if err := p.client.XAdd(ctx, args).Err(); err != nil {
		return fmt.Errorf("发布事件失败: %w", err)
	}
	return nil
}

func newEventID(prefix string) string {
	seq := atomic.AddUint64(&eventSeq, 1)
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), seq)
	}
	return fmt.Sprintf("%s-%d-%d-%d", prefix, time.Now().UnixNano(), seq, n.Int64())
}

// Close 关闭发布器
func (p *RedisPublisher) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

// NewRedisSubscriber 使用默认消费者组创建订阅器
func NewRedisSubscriber(cfg *config.RedisConfig, stream string) (*RedisSubscriber, error) {
	return NewRedisSubscriberWithGroup(cfg, stream, DefaultConsumerGroup, "")
}

// NewRedisSubscriberWithGroup 创建 Redis Stream 订阅器
func NewRedisSubscriberWithGroup(cfg *config.RedisConfig, stream, group, consumer string) (*RedisSubscriber, error) {
	client := newRedisClient(cfg)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("Redis连接失败: %w", err)
	}

	sub := &RedisSubscriber{
		client:       client,
		stream:       normalizeStream(stream),
		group:        normalizeGroup(group),
		consumer:     newConsumerName(consumer),
		claimMinIdle: DefaultPendingMinIdle,
		claimCount:   DefaultPendingBatchSize,
	}
	if err := sub.ensureGroup(context.Background()); err != nil {
		_ = client.Close()
		return nil, err
	}
	return sub, nil
}

// ConfigureAutoClaim 配置 pending 消息自动接管
func (s *RedisSubscriber) ConfigureAutoClaim(minIdle time.Duration, count int64) {
	if minIdle <= 0 {
		s.claimMinIdle = 0
	} else {
		s.claimMinIdle = minIdle
	}

	if count <= 0 {
		s.claimCount = DefaultPendingBatchSize
	} else {
		s.claimCount = count
	}
}

func (s *RedisSubscriber) ensureGroup(ctx context.Context) error {
	err := s.client.XGroupCreateMkStream(ctx, s.stream, s.group, "$").Err()
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return fmt.Errorf("创建消费者组失败: %w", err)
}

// Consume 持续消费事件（成功处理后 ACK）
func (s *RedisSubscriber) Consume(ctx context.Context, handler func(*Event) error) error {
	if handler == nil {
		return fmt.Errorf("handler不能为空")
	}

	for {
		if ctx.Err() != nil {
			return nil
		}

		if err := s.reclaimPending(ctx, handler); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("接管 pending 消息失败: %w", err)
		}

		streams, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    s.group,
			Consumer: s.consumer,
			Streams:  []string{s.stream, ">"},
			Count:    DefaultReadCount,
			Block:    DefaultReadBlock,
		}).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("读取流消息失败: %w", err)
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				if err := s.handleMessage(ctx, msg, handler); err != nil {
					return fmt.Errorf("ACK 失败: %w", err)
				}
			}
		}
	}
}

func (s *RedisSubscriber) reclaimPending(ctx context.Context, handler func(*Event) error) error {
	if s.claimMinIdle <= 0 {
		return nil
	}

	ids, err := s.pendingIDs(ctx)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}

	messages, err := s.claimByIDs(ctx, ids)
	if err != nil {
		return err
	}
	if len(messages) == 0 {
		return nil
	}

	for _, msg := range messages {
		if err := s.handleMessage(ctx, msg, handler); err != nil {
			return err
		}
	}
	return nil
}

func (s *RedisSubscriber) pendingIDs(ctx context.Context) ([]string, error) {
	minIdleMs := int64(s.claimMinIdle / time.Millisecond)
	if minIdleMs < 0 {
		minIdleMs = 0
	}

	result, err := s.client.Do(
		ctx,
		"XPENDING",
		s.stream,
		s.group,
		"IDLE",
		minIdleMs,
		"-",
		"+",
		s.claimCount,
	).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	items, ok := result.([]interface{})
	if !ok {
		return nil, nil
	}

	ids := make([]string, 0, len(items))
	for _, item := range items {
		fields, ok := item.([]interface{})
		if !ok || len(fields) == 0 {
			continue
		}
		id := asString(fields[0])
		if strings.TrimSpace(id) == "" {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *RedisSubscriber) claimByIDs(ctx context.Context, ids []string) ([]redis.XMessage, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	minIdleMs := int64(s.claimMinIdle / time.Millisecond)
	if minIdleMs < 0 {
		minIdleMs = 0
	}

	args := make([]interface{}, 0, 5+len(ids))
	args = append(args, "XCLAIM", s.stream, s.group, s.consumer, minIdleMs)
	for _, id := range ids {
		args = append(args, id)
	}

	result, err := s.client.Do(ctx, args...).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	rawMsgs, ok := result.([]interface{})
	if !ok {
		return nil, nil
	}

	messages := make([]redis.XMessage, 0, len(rawMsgs))
	for _, raw := range rawMsgs {
		entry, ok := raw.([]interface{})
		if !ok || len(entry) < 2 {
			continue
		}

		msg := redis.XMessage{
			ID:     asString(entry[0]),
			Values: map[string]interface{}{},
		}

		pairs, ok := entry[1].([]interface{})
		if !ok {
			messages = append(messages, msg)
			continue
		}
		for i := 0; i+1 < len(pairs); i += 2 {
			key := asString(pairs[i])
			if strings.TrimSpace(key) == "" {
				continue
			}
			msg.Values[key] = pairs[i+1]
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func asString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(v)
	}
}

func (s *RedisSubscriber) handleMessage(ctx context.Context, msg redis.XMessage, handler func(*Event) error) error {
	evt, decodeErr := decodeEvent(msg.Values)
	if decodeErr != nil {
		return s.client.XAck(ctx, s.stream, s.group, msg.ID).Err()
	}

	if err := handler(evt); err != nil {
		// 处理失败时不 ACK，保持在 PEL，后续可通过 reclaim 重试
		return nil
	}

	return s.client.XAck(ctx, s.stream, s.group, msg.ID).Err()
}

func decodeEvent(values map[string]interface{}) (*Event, error) {
	raw, ok := values["event"]
	if !ok {
		return nil, fmt.Errorf("缺少 event 字段")
	}

	data, err := valueToBytes(raw)
	if err != nil {
		return nil, err
	}

	var evt Event
	if err := json.Unmarshal(data, &evt); err != nil {
		return nil, err
	}
	return &evt, nil
}

func valueToBytes(v interface{}) ([]byte, error) {
	switch t := v.(type) {
	case string:
		return []byte(t), nil
	case []byte:
		return t, nil
	case json.RawMessage:
		return []byte(t), nil
	default:
		data, err := json.Marshal(t)
		if err != nil {
			return nil, fmt.Errorf("序列化事件字段失败: %w", err)
		}
		return data, nil
	}
}

// Close 关闭订阅器
func (s *RedisSubscriber) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}
