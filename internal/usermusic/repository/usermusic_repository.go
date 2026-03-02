package repository

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"music-platform/internal/usermusic/model"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

type UserMusicRepository struct {
	db *sql.DB

	profileSchema string
	catalogSchema string

	favoriteTable   string
	historyTable    string
	musicFilesTable string
}

func NewUserMusicRepository(db *sql.DB, profileSchema, catalogSchema string) *UserMusicRepository {
	pSchema := normalizeSchema(profileSchema, "music_users")
	cSchema := normalizeSchema(catalogSchema, pSchema)

	return &UserMusicRepository{
		db:              db,
		profileSchema:   pSchema,
		catalogSchema:   cSchema,
		favoriteTable:   qualifiedTable(pSchema, "user_favorite_music"),
		historyTable:    qualifiedTable(pSchema, "user_play_history"),
		musicFilesTable: qualifiedTable(cSchema, "music_files"),
	}
}

// EnsureTables 初始化 profile schema 的行为表（不依赖跨服务外键）
func (r *UserMusicRepository) EnsureTables() error {
	if _, err := r.db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s DEFAULT CHARSET=utf8mb4", quoteIdent(r.profileSchema))); err != nil {
		return fmt.Errorf("创建 profile schema 失败: %w", err)
	}

	favoriteDDL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id INT NOT NULL AUTO_INCREMENT,
			user_account VARCHAR(50) NOT NULL COMMENT '用户账号',
			music_path VARCHAR(500) NOT NULL COMMENT '音乐文件路径',
			music_title VARCHAR(255) DEFAULT NULL COMMENT '音乐标题',
			artist VARCHAR(255) DEFAULT NULL COMMENT '歌手',
			duration_sec FLOAT DEFAULT NULL COMMENT '时长（秒）',
			is_local TINYINT(1) DEFAULT 0 COMMENT '是否本地音乐',
			created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP COMMENT '添加时间',
			PRIMARY KEY (id),
			UNIQUE KEY uk_user_music (user_account, music_path),
			KEY idx_user_account (user_account),
			KEY idx_created_at (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户喜欢的音乐'
	`, r.favoriteTable)
	if _, err := r.db.Exec(favoriteDDL); err != nil {
		return fmt.Errorf("创建收藏表失败: %w", err)
	}

	historyDDL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id INT NOT NULL AUTO_INCREMENT,
			user_account VARCHAR(50) NOT NULL COMMENT '用户账号',
			music_path VARCHAR(500) NOT NULL COMMENT '音乐文件路径',
			music_title VARCHAR(255) DEFAULT NULL COMMENT '音乐标题',
			artist VARCHAR(255) DEFAULT NULL COMMENT '歌手',
			album VARCHAR(255) DEFAULT NULL COMMENT '专辑',
			duration_sec FLOAT DEFAULT NULL COMMENT '时长（秒）',
			is_local TINYINT(1) DEFAULT 0 COMMENT '是否本地音乐',
			play_time TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP COMMENT '播放时间',
			PRIMARY KEY (id),
			KEY idx_user_account (user_account),
			KEY idx_play_time (play_time),
			KEY idx_user_play_time (user_account, play_time)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户播放历史'
	`, r.historyTable)
	if _, err := r.db.Exec(historyDDL); err != nil {
		return fmt.Errorf("创建历史表失败: %w", err)
	}
	return nil
}

