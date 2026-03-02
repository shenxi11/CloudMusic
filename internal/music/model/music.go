package model

// MusicFile 音乐文件模型
type MusicFile struct {
	ID           int     `json:"id"`
	Path         string  `json:"path"`
	Title        string  `json:"title"`
	Artist       string  `json:"artist"`
	Album        string  `json:"album"`
	DurationSec  float64 `json:"duration_sec"`
	LrcPath      string  `json:"lrc_path"`
	CoverArtPath string  `json:"cover_art_path"`
	SizeBytes    int64   `json:"size_bytes"`
	FileType     string  `json:"file_type"`
	IsAudio      bool    `json:"is_audio"`
}

// FileListItem 文件列表项
type FileListItem struct {
	Path        string  `json:"path"`
	Duration    string  `json:"duration"`
	Artist      string  `json:"artist,omitempty"`
	CoverArtURL *string `json:"cover_art_url,omitempty"`
}

// MusicResponse 音乐详情响应
type MusicResponse struct {
	StreamURL     string   `json:"stream_url"`
	LrcURL        *string  `json:"lrc_url,omitempty"`
	AlbumCoverURL *string  `json:"album_cover_url,omitempty"`
	Duration      *float64 `json:"duration,omitempty"`
	Title         string   `json:"title,omitempty"`
	Artist        string   `json:"artist,omitempty"`
	Album         string   `json:"album,omitempty"`
}

// StreamRequest 流媒体请求
type StreamRequest struct {
	Filename string `json:"filename"`
}

// ArtistMusicRequest 按歌手查询音乐请求
type ArtistMusicRequest struct {
	Artist string `json:"artist"`
}

// ArtistMusicResponse 按歌手查询音乐响应
type ArtistMusicResponse struct {
	Artist string         `json:"artist"`
	Count  int            `json:"count"`
	Musics []FileListItem `json:"musics"`
}

// SearchRequest 关键词搜索请求
type SearchRequest struct {
	Keyword string `json:"keyword"`
}

// MusicFileWithScore 带相关性评分的音乐文件
type MusicFileWithScore struct {
	MusicFile
	Score int // 相关性评分，越高越相关
}
