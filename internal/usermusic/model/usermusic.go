package model

import "time"

// FavoriteMusic 用户喜欢的音乐
type FavoriteMusic struct {
	ID           int       `json:"id"`
	UserAccount  string    `json:"user_account"`
	MusicPath    string    `json:"music_path"`
	MusicTitle   string    `json:"music_title,omitempty"`
	Artist       string    `json:"artist,omitempty"`
	DurationSec  float64   `json:"duration_sec,omitempty"`
	IsLocal      bool      `json:"is_local"`
	CoverArtPath string    `json:"cover_art_path,omitempty"` // 封面图片路径
	CreatedAt    time.Time `json:"created_at"`
}

// PlayHistory 播放历史
type PlayHistory struct {
	ID           int       `json:"id"`
	UserAccount  string    `json:"user_account"`
	MusicPath    string    `json:"music_path"`
	MusicTitle   string    `json:"music_title,omitempty"`
	Artist       string    `json:"artist,omitempty"`
	Album        string    `json:"album,omitempty"`
	DurationSec  float64   `json:"duration_sec,omitempty"`
	IsLocal      bool      `json:"is_local"`
	CoverArtPath string    `json:"cover_art_path,omitempty"` // 封面图片路径
	PlayTime     time.Time `json:"play_time"`
}

// AddFavoriteRequest 添加喜欢音乐请求
type AddFavoriteRequest struct {
	MusicPath   string  `json:"music_path"`
	MusicTitle  string  `json:"music_title,omitempty"`
	Artist      string  `json:"artist,omitempty"`
	DurationSec float64 `json:"duration_sec,omitempty"`
	IsLocal     bool    `json:"is_local"`
}

// AddPlayHistoryRequest 添加播放历史请求
type AddPlayHistoryRequest struct {
	MusicPath   string  `json:"music_path"`
	MusicTitle  string  `json:"music_title,omitempty"`
	Artist      string  `json:"artist,omitempty"`
	Album       string  `json:"album,omitempty"`
	DurationSec float64 `json:"duration_sec,omitempty"`
	IsLocal     bool    `json:"is_local"`
}

// MusicItem 音乐项（用于返回列表）
type MusicItem struct {
	Path        string  `json:"path"`
	Title       string  `json:"title,omitempty"`
	Artist      string  `json:"artist,omitempty"`
	Album       string  `json:"album,omitempty"`
	Duration    string  `json:"duration,omitempty"`
	IsLocal     bool    `json:"is_local"`
	CoverArtURL *string `json:"cover_art_url,omitempty"`
	AddedAt     *string `json:"added_at,omitempty"`  // 喜欢列表使用
	PlayTime    *string `json:"play_time,omitempty"` // 播放历史使用
}
