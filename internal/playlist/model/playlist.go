package model

import (
	"errors"
	"time"
)

var (
	ErrPlaylistNotFound    = errors.New("歌单不存在或无权限访问")
	ErrInvalidReorderInput = errors.New("排序项不完整或不合法")
)

type Playlist struct {
	ID               int64
	UserAccount      string
	Name             string
	Description      string
	CoverPath        string
	TrackCount       int
	TotalDurationSec float64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type PlaylistItemRecord struct {
	ID           int64
	PlaylistID   int64
	UserAccount  string
	Position     int
	MusicPath    string
	MusicTitle   string
	Artist       string
	Album        string
	DurationSec  float64
	IsLocal      bool
	CoverArtPath string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreatePlaylistRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	CoverPath   string `json:"cover_path,omitempty"`
}

type UpdatePlaylistRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	CoverPath   string `json:"cover_path,omitempty"`
}

type PlaylistTrackInput struct {
	MusicPath    string  `json:"music_path"`
	MusicTitle   string  `json:"music_title,omitempty"`
	Artist       string  `json:"artist,omitempty"`
	Album        string  `json:"album,omitempty"`
	DurationSec  float64 `json:"duration_sec,omitempty"`
	IsLocal      bool    `json:"is_local"`
	CoverArtPath string  `json:"cover_art_path,omitempty"`
}

type AddPlaylistItemsRequest struct {
	Items []PlaylistTrackInput `json:"items"`
}

type RemovePlaylistItemsRequest struct {
	MusicPaths []string `json:"music_paths"`
}

type PlaylistReorderItem struct {
	MusicPath string `json:"music_path"`
	Position  int    `json:"position"`
}

type ReorderPlaylistItemsRequest struct {
	Items []PlaylistReorderItem `json:"items"`
}

type PlaylistSummary struct {
	ID               int64   `json:"id"`
	Name             string  `json:"name"`
	Description      string  `json:"description,omitempty"`
	CoverURL         string  `json:"cover_url,omitempty"`
	TrackCount       int     `json:"track_count"`
	TotalDurationSec float64 `json:"total_duration_sec"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

type PlaylistItem struct {
	ID          int64   `json:"id"`
	Position    int     `json:"position"`
	MusicPath   string  `json:"music_path"`
	MusicTitle  string  `json:"music_title,omitempty"`
	Artist      string  `json:"artist,omitempty"`
	Album       string  `json:"album,omitempty"`
	DurationSec float64 `json:"duration_sec,omitempty"`
	IsLocal     bool    `json:"is_local"`
	CoverArtURL string  `json:"cover_art_url,omitempty"`
	AddedAt     string  `json:"added_at"`
}

type PlaylistDetail struct {
	ID               int64          `json:"id"`
	Name             string         `json:"name"`
	Description      string         `json:"description,omitempty"`
	CoverURL         string         `json:"cover_url,omitempty"`
	TrackCount       int            `json:"track_count"`
	TotalDurationSec float64        `json:"total_duration_sec"`
	CreatedAt        string         `json:"created_at"`
	UpdatedAt        string         `json:"updated_at"`
	Items            []PlaylistItem `json:"items"`
}

type PlaylistListResponse struct {
	Items    []PlaylistSummary `json:"items"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Total    int               `json:"total"`
}
