package repository

import (
	"context"
	"database/sql"

	"music-platform/internal/user/model"
)

// UserMusicRepository 用户音乐收藏仓储接口
type UserMusicRepository interface {
	Create(ctx context.Context, userMusic *model.UserMusic) error
	FindByUsername(ctx context.Context, username string) ([]*model.UserMusic, error)
	Delete(ctx context.Context, username string, musicPath string) error
}

type userMusicRepository struct {
	db *sql.DB
}

// NewUserMusicRepository 创建用户音乐仓储
func NewUserMusicRepository(db *sql.DB) UserMusicRepository {
	return &userMusicRepository{db: db}
}

// Create 创建用户音乐收藏
func (r *userMusicRepository) Create(ctx context.Context, userMusic *model.UserMusic) error {
	query := "INSERT INTO user_path (username, music_path) VALUES (?, ?)"
	result, err := r.db.ExecContext(ctx, query, userMusic.Username, userMusic.MusicPath)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	userMusic.ID = int(id)
	return nil
}

// FindByUsername 根据用户名查找收藏
func (r *userMusicRepository) FindByUsername(ctx context.Context, username string) ([]*model.UserMusic, error) {
	query := "SELECT id, username, music_path FROM user_path WHERE username = ?"
	rows, err := r.db.QueryContext(ctx, query, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var musicList []*model.UserMusic
	for rows.Next() {
		um := &model.UserMusic{}
		if err := rows.Scan(&um.ID, &um.Username, &um.MusicPath); err != nil {
			return nil, err
		}
		musicList = append(musicList, um)
	}

	return musicList, rows.Err()
}

// Delete 删除用户音乐收藏
func (r *userMusicRepository) Delete(ctx context.Context, username string, musicPath string) error {
	query := "DELETE FROM user_path WHERE username = ? AND music_path = ?"
	_, err := r.db.ExecContext(ctx, query, username, musicPath)
	return err
}
