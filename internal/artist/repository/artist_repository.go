package repository

import (
	"context"
	"database/sql"

	"music-platform/internal/artist/model"
)

// ArtistRepository 歌手仓储接口
type ArtistRepository interface {
	ExistsByName(ctx context.Context, name string) (bool, error)
	FindByName(ctx context.Context, name string) (*model.Artist, error)
}

type artistRepository struct {
	db *sql.DB
}

// NewArtistRepository 创建歌手仓储
func NewArtistRepository(db *sql.DB) ArtistRepository {
	return &artistRepository{db: db}
}

// ExistsByName 检查歌手是否存在（精确匹配）
func (r *artistRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	query := "SELECT COUNT(*) FROM artists WHERE name = ?"
	var count int
	err := r.db.QueryRowContext(ctx, query, name).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindByName 根据名称查找歌手
func (r *artistRepository) FindByName(ctx context.Context, name string) (*model.Artist, error) {
	query := "SELECT id, name, created_at, updated_at FROM artists WHERE name = ?"
	artist := &model.Artist{}
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&artist.ID,
		&artist.Name,
		&artist.CreatedAt,
		&artist.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return artist, nil
}
