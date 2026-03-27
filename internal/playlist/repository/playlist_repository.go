package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"music-platform/internal/playlist/model"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

type PlaylistRepository struct {
	db *sql.DB

	profileSchema string
	catalogSchema string

	playlistTable      string
	playlistItemsTable string
	musicFilesTable    string
}

func NewPlaylistRepository(db *sql.DB, profileSchema, catalogSchema string) *PlaylistRepository {
	pSchema := normalizeSchema(profileSchema, "music_users")
	cSchema := normalizeSchema(catalogSchema, pSchema)

	return &PlaylistRepository{
		db:                 db,
		profileSchema:      pSchema,
		catalogSchema:      cSchema,
		playlistTable:      qualifiedTable(pSchema, "user_playlists"),
		playlistItemsTable: qualifiedTable(pSchema, "user_playlist_items"),
		musicFilesTable:    qualifiedTable(cSchema, "music_files"),
	}
}

func (r *PlaylistRepository) EnsureTables() error {
	if _, err := r.db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s DEFAULT CHARSET=utf8mb4", quoteIdent(r.profileSchema))); err != nil {
		return fmt.Errorf("创建 profile schema 失败: %w", err)
	}

	playlistDDL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGINT NOT NULL AUTO_INCREMENT,
			user_account VARCHAR(50) NOT NULL COMMENT '用户账号',
			name VARCHAR(128) NOT NULL COMMENT '歌单名称',
			description VARCHAR(1000) DEFAULT NULL COMMENT '歌单简介',
			cover_path VARCHAR(500) DEFAULT NULL COMMENT '歌单封面路径',
			track_count INT NOT NULL DEFAULT 0 COMMENT '歌曲数量',
			total_duration_sec FLOAT NOT NULL DEFAULT 0 COMMENT '总时长（秒）',
			created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
			updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
			PRIMARY KEY (id),
			KEY idx_user_account_updated_at (user_account, updated_at),
			KEY idx_user_account_created_at (user_account, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户私有歌单'
	`, r.playlistTable)
	if _, err := r.db.Exec(playlistDDL); err != nil {
		return fmt.Errorf("创建歌单表失败: %w", err)
	}

	itemDDL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGINT NOT NULL AUTO_INCREMENT,
			playlist_id BIGINT NOT NULL COMMENT '歌单ID',
			user_account VARCHAR(50) NOT NULL COMMENT '用户账号',
			position INT NOT NULL COMMENT '歌单排序',
			music_path VARCHAR(500) NOT NULL COMMENT '音乐路径',
			music_title VARCHAR(255) DEFAULT NULL COMMENT '歌曲名称',
			artist VARCHAR(255) DEFAULT NULL COMMENT '歌手',
			album VARCHAR(255) DEFAULT NULL COMMENT '专辑',
			duration_sec FLOAT DEFAULT NULL COMMENT '时长（秒）',
			is_local TINYINT(1) DEFAULT 0 COMMENT '是否本地歌曲',
			cover_art_path VARCHAR(500) DEFAULT NULL COMMENT '封面路径',
			created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP COMMENT '加入时间',
			updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
			PRIMARY KEY (id),
			UNIQUE KEY uk_playlist_music (playlist_id, music_path),
			UNIQUE KEY uk_playlist_position (playlist_id, position),
			KEY idx_user_playlist (user_account, playlist_id),
			KEY idx_playlist_created_at (playlist_id, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户歌单歌曲'
	`, r.playlistItemsTable)
	if _, err := r.db.Exec(itemDDL); err != nil {
		return fmt.Errorf("创建歌单歌曲表失败: %w", err)
	}

	return nil
}

