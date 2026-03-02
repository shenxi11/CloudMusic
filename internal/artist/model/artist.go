package model

import "time"

// Artist 歌手实体
type Artist struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchArtistRequest 搜索歌手请求
type SearchArtistRequest struct {
	Artist string `json:"artist"`
}

// SearchArtistResponse 搜索歌手响应
type SearchArtistResponse struct {
	Exists bool `json:"exists"`
}
