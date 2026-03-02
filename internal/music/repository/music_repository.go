package repository

import (
	"context"
	"database/sql"

	"music-platform/internal/music/model"
)

// MusicRepository 音乐仓储接口
type MusicRepository interface {
	FindAll(ctx context.Context) ([]*model.MusicFile, error)
	FindByPath(ctx context.Context, path string) (*model.MusicFile, error)
	FindByPathLike(ctx context.Context, filename string) (*model.MusicFile, error)
	FindByArtist(ctx context.Context, artist string) ([]*model.MusicFile, error)
	SearchByKeyword(ctx context.Context, keyword string) ([]*model.MusicFile, error)
}

type musicRepository struct {
	db *sql.DB
}

// NewMusicRepository 创建音乐仓储
func NewMusicRepository(db *sql.DB) MusicRepository {
	return &musicRepository{db: db}
}

// FindAll 查询所有音频文件
func (r *musicRepository) FindAll(ctx context.Context) ([]*model.MusicFile, error) {
	query := "SELECT path, duration_sec, artist, cover_art_path FROM music_files WHERE is_audio = 1"
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var musicFiles []*model.MusicFile
	for rows.Next() {
		mf := &model.MusicFile{}
		var coverArtPath sql.NullString

		if err := rows.Scan(&mf.Path, &mf.DurationSec, &mf.Artist, &coverArtPath); err != nil {
			return nil, err
		}

		if coverArtPath.Valid {
			mf.CoverArtPath = coverArtPath.String
		}

		musicFiles = append(musicFiles, mf)
	}

	return musicFiles, rows.Err()
}

// SearchByKeyword 根据关键词搜索音乐（搜索 title, artist, album, path）
func (r *musicRepository) SearchByKeyword(ctx context.Context, keyword string) ([]*model.MusicFile, error) {
	// 使用 LIKE 查询，搜索标题、歌手、专辑和路径
	query := `SELECT path, title, artist, album, duration_sec, cover_art_path 
	          FROM music_files 
	          WHERE is_audio = 1 
	          AND (
	              title LIKE ? OR 
	              artist LIKE ? OR 
	              album LIKE ? OR 
	              path LIKE ?
	          )`

	pattern := "%" + keyword + "%"
	rows, err := r.db.QueryContext(ctx, query, pattern, pattern, pattern, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var musicFiles []*model.MusicFile
	for rows.Next() {
		mf := &model.MusicFile{}
		var title, album, coverArtPath sql.NullString

		if err := rows.Scan(&mf.Path, &title, &mf.Artist, &album, &mf.DurationSec, &coverArtPath); err != nil {
			return nil, err
		}

		if title.Valid {
			mf.Title = title.String
		}
		if album.Valid {
			mf.Album = album.String
		}
		if coverArtPath.Valid {
			mf.CoverArtPath = coverArtPath.String
		}

		musicFiles = append(musicFiles, mf)
	}

	return musicFiles, rows.Err()
}

// FindByPath 根据路径查询音乐
func (r *musicRepository) FindByPath(ctx context.Context, path string) (*model.MusicFile, error) {
	mf := &model.MusicFile{}
	var lrcPath, coverArtPath sql.NullString

	query := `SELECT id, path, title, artist, album, duration_sec, lrc_path, cover_art_path 
	          FROM music_files WHERE path = ?`

	err := r.db.QueryRowContext(ctx, query, path).Scan(
		&mf.ID, &mf.Path, &mf.Title, &mf.Artist, &mf.Album,
		&mf.DurationSec, &lrcPath, &coverArtPath,
	)

	if err != nil {
		return nil, err
	}

	if lrcPath.Valid {
		mf.LrcPath = lrcPath.String
	}
	if coverArtPath.Valid {
		mf.CoverArtPath = coverArtPath.String
	}

	return mf, nil
}

// FindByPathLike 根据文件名模糊查询
func (r *musicRepository) FindByPathLike(ctx context.Context, filename string) (*model.MusicFile, error) {
	mf := &model.MusicFile{}
	var lrcPath, coverArtPath sql.NullString

	query := `SELECT id, path, title, artist, album, duration_sec, lrc_path, cover_art_path 
	          FROM music_files WHERE path LIKE ?`

	err := r.db.QueryRowContext(ctx, query, "%"+filename).Scan(
		&mf.ID, &mf.Path, &mf.Title, &mf.Artist, &mf.Album,
		&mf.DurationSec, &lrcPath, &coverArtPath,
	)

	if err != nil {
		return nil, err
	}

	if lrcPath.Valid {
		mf.LrcPath = lrcPath.String
	}
	if coverArtPath.Valid {
		mf.CoverArtPath = coverArtPath.String
	}

	return mf, nil
}

// FindByArtist 根据歌手查询音乐
func (r *musicRepository) FindByArtist(ctx context.Context, artist string) ([]*model.MusicFile, error) {
	query := `SELECT path, title, artist, album, duration_sec, cover_art_path 
          FROM music_files WHERE is_audio = 1 AND artist LIKE ?`

	rows, err := r.db.QueryContext(ctx, query, "%"+artist+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var musicFiles []*model.MusicFile
	for rows.Next() {
		mf := &model.MusicFile{}
		var album, coverArtPath sql.NullString

		if err := rows.Scan(&mf.Path, &mf.Title, &mf.Artist, &album, &mf.DurationSec, &coverArtPath); err != nil {
			return nil, err
		}

		if album.Valid {
			mf.Album = album.String
		}
		if coverArtPath.Valid {
			mf.CoverArtPath = coverArtPath.String
		}

		musicFiles = append(musicFiles, mf)
	}

	return musicFiles, rows.Err()
}
