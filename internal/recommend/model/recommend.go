package model

import "time"

// RecommendQuery 推荐请求参数
type RecommendQuery struct {
	UserID        string
	Scene         string
	Limit         int
	ExcludePlayed bool
	Cursor        string
}

// RecommendationItem 推荐项
type RecommendationItem struct {
	SongID      string  `json:"song_id"`
	Path        string  `json:"path"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Album       string  `json:"album,omitempty"`
	DurationSec float64 `json:"duration_sec,omitempty"`
	CoverArtURL *string `json:"cover_art_url,omitempty"`
	StreamURL   string  `json:"stream_url"`
	LrcURL      *string `json:"lrc_url,omitempty"`
	Score       float64 `json:"score"`
	Reason      string  `json:"reason"`
	Source      string  `json:"source"`
}

// RecommendationListData 推荐响应 data
type RecommendationListData struct {
	RequestID    string               `json:"request_id"`
	UserID       string               `json:"user_id"`
	Scene        string               `json:"scene"`
	ModelVersion string               `json:"model_version"`
	NextCursor   string               `json:"next_cursor,omitempty"`
	Items        []RecommendationItem `json:"items"`
}

// FeedbackRequest 推荐反馈请求
type FeedbackRequest struct {
	UserID       string `json:"user_id"`
	SongID       string `json:"song_id"`
	EventType    string `json:"event_type"`
	PlayMS       int64  `json:"play_ms"`
	DurationMS   int64  `json:"duration_ms"`
	Scene        string `json:"scene"`
	RequestID    string `json:"request_id"`
	ModelVersion string `json:"model_version"`
	EventAt      string `json:"event_at"`
}

// FeedbackRecord 推荐反馈入库模型
type FeedbackRecord struct {
	UserID       string
	SongID       string
	EventType    string
	PlayMS       int64
	DurationMS   int64
	Scene        string
	RequestID    string
	ModelVersion string
	EventAt      time.Time
}

// TrainRequest 触发重训请求
type TrainRequest struct {
	ModelName string `json:"model_name"`
	ForceFull bool   `json:"force_full"`
}

// TrainAccepted 重训任务受理响应
type TrainAccepted struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

// ModelStatus 当前模型状态
type ModelStatus struct {
	ModelName    string         `json:"model_name"`
	ModelVersion string         `json:"model_version"`
	Status       string         `json:"status"`
	TrainedAt    *string        `json:"trained_at,omitempty"`
	Metrics      map[string]any `json:"metrics,omitempty"`
}

// SongCandidate 候选歌曲（仓储层）
type SongCandidate struct {
	Path         string
	Title        string
	Artist       string
	Album        string
	DurationSec  float64
	CoverArtPath string
	LrcPath      string
}
