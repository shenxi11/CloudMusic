package repository

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	chartmodel "music-platform/internal/chart/model"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

type ChartRepository struct {
	db *sql.DB

	profileSchema string
	catalogSchema string

	historyTable    string
	musicFilesTable string
}

func NewChartRepository(db *sql.DB, profileSchema, catalogSchema string) *ChartRepository {
	pSchema := normalizeSchema(profileSchema, "music_users")
	cSchema := normalizeSchema(catalogSchema, pSchema)

	return &ChartRepository{
		db:              db,
		profileSchema:   pSchema,
		catalogSchema:   cSchema,
		historyTable:    qualifiedTable(pSchema, "user_play_history"),
		musicFilesTable: qualifiedTable(cSchema, "music_files"),
	}
}

func (r *ChartRepository) ResolveCatalogTrackMeta(ctx context.Context, musicPath string) (*chartmodel.HotTrackMeta, error) {
	query := fmt.Sprintf(`
		SELECT
			COALESCE(title, ''),
			COALESCE(artist, ''),
			COALESCE(album, ''),
			COALESCE(duration_sec, 0),
			COALESCE(cover_art_path, '')
		FROM %s
		WHERE path = ?
		LIMIT 1
	`, r.musicFilesTable)

	var meta chartmodel.HotTrackMeta
	var coverPath string
	err := r.db.QueryRowContext(ctx, query, musicPath).Scan(
		&meta.Title,
		&meta.Artist,
		&meta.Album,
		&meta.DurationSec,
		&coverPath,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询 catalog 元数据失败: %w", err)
	}

	meta.MusicPath = musicPath
	meta.Source = chartmodel.SourceCatalog
	meta.CoverArtURL = strings.TrimSpace(coverPath)
	return &meta, nil
}

func (r *ChartRepository) ListAllHotTrackStats(ctx context.Context) ([]chartmodel.HotTrackStat, error) {
	normalizedPathExpr := "CASE WHEN h.music_path LIKE 'http%%' THEN SUBSTRING_INDEX(h.music_path, '/uploads/', -1) ELSE h.music_path END"
	query := fmt.Sprintf(`
		SELECT
			%s AS music_path,
			COALESCE(MAX(NULLIF(h.music_title, '')), '') AS music_title,
			COALESCE(MAX(NULLIF(h.artist, '')), '') AS artist,
			COALESCE(MAX(NULLIF(h.album, '')), '') AS album,
			COALESCE(MAX(h.duration_sec), 0) AS duration_sec,
			COALESCE(MAX(m.cover_art_path), '') AS cover_art_path,
			COUNT(*) AS play_count,
			MAX(h.play_time) AS last_play_time
		FROM %s h
		LEFT JOIN %s m ON %s = m.path
		WHERE h.is_local = 0 AND TRIM(COALESCE(h.music_path, '')) <> ''
		GROUP BY %s
	`, normalizedPathExpr, r.historyTable, r.musicFilesTable, normalizedPathExpr, normalizedPathExpr)

	return r.scanHotTrackStats(ctx, query)
}

func (r *ChartRepository) ListDailyHotTrackStats(ctx context.Context, start time.Time) ([]chartmodel.DailyHotTrackStat, error) {
	normalizedPathExpr := "CASE WHEN h.music_path LIKE 'http%%' THEN SUBSTRING_INDEX(h.music_path, '/uploads/', -1) ELSE h.music_path END"
	query := fmt.Sprintf(`
		SELECT
			DATE_FORMAT(h.play_time, '%%Y%%m%%d') AS bucket_day,
			%s AS music_path,
			COALESCE(MAX(NULLIF(h.music_title, '')), '') AS music_title,
			COALESCE(MAX(NULLIF(h.artist, '')), '') AS artist,
			COALESCE(MAX(NULLIF(h.album, '')), '') AS album,
			COALESCE(MAX(h.duration_sec), 0) AS duration_sec,
			COALESCE(MAX(m.cover_art_path), '') AS cover_art_path,
			COUNT(*) AS play_count,
			MAX(h.play_time) AS last_play_time
		FROM %s h
		LEFT JOIN %s m ON %s = m.path
		WHERE h.is_local = 0
		  AND TRIM(COALESCE(h.music_path, '')) <> ''
		  AND h.play_time >= ?
		GROUP BY bucket_day, %s
	`, normalizedPathExpr, r.historyTable, r.musicFilesTable, normalizedPathExpr, normalizedPathExpr)

	rows, err := r.db.QueryContext(ctx, query, start)
	if err != nil {
		return nil, fmt.Errorf("查询按天热歌统计失败: %w", err)
	}
	defer rows.Close()

	items := make([]chartmodel.DailyHotTrackStat, 0, 128)
	for rows.Next() {
		var item chartmodel.DailyHotTrackStat
		if err := rows.Scan(
			&item.Day,
			&item.MusicPath,
			&item.MusicTitle,
			&item.Artist,
			&item.Album,
			&item.DurationSec,
			&item.CoverArtPath,
			&item.PlayCount,
			&item.LastPlayTime,
		); err != nil {
			return nil, fmt.Errorf("扫描按天热歌统计失败: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历按天热歌统计失败: %w", err)
	}
	return items, nil
}

func (r *ChartRepository) scanHotTrackStats(ctx context.Context, query string, args ...interface{}) ([]chartmodel.HotTrackStat, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询热歌统计失败: %w", err)
	}
	defer rows.Close()

	items := make([]chartmodel.HotTrackStat, 0, 128)
	for rows.Next() {
		var item chartmodel.HotTrackStat
		if err := rows.Scan(
			&item.MusicPath,
			&item.MusicTitle,
			&item.Artist,
			&item.Album,
			&item.DurationSec,
			&item.CoverArtPath,
			&item.PlayCount,
			&item.LastPlayTime,
		); err != nil {
			return nil, fmt.Errorf("扫描热歌统计失败: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历热歌统计失败: %w", err)
	}
	return items, nil
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

func qualifiedTable(schema, table string) string {
	return quoteIdent(schema) + "." + quoteIdent(table)
}

func quoteIdent(ident string) string {
	if !identifierPattern.MatchString(ident) {
		return "`music_users`"
	}
	return "`" + ident + "`"
}
