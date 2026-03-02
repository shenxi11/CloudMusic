package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"music-platform/internal/common/config"
	"music-platform/internal/common/database"
	"music-platform/internal/common/eventbus"
	"music-platform/internal/common/logger"
)

var errMovedToDLQ = errors.New("moved_to_dlq")

func ensureEventTables(db *sql.DB) error {
	domainEvents := `
	CREATE TABLE IF NOT EXISTS domain_events (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		event_id VARCHAR(128) NOT NULL,
		event_type VARCHAR(128) NOT NULL,
		event_source VARCHAR(64) NOT NULL,
		payload JSON NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE KEY uk_event_id (event_id),
		KEY idx_type_created (event_type, created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`
	if _, err := db.Exec(domainEvents); err != nil {
		return err
	}

	domainEventDLQ := `
	CREATE TABLE IF NOT EXISTS domain_event_dlq (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		event_id VARCHAR(128) NOT NULL,
		event_type VARCHAR(128) NOT NULL,
		event_source VARCHAR(64) NOT NULL,
		payload JSON NOT NULL,
		retry_count INT NOT NULL,
		last_error VARCHAR(512) NOT NULL,
		failed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		UNIQUE KEY uk_event_id (event_id),
		KEY idx_failed_at (failed_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`
	_, err := db.Exec(domainEventDLQ)
	return err
}

func insertEvent(db *sql.DB, evt *eventbus.Event) error {
	query := `
	INSERT INTO domain_events (event_id, event_type, event_source, payload)
	VALUES (?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE event_id = event_id
	`
	_, err := db.Exec(query, evt.ID, evt.Type, evt.Source, string(evt.Data))
	return err
}

func insertDLQ(db *sql.DB, evt *eventbus.Event, retryCount int, lastErr error) error {
	query := `
	INSERT INTO domain_event_dlq (event_id, event_type, event_source, payload, retry_count, last_error)
	VALUES (?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE
		retry_count = VALUES(retry_count),
		last_error = VALUES(last_error),
		updated_at = NOW()
	`
	_, err := db.Exec(
		query,
		evt.ID,
		evt.Type,
		evt.Source,
		string(evt.Data),
		retryCount,
		truncateError(lastErr),
	)
	return err
}

func processEventWithRetry(db *sql.DB, evt *eventbus.Event, maxRetry int, retryDelay time.Duration) error {
	if maxRetry <= 0 {
		maxRetry = 3
	}
	if retryDelay <= 0 {
		retryDelay = 300 * time.Millisecond
	}

	var err error
	for attempt := 1; attempt <= maxRetry; attempt++ {
		err = insertEvent(db, evt)
		if err == nil {
			return nil
		}
		if attempt < maxRetry {
			time.Sleep(retryDelay * time.Duration(attempt))
		}
	}

	if dlqErr := insertDLQ(db, evt, maxRetry, err); dlqErr != nil {
		return fmt.Errorf("事件处理失败，且写入 DLQ 失败: raw_err=%v dlq_err=%w", err, dlqErr)
	}
	return fmt.Errorf("%w: %v", errMovedToDLQ, err)
}

func truncateError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if len(msg) <= 512 {
		return msg
	}
	return msg[:512]
}

func main() {
	if err := logger.Init("event_worker.log"); err != nil {
		fmt.Printf("事件消费器日志初始化失败: %v\n", err)
		os.Exit(1)
	}
	logger.Info("事件消费器启动中...")

	cfg := config.MustLoad("configs/config.yaml")
	if err := database.Init(&cfg.Database); err != nil {
		logger.Fatal("事件消费器数据库初始化失败: %v", err)
	}
	defer database.Close()

	db := database.GetDB()
	if err := ensureEventTables(db); err != nil {
		logger.Fatal("初始化事件表失败: %v", err)
	}

	maxRetry := cfg.Event.Worker.MaxRetry
	if maxRetry <= 0 {
		maxRetry = 3
	}
	retryDelay := time.Duration(cfg.Event.Worker.RetryDelayMs) * time.Millisecond
	if retryDelay <= 0 {
		retryDelay = 300 * time.Millisecond
	}

	streamName := strings.TrimSpace(cfg.Event.Bus.Stream)
	if streamName == "" {
		streamName = eventbus.DefaultChannel
	}
	groupName := strings.TrimSpace(cfg.Event.Bus.Group)
	if groupName == "" {
		groupName = eventbus.DefaultConsumerGroup
	}
	consumerName := strings.TrimSpace(cfg.Event.Bus.Consumer)
	pendingMinIdleMs := cfg.Event.Bus.PendingMinIdleMs
	if pendingMinIdleMs <= 0 {
		pendingMinIdleMs = 30000
	}
	pendingBatchSize := cfg.Event.Bus.PendingBatchSize
	if pendingBatchSize <= 0 {
		pendingBatchSize = 50
	}

	subscriber, err := eventbus.NewRedisSubscriberWithGroup(&cfg.Redis, streamName, groupName, consumerName)
	if err != nil {
		logger.Fatal("初始化事件订阅器失败: %v", err)
	}
	subscriber.ConfigureAutoClaim(time.Duration(pendingMinIdleMs)*time.Millisecond, int64(pendingBatchSize))
	defer subscriber.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info("事件消费器已订阅流: %s (group=%s, pending_min_idle=%dms)", streamName, groupName, pendingMinIdleMs)
	if err := subscriber.Consume(ctx, func(evt *eventbus.Event) error {
		if evt == nil {
			return nil
		}
		if err := processEventWithRetry(db, evt, maxRetry, retryDelay); err != nil {
			if errors.Is(err, errMovedToDLQ) {
				logger.Error("事件处理失败并已写入DLQ type=%s id=%s err=%v", evt.Type, evt.ID, err)
				return nil
			}
			logger.Error("事件处理失败（将保留 pending 等待重试） type=%s id=%s err=%v", evt.Type, evt.ID, err)
			return err
		}
		logger.Info("事件入库成功 type=%s id=%s source=%s", evt.Type, evt.ID, evt.Source)
		return nil
	}); err != nil {
		logger.Fatal("事件消费循环异常退出: %v", err)
	}
}
