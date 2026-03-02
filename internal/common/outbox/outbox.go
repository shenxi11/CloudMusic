package outbox

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"music-platform/internal/common/eventbus"
)

const (
	StatusPending = 0
	StatusSent    = 1
	StatusDead    = 2
)

type Record struct {
	ID         int64
	Event      *eventbus.Event
	RetryCount int
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) EnsureTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS event_outbox (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		event_id VARCHAR(128) NOT NULL,
		event_type VARCHAR(128) NOT NULL,
		event_source VARCHAR(64) NOT NULL,
		event_version INT NOT NULL DEFAULT 1,
		event_timestamp BIGINT NOT NULL,
		payload JSON NOT NULL,
		status TINYINT NOT NULL DEFAULT 0,
		retry_count INT NOT NULL DEFAULT 0,
		next_retry_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_error VARCHAR(512) NOT NULL DEFAULT '',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		published_at TIMESTAMP NULL DEFAULT NULL,
		UNIQUE KEY uk_event_id (event_id),
		KEY idx_status_retry (status, next_retry_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`
	_, err := s.db.Exec(query)
	if err != nil {
		return fmt.Errorf("创建 event_outbox 表失败: %w", err)
	}
	return nil
}

func (s *Store) SavePending(evt *eventbus.Event, reason string) error {
	if evt == nil {
		return fmt.Errorf("事件不能为空")
	}

	query := `
	INSERT INTO event_outbox (
		event_id, event_type, event_source, event_version, event_timestamp,
		payload, status, retry_count, next_retry_at, last_error
	)
	VALUES (?, ?, ?, ?, ?, ?, ?, 0, NOW(), ?)
	ON DUPLICATE KEY UPDATE
		updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(
		query,
		evt.ID,
		evt.Type,
		evt.Source,
		evt.Version,
		evt.Timestamp,
		string(evt.Data),
		StatusPending,
		truncateError(reason),
	)
	if err != nil {
		return fmt.Errorf("写入 event_outbox 失败: %w", err)
	}
	return nil
}

func (s *Store) FetchPending(limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
	SELECT id, event_id, event_type, event_source, event_version, event_timestamp, payload, retry_count
	FROM event_outbox
	WHERE status = ? AND next_retry_at <= NOW()
	ORDER BY id ASC
	LIMIT ?
	`
	rows, err := s.db.Query(query, StatusPending, limit)
	if err != nil {
		return nil, fmt.Errorf("查询待投递 outbox 失败: %w", err)
	}
	defer rows.Close()

	records := make([]Record, 0, limit)
	for rows.Next() {
		var rec Record
		var evt eventbus.Event
		var payload []byte
		if err := rows.Scan(
			&rec.ID,
			&evt.ID,
			&evt.Type,
			&evt.Source,
			&evt.Version,
			&evt.Timestamp,
			&payload,
			&rec.RetryCount,
		); err != nil {
			return nil, fmt.Errorf("扫描 outbox 记录失败: %w", err)
		}

		if !json.Valid(payload) {
			payload = []byte(`{}`)
		}
		evt.Data = payload
		rec.Event = &evt
		records = append(records, rec)
	}
	return records, nil
}

func (s *Store) MarkPublished(id int64) error {
	query := `
	UPDATE event_outbox
	SET status = ?, published_at = NOW(), updated_at = NOW()
	WHERE id = ?
	`
	_, err := s.db.Exec(query, StatusSent, id)
	if err != nil {
		return fmt.Errorf("标记 outbox 已发布失败: %w", err)
	}
	return nil
}

func (s *Store) MarkRetry(id int64, nextRetry time.Time, lastErr string) error {
	query := `
	UPDATE event_outbox
	SET retry_count = retry_count + 1, next_retry_at = ?, last_error = ?, updated_at = NOW()
	WHERE id = ?
	`
	_, err := s.db.Exec(query, nextRetry, truncateError(lastErr), id)
	if err != nil {
		return fmt.Errorf("更新 outbox 重试信息失败: %w", err)
	}
	return nil
}

func (s *Store) MarkDead(id int64, lastErr string) error {
	query := `
	UPDATE event_outbox
	SET status = ?, retry_count = retry_count + 1, last_error = ?, updated_at = NOW()
	WHERE id = ?
	`
	_, err := s.db.Exec(query, StatusDead, truncateError(lastErr), id)
	if err != nil {
		return fmt.Errorf("标记 outbox 死信失败: %w", err)
	}
	return nil
}

func truncateError(errMsg string) string {
	msg := strings.TrimSpace(errMsg)
	if len(msg) <= 512 {
		return msg
	}
	return msg[:512]
}
