package model

import "time"

// User 用户模型（与数据库表结构一致）
type User struct {
	Account   string    `json:"account" db:"account"` // 主键
	Password  string    `json:"-" db:"password"`      // 不返回密码
	Username  string    `json:"username" db:"username"`
	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at,omitempty" db:"updated_at"`
}

// UserMusic 用户收藏的音乐（对应 user_path 表）
type UserMusic struct {
	ID        int    `json:"id" db:"id"`
	Username  string `json:"username" db:"username"` // 关联字段改为 username
	MusicPath string `json:"music_path" db:"music_path"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
	Username string `json:"username"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Success      string   `json:"success"`      // 兼容旧客户端
	SuccessBool  bool     `json:"success_bool"` // 新布尔字段
	Username     string   `json:"username"`
	SongPathList []string `json:"song_path_list"`
}

// AddMusicRequest 添加音乐请求
type AddMusicRequest struct {
	Username  string `json:"username"`
	MusicPath string `json:"music_path"`
}