func (r *PlaylistRepository) CreatePlaylist(userAccount string, req model.CreatePlaylistRequest) (int64, error) {
	query := fmt.Sprintf(`
		INSERT INTO %s
		(user_account, name, description, cover_path, track_count, total_duration_sec, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, 0, ?, ?)
	`, r.playlistTable)
	now := time.Now()
	result, err := r.db.Exec(query, userAccount, req.Name, nullableString(req.Description), nullableString(req.CoverPath), now, now)
	if err != nil {
		return 0, fmt.Errorf("创建歌单失败: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("获取歌单ID失败: %w", err)
	}
	return id, nil
}

func (r *PlaylistRepository) UpdatePlaylist(userAccount string, playlistID int64, req model.UpdatePlaylistRequest) error {
	query := fmt.Sprintf(`
		UPDATE %s
		SET name = ?, description = ?, cover_path = ?, updated_at = ?
		WHERE id = ? AND user_account = ?
	`, r.playlistTable)
	result, err := r.db.Exec(query, req.Name, nullableString(req.Description), nullableString(req.CoverPath), time.Now(), playlistID, userAccount)
	if err != nil {
		return fmt.Errorf("更新歌单失败: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return model.ErrPlaylistNotFound
	}
	return nil
}

func (r *PlaylistRepository) DeletePlaylist(userAccount string, playlistID int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer rollbackQuietly(tx)

	if _, err := r.mustGetPlaylistTx(tx, userAccount, playlistID); err != nil {
		return err
	}

	deleteItemsQuery := fmt.Sprintf(`DELETE FROM %s WHERE playlist_id = ? AND user_account = ?`, r.playlistItemsTable)
	if _, err := tx.Exec(deleteItemsQuery, playlistID, userAccount); err != nil {
		return fmt.Errorf("删除歌单歌曲失败: %w", err)
	}

	deletePlaylistQuery := fmt.Sprintf(`DELETE FROM %s WHERE id = ? AND user_account = ?`, r.playlistTable)
	if _, err := tx.Exec(deletePlaylistQuery, playlistID, userAccount); err != nil {
		return fmt.Errorf("删除歌单失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}

func (r *PlaylistRepository) ListPlaylists(userAccount string, page, pageSize int) ([]model.Playlist, int, error) {
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE user_account = ?`, r.playlistTable)
	var total int
	if err := r.db.QueryRow(countQuery, userAccount).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询歌单总数失败: %w", err)
	}

	offset := (page - 1) * pageSize
	query := fmt.Sprintf(`
		SELECT id, user_account, name, COALESCE(description, ''), COALESCE(cover_path, ''), track_count, total_duration_sec, created_at, updated_at
		FROM %s
		WHERE user_account = ?
		ORDER BY updated_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, r.playlistTable)
	rows, err := r.db.Query(query, userAccount, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询歌单列表失败: %w", err)
	}
	defer rows.Close()

	var playlists []model.Playlist
	for rows.Next() {
		var item model.Playlist
		if err := rows.Scan(&item.ID, &item.UserAccount, &item.Name, &item.Description, &item.CoverPath, &item.TrackCount, &item.TotalDurationSec, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("扫描歌单列表失败: %w", err)
		}
		playlists = append(playlists, item)
	}
	return playlists, total, nil
}

func (r *PlaylistRepository) GetPlaylistDetail(userAccount string, playlistID int64) (*model.Playlist, []model.PlaylistItemRecord, error) {
	playlistQuery := fmt.Sprintf(`
		SELECT id, user_account, name, COALESCE(description, ''), COALESCE(cover_path, ''), track_count, total_duration_sec, created_at, updated_at
		FROM %s
		WHERE id = ? AND user_account = ?
	`, r.playlistTable)

	var playlist model.Playlist
	if err := r.db.QueryRow(playlistQuery, playlistID, userAccount).Scan(
		&playlist.ID,
		&playlist.UserAccount,
		&playlist.Name,
		&playlist.Description,
		&playlist.CoverPath,
		&playlist.TrackCount,
		&playlist.TotalDurationSec,
		&playlist.CreatedAt,
		&playlist.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, model.ErrPlaylistNotFound
		}
		return nil, nil, fmt.Errorf("查询歌单详情失败: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT
			i.id, i.playlist_id, i.user_account, i.position, i.music_path,
			COALESCE(i.music_title, ''), COALESCE(i.artist, ''), COALESCE(i.album, ''),
			COALESCE(i.duration_sec, 0), i.is_local,
			COALESCE(NULLIF(i.cover_art_path, ''), COALESCE(m.cover_art_path, '')) AS cover_art_path,
			i.created_at, i.updated_at
		FROM %s i
		LEFT JOIN %s m ON (
			CASE
				WHEN i.is_local = 0 THEN
					CASE
						WHEN i.music_path LIKE 'http%%' THEN SUBSTRING_INDEX(i.music_path, '/uploads/', -1)
						ELSE i.music_path
					END
				ELSE i.music_path
			END
		) = m.path
		WHERE i.playlist_id = ? AND i.user_account = ?
		ORDER BY i.position ASC, i.id ASC
	`, r.playlistItemsTable, r.musicFilesTable)

	rows, err := r.db.Query(query, playlistID, userAccount)
	if err != nil {
		return nil, nil, fmt.Errorf("查询歌单歌曲失败: %w", err)
	}
	defer rows.Close()

	var items []model.PlaylistItemRecord
	for rows.Next() {
		var item model.PlaylistItemRecord
		if err := rows.Scan(
			&item.ID,
			&item.PlaylistID,
			&item.UserAccount,
			&item.Position,
			&item.MusicPath,
			&item.MusicTitle,
			&item.Artist,
			&item.Album,
			&item.DurationSec,
			&item.IsLocal,
			&item.CoverArtPath,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("扫描歌单歌曲失败: %w", err)
		}
		items = append(items, item)
	}

	return &playlist, items, nil
}

func (r *PlaylistRepository) AddPlaylistItems(userAccount string, playlistID int64, items []model.PlaylistTrackInput) (int64, int64, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("开启事务失败: %w", err)
	}
	defer rollbackQuietly(tx)

	if _, err := r.mustGetPlaylistTx(tx, userAccount, playlistID); err != nil {
		return 0, 0, err
	}

	existingPaths, maxPosition, err := r.getExistingPathsTx(tx, playlistID, userAccount)
	if err != nil {
		return 0, 0, err
	}

	insertQuery := fmt.Sprintf(`
		INSERT INTO %s
		(playlist_id, user_account, position, music_path, music_title, artist, album, duration_sec, is_local, cover_art_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, r.playlistItemsTable)

	var added int64
	var skipped int64
	seen := make(map[string]struct{}, len(items))
	position := maxPosition
	now := time.Now()

	for _, item := range items {
		path := strings.TrimSpace(item.MusicPath)
		if path == "" {
			skipped++
			continue
		}
		if _, ok := existingPaths[path]; ok {
			skipped++
			continue
		}
		if _, ok := seen[path]; ok {
			skipped++
			continue
		}

		position++
		if _, err := tx.Exec(
			insertQuery,
			playlistID,
			userAccount,
			position,
			path,
			nullableString(item.MusicTitle),
			nullableString(item.Artist),
			nullableString(item.Album),
			item.DurationSec,
			item.IsLocal,
			nullableString(item.CoverArtPath),
			now,
			now,
		); err != nil {
			if isDuplicateEntryError(err) {
				skipped++
				continue
			}
			return 0, 0, fmt.Errorf("添加歌单歌曲失败: %w", err)
		}
		seen[path] = struct{}{}
		added++
	}

	if err := r.refreshPlaylistAggregateTx(tx, playlistID, userAccount); err != nil {
		return 0, 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("提交事务失败: %w", err)
	}
	return added, skipped, nil
}

func (r *PlaylistRepository) RemovePlaylistItems(userAccount string, playlistID int64, musicPaths []string) (int64, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("开启事务失败: %w", err)
	}
	defer rollbackQuietly(tx)

	if _, err := r.mustGetPlaylistTx(tx, userAccount, playlistID); err != nil {
		return 0, err
	}

	uniquePaths := uniqueNonEmptyPaths(musicPaths)
	if len(uniquePaths) == 0 {
		return 0, fmt.Errorf("音乐路径列表不能为空")
	}

	placeholders := make([]string, len(uniquePaths))
	args := make([]interface{}, 0, len(uniquePaths)+2)
	args = append(args, playlistID, userAccount)
	for i, path := range uniquePaths {
		placeholders[i] = "?"
		args = append(args, path)
	}

	deleteQuery := fmt.Sprintf(`
		DELETE FROM %s
		WHERE playlist_id = ? AND user_account = ? AND music_path IN (%s)
	`, r.playlistItemsTable, strings.Join(placeholders, ","))
	result, err := tx.Exec(deleteQuery, args...)
	if err != nil {
		return 0, fmt.Errorf("删除歌单歌曲失败: %w", err)
	}
	deleted, _ := result.RowsAffected()

	if err := r.renumberPlaylistItemsTx(tx, playlistID, userAccount); err != nil {
		return 0, err
	}
	if err := r.refreshPlaylistAggregateTx(tx, playlistID, userAccount); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("提交事务失败: %w", err)
	}
	return deleted, nil
}

func (r *PlaylistRepository) ReorderPlaylistItems(userAccount string, playlistID int64, items []model.PlaylistReorderItem) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer rollbackQuietly(tx)

	if _, err := r.mustGetPlaylistTx(tx, userAccount, playlistID); err != nil {
		return err
	}

	currentItems, err := r.getPlaylistItemsBasicTx(tx, playlistID, userAccount)
	if err != nil {
		return err
	}
	if len(currentItems) == 0 {
		return fmt.Errorf("歌单为空，无法排序")
	}
	if len(items) != len(currentItems) {
		return model.ErrInvalidReorderInput
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Position < items[j].Position
	})

	existing := make(map[string]struct{}, len(currentItems))
	for _, item := range currentItems {
		existing[item.MusicPath] = struct{}{}
	}

	seen := make(map[string]struct{}, len(items))
	for i, item := range items {
		if strings.TrimSpace(item.MusicPath) == "" || item.Position != i+1 {
			return model.ErrInvalidReorderInput
		}
		if _, ok := existing[item.MusicPath]; !ok {
			return model.ErrInvalidReorderInput
		}
		if _, ok := seen[item.MusicPath]; ok {
			return model.ErrInvalidReorderInput
		}
		seen[item.MusicPath] = struct{}{}
	}

	tempShiftQuery := fmt.Sprintf(`UPDATE %s SET position = position + 1000000 WHERE playlist_id = ? AND user_account = ?`, r.playlistItemsTable)
	if _, err := tx.Exec(tempShiftQuery, playlistID, userAccount); err != nil {
		return fmt.Errorf("预处理歌单排序失败: %w", err)
	}

	updateQuery := fmt.Sprintf(`UPDATE %s SET position = ?, updated_at = ? WHERE playlist_id = ? AND user_account = ? AND music_path = ?`, r.playlistItemsTable)
	now := time.Now()
	for _, item := range items {
		if _, err := tx.Exec(updateQuery, item.Position, now, playlistID, userAccount, item.MusicPath); err != nil {
			return fmt.Errorf("更新歌单排序失败: %w", err)
		}
	}

	touchQuery := fmt.Sprintf(`UPDATE %s SET updated_at = ? WHERE id = ? AND user_account = ?`, r.playlistTable)
	if _, err := tx.Exec(touchQuery, now, playlistID, userAccount); err != nil {
		return fmt.Errorf("更新歌单时间失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}

func (r *PlaylistRepository) mustGetPlaylistTx(tx *sql.Tx, userAccount string, playlistID int64) (*model.Playlist, error) {
	query := fmt.Sprintf(`
		SELECT id, user_account, name, COALESCE(description, ''), COALESCE(cover_path, ''), track_count, total_duration_sec, created_at, updated_at
		FROM %s
		WHERE id = ? AND user_account = ?
	`, r.playlistTable)

	var playlist model.Playlist
	if err := tx.QueryRow(query, playlistID, userAccount).Scan(
		&playlist.ID,
		&playlist.UserAccount,
		&playlist.Name,
		&playlist.Description,
		&playlist.CoverPath,
		&playlist.TrackCount,
		&playlist.TotalDurationSec,
		&playlist.CreatedAt,
		&playlist.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrPlaylistNotFound
		}
		return nil, fmt.Errorf("查询歌单失败: %w", err)
	}
	return &playlist, nil
}

func (r *PlaylistRepository) getExistingPathsTx(tx *sql.Tx, playlistID int64, userAccount string) (map[string]struct{}, int, error) {
	query := fmt.Sprintf(`SELECT music_path, position FROM %s WHERE playlist_id = ? AND user_account = ? ORDER BY position ASC`, r.playlistItemsTable)
	rows, err := tx.Query(query, playlistID, userAccount)
	if err != nil {
		return nil, 0, fmt.Errorf("查询已有歌单歌曲失败: %w", err)
	}
	defer rows.Close()

	result := make(map[string]struct{})
	maxPosition := 0
	for rows.Next() {
		var musicPath string
		var position int
		if err := rows.Scan(&musicPath, &position); err != nil {
			return nil, 0, fmt.Errorf("扫描歌单歌曲失败: %w", err)
		}
		result[musicPath] = struct{}{}
		if position > maxPosition {
			maxPosition = position
		}
	}
	return result, maxPosition, nil
}

func (r *PlaylistRepository) getPlaylistItemsBasicTx(tx *sql.Tx, playlistID int64, userAccount string) ([]model.PlaylistItemRecord, error) {
	query := fmt.Sprintf(`SELECT id, music_path, position FROM %s WHERE playlist_id = ? AND user_account = ? ORDER BY position ASC, id ASC`, r.playlistItemsTable)
	rows, err := tx.Query(query, playlistID, userAccount)
	if err != nil {
		return nil, fmt.Errorf("查询歌单排序信息失败: %w", err)
	}
	defer rows.Close()

	var items []model.PlaylistItemRecord
	for rows.Next() {
		var item model.PlaylistItemRecord
		if err := rows.Scan(&item.ID, &item.MusicPath, &item.Position); err != nil {
			return nil, fmt.Errorf("扫描歌单排序信息失败: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *PlaylistRepository) renumberPlaylistItemsTx(tx *sql.Tx, playlistID int64, userAccount string) error {
	items, err := r.getPlaylistItemsBasicTx(tx, playlistID, userAccount)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	shiftQuery := fmt.Sprintf(`UPDATE %s SET position = position + 1000000 WHERE playlist_id = ? AND user_account = ?`, r.playlistItemsTable)
	if _, err := tx.Exec(shiftQuery, playlistID, userAccount); err != nil {
		return fmt.Errorf("预处理歌单位置失败: %w", err)
	}

	updateQuery := fmt.Sprintf(`UPDATE %s SET position = ?, updated_at = ? WHERE id = ?`, r.playlistItemsTable)
	now := time.Now()
	for idx, item := range items {
		if _, err := tx.Exec(updateQuery, idx+1, now, item.ID); err != nil {
			return fmt.Errorf("重排歌单位置失败: %w", err)
		}
	}
	return nil
}

func (r *PlaylistRepository) refreshPlaylistAggregateTx(tx *sql.Tx, playlistID int64, userAccount string) error {
	query := fmt.Sprintf(`
		UPDATE %s p
		SET
			p.track_count = (
				SELECT COUNT(*)
				FROM %s i
				WHERE i.playlist_id = p.id AND i.user_account = p.user_account
			),
			p.total_duration_sec = (
				SELECT COALESCE(SUM(COALESCE(i.duration_sec, 0)), 0)
				FROM %s i
				WHERE i.playlist_id = p.id AND i.user_account = p.user_account
			),
			p.updated_at = ?
		WHERE p.id = ? AND p.user_account = ?
	`, r.playlistTable, r.playlistItemsTable, r.playlistItemsTable)

	if _, err := tx.Exec(query, time.Now(), playlistID, userAccount); err != nil {
		return fmt.Errorf("刷新歌单聚合信息失败: %w", err)
	}
	return nil
}

func uniqueNonEmptyPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		p := strings.TrimSpace(path)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func rollbackQuietly(tx *sql.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}

func nullableString(v string) interface{} {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return nil
	}
	return trimmed
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
