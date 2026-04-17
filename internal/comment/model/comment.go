package model

import (
	"errors"
	"time"
)

var (
	ErrCommentNotFound     = errors.New("评论不存在")
	ErrRootCommentRequired = errors.New("仅支持对主评论进行回复或查看楼层")
	ErrReplyTargetInvalid  = errors.New("回复目标不存在或不属于当前楼层")
	ErrDeleteForbidden     = errors.New("只能删除自己的评论")
	ErrOnlineMusicOnly     = errors.New("仅支持在线音乐评论")
	ErrInvalidSession      = errors.New("在线会话无效，请重新登录")
	ErrAuthUnavailable     = errors.New("评论服务暂时无法校验登录态")
)

type TrackMeta struct {
	MusicPath    string
	Source       string
	SourceID     string
	MusicTitle   string
	Artist       string
	CoverArtPath string
}

type CommentThread struct {
	ID                int64
	MusicPath         string
	Source            string
	SourceID          string
	MusicTitle        string
	Artist            string
	CoverArtPath      string
	RootCommentCount  int
	TotalCommentCount int
	LastCommentedAt   *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CommentRecord struct {
	ID                 int64
	ThreadID           int64
	RootCommentID      int64
	ReplyToCommentID   *int64
	UserAccount        string
	UsernameSnapshot   string
	AvatarPathSnapshot string
	ReplyToUserAccount string
	ReplyToUsername    string
	Content            string
	IsDeleted          bool
	ReplyCount         int
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          *time.Time
}

type CreateCommentRequest struct {
	UserAccount  string `json:"user_account"`
	SessionToken string `json:"session_token"`
	MusicPath    string `json:"music_path"`
	MusicTitle   string `json:"music_title,omitempty"`
	Artist       string `json:"artist,omitempty"`
	Content      string `json:"content"`
}

type CreateReplyRequest struct {
	UserAccount     string `json:"user_account"`
	SessionToken    string `json:"session_token"`
	Content         string `json:"content"`
	TargetCommentID int64  `json:"target_comment_id,omitempty"`
}

type DeleteCommentRequest struct {
	UserAccount  string `json:"user_account"`
	SessionToken string `json:"session_token"`
}

type CommentReplyTo struct {
	CommentID   int64  `json:"comment_id"`
	UserAccount string `json:"user_account"`
	Username    string `json:"username"`
}

type CommentItem struct {
	CommentID     int64           `json:"comment_id"`
	RootCommentID int64           `json:"root_comment_id,omitempty"`
	UserAccount   string          `json:"user_account"`
	Username      string          `json:"username"`
	AvatarURL     string          `json:"avatar_url,omitempty"`
	Content       string          `json:"content"`
	IsDeleted     bool            `json:"is_deleted"`
	CreatedAt     string          `json:"created_at"`
	ReplyCount    int             `json:"reply_count,omitempty"`
	ReplyTo       *CommentReplyTo `json:"reply_to,omitempty"`
}

type ThreadCommentsResponse struct {
	MusicPath         string        `json:"music_path"`
	Source            string        `json:"source"`
	SourceID          string        `json:"source_id,omitempty"`
	MusicTitle        string        `json:"music_title,omitempty"`
	Artist            string        `json:"artist,omitempty"`
	CoverArtURL       string        `json:"cover_art_url,omitempty"`
	RootCommentCount  int           `json:"root_comment_count"`
	TotalCommentCount int           `json:"total_comment_count"`
	Page              int           `json:"page"`
	PageSize          int           `json:"page_size"`
	Items             []CommentItem `json:"items"`
}

type CommentRepliesResponse struct {
	RootCommentID int64         `json:"root_comment_id"`
	Page          int           `json:"page"`
	PageSize      int           `json:"page_size"`
	Total         int           `json:"total"`
	Items         []CommentItem `json:"items"`
}

type CommentActionResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	CommentID int64       `json:"comment_id,omitempty"`
	Comment   CommentItem `json:"comment,omitempty"`
}
