package model

import "time"

const (
	HotChartID    = "hot_online"
	HotChartTitle = "在线热歌榜"

	Window7d  = "7d"
	Window30d = "30d"
	WindowAll = "all"

	SourceCatalog = "catalog"
	SourceJamendo = "jamendo"
)

type HotChartQuery struct {
	Window string
	Limit  int
}

type HotChartRebuildQuery struct {
	Window string
}

type HotChartResponse struct {
	ChartID     string         `json:"chart_id"`
	Title       string         `json:"title"`
	Window      string         `json:"window"`
	GeneratedAt string         `json:"generated_at"`
	Items       []HotChartItem `json:"items"`
}

type HotChartItem struct {
	Rank        int     `json:"rank"`
	MusicPath   string  `json:"music_path"`
	Path        string  `json:"path"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist,omitempty"`
	Album       string  `json:"album,omitempty"`
	DurationSec float64 `json:"duration_sec,omitempty"`
	CoverArtURL *string `json:"cover_art_url,omitempty"`
	Source      string  `json:"source"`
	SourceID    string  `json:"source_id"`
	PlayCount   int64   `json:"play_count"`
}

type HotChartRebuildResponse struct {
	Window         string `json:"window"`
	RebuiltBuckets int    `json:"rebuilt_buckets"`
	RebuiltItems   int64  `json:"rebuilt_items"`
	GeneratedAt    string `json:"generated_at"`
}

type HotTrackMeta struct {
	MusicPath   string  `json:"music_path"`
	Title       string  `json:"title,omitempty"`
	Artist      string  `json:"artist,omitempty"`
	Album       string  `json:"album,omitempty"`
	DurationSec float64 `json:"duration_sec,omitempty"`
	CoverArtURL string  `json:"cover_art_url,omitempty"`
	Source      string  `json:"source"`
	SourceID    string  `json:"source_id,omitempty"`
}

type HotTrackPlay struct {
	MusicPath   string
	Title       string
	Artist      string
	Album       string
	DurationSec float64
	IsLocal     bool
}

type HotTrackStat struct {
	MusicPath    string
	MusicTitle   string
	Artist       string
	Album        string
	DurationSec  float64
	CoverArtPath string
	PlayCount    int64
	LastPlayTime time.Time
}

type DailyHotTrackStat struct {
	Day string
	HotTrackStat
}

type ScoredMusicPath struct {
	MusicPath string
	Score     float64
}
