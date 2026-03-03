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
	Success                  string   `json:"success"`      // 兼容旧客户端
	SuccessBool              bool     `json:"success_bool"` // 新布尔字段
	Username                 string   `json:"username"`
	SongPathList             []string `json:"song_path_list"`
	OnlineSessionToken       string   `json:"online_session_token,omitempty"`
	OnlineHeartbeatIntervalS int      `json:"online_heartbeat_interval_sec,omitempty"`
	OnlineTTLSec             int      `json:"online_ttl_sec,omitempty"`
}

// AddMusicRequest 添加音乐请求
type AddMusicRequest struct {
	Username  string `json:"username"`
	MusicPath string `json:"music_path"`
}

// UserPingRequest 用户在线心跳请求
type UserPingRequest struct {
	Account  string `json:"account"`
	Username string `json:"username"`
}

// OnlineSessionStartRequest 在线会话创建请求
type OnlineSessionStartRequest struct {
	Account  string `json:"account"`
	Username string `json:"username"`
	DeviceID string `json:"device_id"`
}

// OnlineHeartbeatRequest 在线会话心跳请求
type OnlineHeartbeatRequest struct {
	Account      string `json:"account"`
	Username     string `json:"username"`
	SessionToken string `json:"session_token"`
	DeviceID     string `json:"device_id"`
}

// OnlineStatusRequest 在线状态查询请求
type OnlineStatusRequest struct {
	Account      string `json:"account"`
	Username     string `json:"username"`
	SessionToken string `json:"session_token"`
}

// OnlineLogoutRequest 在线会话下线请求
type OnlineLogoutRequest struct {
	Account      string `json:"account"`
	Username     string `json:"username"`
	SessionToken string `json:"session_token"`
}

// OnlineSessionResponse 在线会话响应
type OnlineSessionResponse struct {
	Account              string `json:"account"`
	SessionToken         string `json:"session_token"`
	HeartbeatIntervalSec int    `json:"heartbeat_interval_sec"`
	OnlineTTLSec         int    `json:"online_ttl_sec"`
	LastSeenAt           int64  `json:"last_seen_at"`
	ExpireAt             int64  `json:"expire_at"`
}

// OnlineStatusResponse 在线状态响应
type OnlineStatusResponse struct {
	Account              string `json:"account"`
	Online               bool   `json:"online"`
	LastSeenAt           int64  `json:"last_seen_at"`
	TTLRemainingSec      int64  `json:"ttl_remaining_sec"`
	HeartbeatIntervalSec int    `json:"heartbeat_interval_sec"`
	OnlineTTLSec         int    `json:"online_ttl_sec"`
}