// AddFavorite 添加喜欢的音乐
func (r *UserMusicRepository) AddFavorite(userAccount string, req model.AddFavoriteRequest) error {
	query := fmt.Sprintf(`
		INSERT INTO %s
		(user_account, music_path, music_title, artist, duration_sec, is_local, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, r.favoriteTable)
	_, err := r.db.Exec(query, userAccount, req.MusicPath, req.MusicTitle,
		req.Artist, req.DurationSec, req.IsLocal, time.Now())
	if err != nil {
		if isDuplicateEntryError(err) {
			return fmt.Errorf("该音乐已经在喜欢列表中")
		}
		return fmt.Errorf("添加喜欢音乐失败: %w", err)
	}
	return nil
}

// RemoveFavorite 移除喜欢的音乐
func (r *UserMusicRepository) RemoveFavorite(userAccount, musicPath string) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE user_account = ? AND music_path = ?`, r.favoriteTable)
	result, err := r.db.Exec(query, userAccount, musicPath)
	if err != nil {
		return fmt.Errorf("移除喜欢音乐失败: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("该音乐不在喜欢列表中")
	}
	return nil
}

// ListFavorites 获取用户喜欢的音乐列表（包含封面信息）
func (r *UserMusicRepository) ListFavorites(userAccount string) ([]model.FavoriteMusic, error) {
	query := fmt.Sprintf(`
		SELECT 
			f.id, f.user_account, f.music_path, f.music_title, f.artist, 
			f.duration_sec, f.is_local, f.created_at,
			COALESCE(m.cover_art_path, '') as cover_art_path
		FROM %s f
		LEFT JOIN %s m ON (
			CASE 
				WHEN f.is_local = 0 THEN 
					CASE 
						WHEN f.music_path LIKE 'http%%' THEN SUBSTRING_INDEX(f.music_path, '/uploads/', -1)
						ELSE f.music_path
					END
				ELSE f.music_path
			END
		) = m.path
		WHERE f.user_account = ?
		ORDER BY f.created_at DESC
	`, r.favoriteTable, r.musicFilesTable)
	rows, err := r.db.Query(query, userAccount)
	if err != nil {
		return nil, fmt.Errorf("查询喜欢音乐失败: %w", err)
	}
	defer rows.Close()

	var favorites []model.FavoriteMusic
	for rows.Next() {
		var fav model.FavoriteMusic
		err := rows.Scan(&fav.ID, &fav.UserAccount, &fav.MusicPath,
			&fav.MusicTitle, &fav.Artist, &fav.DurationSec, &fav.IsLocal, &fav.CreatedAt, &fav.CoverArtPath)
		if err != nil {
			return nil, fmt.Errorf("扫描喜欢音乐失败: %w", err)
		}
		favorites = append(favorites, fav)
	}
	return favorites, nil
}

// AddPlayHistory 添加播放历史
func (r *UserMusicRepository) AddPlayHistory(userAccount string, req model.AddPlayHistoryRequest) error {
	query := fmt.Sprintf(`
		INSERT INTO %s
		(user_account, music_path, music_title, artist, album, duration_sec, is_local, play_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, r.historyTable)
	_, err := r.db.Exec(query, userAccount, req.MusicPath, req.MusicTitle,
		req.Artist, req.Album, req.DurationSec, req.IsLocal, time.Now())
	if err != nil {
		return fmt.Errorf("添加播放历史失败: %w", err)
	}
	return nil
}

// ListPlayHistory 获取播放历史（按时间倒序，包含封面信息）
func (r *UserMusicRepository) ListPlayHistory(userAccount string, limit int) ([]model.PlayHistory, error) {
	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf(`
		SELECT 
			h.id, h.user_account, h.music_path, h.music_title, h.artist, h.album, 
			h.duration_sec, h.is_local, h.play_time,
			COALESCE(m.cover_art_path, '') as cover_art_path
		FROM %s h
		LEFT JOIN %s m ON (
			CASE 
				WHEN h.is_local = 0 THEN SUBSTRING_INDEX(h.music_path, '/uploads/', -1)
				ELSE h.music_path
			END
		) = m.path
		WHERE h.user_account = ?
		ORDER BY h.play_time DESC
		LIMIT ?
	`, r.historyTable, r.musicFilesTable)
	rows, err := r.db.Query(query, userAccount, limit)
	if err != nil {
		return nil, fmt.Errorf("查询播放历史失败: %w", err)
	}
	defer rows.Close()

	var history []model.PlayHistory
	for rows.Next() {
		var h model.PlayHistory
		err := rows.Scan(&h.ID, &h.UserAccount, &h.MusicPath, &h.MusicTitle,
			&h.Artist, &h.Album, &h.DurationSec, &h.IsLocal, &h.PlayTime, &h.CoverArtPath)
		if err != nil {
			return nil, fmt.Errorf("扫描播放历史失败: %w", err)
		}
		history = append(history, h)
	}
	return history, nil
}

// ListPlayHistoryDistinct 获取去重的播放历史（每首歌只显示最近一次播放，包含封面信息）
func (r *UserMusicRepository) ListPlayHistoryDistinct(userAccount string, limit int) ([]model.PlayHistory, error) {
	if limit <= 0 {
		limit = 50
	}

	query := fmt.Sprintf(`
		SELECT 
			h1.id, h1.user_account, h1.music_path, h1.music_title, h1.artist, h1.album,
			h1.duration_sec, h1.is_local, h1.play_time,
			COALESCE(m.cover_art_path, '') as cover_art_path
		FROM %s h1
		INNER JOIN (
			SELECT music_path, MAX(play_time) as max_play_time
			FROM %s
			WHERE user_account = ?
			GROUP BY music_path
		) h2 ON h1.music_path = h2.music_path AND h1.play_time = h2.max_play_time
		LEFT JOIN %s m ON (
			CASE 
				WHEN h1.is_local = 0 THEN SUBSTRING_INDEX(h1.music_path, '/uploads/', -1)
				ELSE h1.music_path
			END
		) = m.path
		WHERE h1.user_account = ?
		ORDER BY h1.play_time DESC
		LIMIT ?
	`, r.historyTable, r.historyTable, r.musicFilesTable)
	rows, err := r.db.Query(query, userAccount, userAccount, limit)
	if err != nil {
		return nil, fmt.Errorf("查询去重播放历史失败: %w", err)
	}
	defer rows.Close()

	var history []model.PlayHistory
	for rows.Next() {
		var h model.PlayHistory
		err := rows.Scan(&h.ID, &h.UserAccount, &h.MusicPath, &h.MusicTitle,
			&h.Artist, &h.Album, &h.DurationSec, &h.IsLocal, &h.PlayTime, &h.CoverArtPath)
		if err != nil {
			return nil, fmt.Errorf("扫描去重播放历史失败: %w", err)
		}
		history = append(history, h)
	}
	return history, nil
}

// DeletePlayHistory 删除指定的播放历史记录（支持批量删除）
func (r *UserMusicRepository) DeletePlayHistory(userAccount string, musicPaths []string) (int64, error) {
	if len(musicPaths) == 0 {
		return 0, fmt.Errorf("音乐路径列表不能为空")
	}

	placeholders := make([]string, len(musicPaths))
	args := make([]interface{}, 0, len(musicPaths)+1)
	args = append(args, userAccount)

	for i, path := range musicPaths {
		placeholders[i] = "?"
		args = append(args, path)
	}

	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE user_account = ? AND music_path IN (%s)
	`, r.historyTable, joinStrings(placeholders, ","))

	result, err := r.db.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("删除播放历史失败: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	return rowsAffected, nil
}

// ClearPlayHistory 清空用户的全部播放历史
func (r *UserMusicRepository) ClearPlayHistory(userAccount string) (int64, error) {
	query := fmt.Sprintf(`DELETE FROM %s WHERE user_account = ?`, r.historyTable)
	result, err := r.db.Exec(query, userAccount)
	if err != nil {
		return 0, fmt.Errorf("清空播放历史失败: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	return rowsAffected, nil
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func isDuplicateEntryError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Error 1062") || strings.Contains(msg, "Duplicate entry")
}

func normalizeSchema(schema, fallback string) string {
	s := strings.TrimSpace(schema)
	if s == "" {
		s = strings.TrimSpace(fallback)
	}
	if s == "" {
		s = "music_users"
	}
	if !identifierPattern.MatchString(s) {
		return "music_users"
	}
	return s
}

func quoteIdent(ident string) string {
	if !identifierPattern.MatchString(ident) {
		return "`music_users`"
	}
	return "`" + ident + "`"
}

func qualifiedTable(schema, table string) string {
	t := strings.TrimSpace(table)
	if !identifierPattern.MatchString(t) {
		t = "unknown_table"
	}
	return quoteIdent(schema) + "." + "`" + t + "`"
}
