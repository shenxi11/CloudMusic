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
