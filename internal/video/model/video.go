package model

// VideoFile 视频文件信息
type VideoFile struct {
	Name string `json:"name"` // 文件名（不含扩展名）
	Path string `json:"path"` // 相对路径
	Size int64  `json:"size"` // 文件大小（字节）
}

// VideoStreamRequest 视频流请求
type VideoStreamRequest struct {
	Path string `json:"path"` // 视频文件路径
}

// VideoStreamResponse 视频流响应
type VideoStreamResponse struct {
	URL string `json:"url"` // 流媒体URL
}

// VideoPlaybackInfoRequest 视频播放信息请求。
type VideoPlaybackInfoRequest struct {
	Path string `json:"path"` // 视频文件路径
}

// VideoPlaybackVariant 描述一个可播放的视频清晰度版本。
type VideoPlaybackVariant struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Bandwidth int    `json:"bandwidth"`
	URL       string `json:"url"`
	Ready     bool   `json:"ready"`
}

// VideoPlaybackInfoResponse 返回视频直链、HLS 和多清晰度信息。
type VideoPlaybackInfoResponse struct {
	Path       string                 `json:"path"`
	DirectURL  string                 `json:"direct_url"`
	HLSReady   bool                   `json:"hls_ready"`
	MasterURL  string                 `json:"master_url"`
	Variants   []VideoPlaybackVariant `json:"variants"`
	DurationMs int64                  `json:"duration_ms"`
	Status     string                 `json:"status"`
	Version    string                 `json:"version"`
}
