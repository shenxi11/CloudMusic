package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"music-platform/internal/recommend/model"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

type RecommendRepository struct {
	db *sql.DB

	profileSchema string
	catalogSchema string

	feedbackTable string
	modelTable    string
	jobTable      string
	historyTable  string
	favoriteTable string
	musicTable    string
}

func NewRecommendRepository(db *sql.DB, profileSchema, catalogSchema string) *RecommendRepository {
	pSchema := normalizeSchema(profileSchema, "music_profile")
	cSchema := normalizeSchema(catalogSchema, "music_users")

	return &RecommendRepository{
		db:            db,
		profileSchema: pSchema,
		catalogSchema: cSchema,
		feedbackTable: qualifiedTable(pSchema, "user_recommendation_feedback"),
		modelTable:    qualifiedTable(pSchema, "recommendation_model_status"),
		jobTable:      qualifiedTable(pSchema, "recommendation_training_jobs"),
		historyTable:  qualifiedTable(pSchema, "user_play_history"),
		favoriteTable: qualifiedTable(pSchema, "user_favorite_music"),
		musicTable:    qualifiedTable(cSchema, "music_files"),
	}
}

func (r *RecommendRepository) EnsureTables() error {
	if _, err := r.db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s DEFAULT CHARSET=utf8mb4", quoteIdent(r.profileSchema))); err != nil {
		return fmt.Errorf("创建 profile schema 失败: %w", err)
	}

	feedbackDDL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGINT NOT NULL AUTO_INCREMENT,
			user_account VARCHAR(64) NOT NULL,
			song_id VARCHAR(500) NOT NULL,
			event_type VARCHAR(32) NOT NULL,
			play_ms BIGINT NOT NULL DEFAULT 0,
			duration_ms BIGINT NOT NULL DEFAULT 0,
			scene VARCHAR(32) NOT NULL DEFAULT 'home',
			request_id VARCHAR(64) DEFAULT NULL,
			model_version VARCHAR(64) DEFAULT NULL,
			event_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			KEY idx_user_event_time (user_account, event_at),
			KEY idx_song_event_time (song_id, event_at),
			KEY idx_request_id (request_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='推荐反馈行为事件'
	`, r.feedbackTable)
	if _, err := r.db.Exec(feedbackDDL); err != nil {
		return fmt.Errorf("创建推荐反馈表失败: %w", err)
	}

	modelDDL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			model_name VARCHAR(64) NOT NULL,
			model_version VARCHAR(64) NOT NULL,
			status VARCHAR(32) NOT NULL DEFAULT 'ready',
			metrics_json JSON DEFAULT NULL,
			trained_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (model_name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='推荐模型状态'
	`, r.modelTable)
	if _, err := r.db.Exec(modelDDL); err != nil {
		return fmt.Errorf("创建推荐模型状态表失败: %w", err)
	}

	jobDDL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGINT NOT NULL AUTO_INCREMENT,
			task_id VARCHAR(64) NOT NULL,
			model_name VARCHAR(64) NOT NULL,
			model_version VARCHAR(64) DEFAULT NULL,
			force_full TINYINT(1) NOT NULL DEFAULT 0,
			status VARCHAR(32) NOT NULL DEFAULT 'queued',
			trigger_by VARCHAR(64) DEFAULT NULL,
			error_message VARCHAR(512) DEFAULT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			finished_at TIMESTAMP NULL DEFAULT NULL,
			PRIMARY KEY (id),
			UNIQUE KEY uk_task_id (task_id),
			KEY idx_status_created (status, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='推荐模型训练任务'
	`, r.jobTable)
	if _, err := r.db.Exec(jobDDL); err != nil {
		return fmt.Errorf("创建推荐训练任务表失败: %w", err)
	}

	seedSQL := fmt.Sprintf(`
		INSERT INTO %s (model_name, model_version, status, metrics_json, trained_at)
		VALUES ('rule_hybrid', 'rule_hybrid_v1', 'ready', JSON_OBJECT('algo','rule_hybrid','note','bootstrap'), NOW())
		ON DUPLICATE KEY UPDATE
			model_name = model_name
	`, r.modelTable)
	if _, err := r.db.Exec(seedSQL); err != nil {
		return fmt.Errorf("初始化推荐模型状态失败: %w", err)
	}

	return nil
}

func (r *RecommendRepository) ListCandidates(ctx context.Context, limit int) ([]model.SongCandidate, error) {
	if limit <= 0 {
		limit = 400
	}
	query := fmt.Sprintf(`
		SELECT
			path,
			COALESCE(title, ''),
			COALESCE(artist, ''),
			COALESCE(album, ''),
			COALESCE(duration_sec, 0),
			COALESCE(cover_art_path, ''),
			COALESCE(lrc_path, '')
		FROM %s
		WHERE is_audio = 1
		ORDER BY path ASC
		LIMIT ?
	`, r.musicTable)
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("查询候选歌曲失败: %w", err)
	}
	defer rows.Close()

	out := make([]model.SongCandidate, 0, limit)
	for rows.Next() {
		var item model.SongCandidate
		if err := rows.Scan(
			&item.Path,
			&item.Title,
			&item.Artist,
			&item.Album,
			&item.DurationSec,
			&item.CoverArtPath,
			&item.LrcPath,
		); err != nil {
			return nil, fmt.Errorf("扫描候选歌曲失败: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *RecommendRepository) GetSongByPath(ctx context.Context, path string) (*model.SongCandidate, error) {
	query := fmt.Sprintf(`
		SELECT
			path,
			COALESCE(title, ''),
			COALESCE(artist, ''),
			COALESCE(album, ''),
			COALESCE(duration_sec, 0),
			COALESCE(cover_art_path, ''),
			COALESCE(lrc_path, '')
		FROM %s
		WHERE is_audio = 1 AND path = ?
		LIMIT 1
	`, r.musicTable)

	var item model.SongCandidate
	if err := r.db.QueryRowContext(ctx, query, path).Scan(
		&item.Path,
		&item.Title,
		&item.Artist,
		&item.Album,
		&item.DurationSec,
		&item.CoverArtPath,
		&item.LrcPath,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RecommendRepository) GetUserHistoryStats(ctx context.Context, userID string) (map[string]float64, map[string]float64, map[string]struct{}, error) {
	songScore := make(map[string]float64)
	artistScore := make(map[string]float64)
	playedSet := make(map[string]struct{})

	query := fmt.Sprintf(`
		SELECT music_path, COALESCE(artist, ''), COUNT(*) AS c
		FROM %s
		WHERE user_account = ?
		GROUP BY music_path, artist
	`, r.historyTable)
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("查询用户播放历史统计失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var path, artist string
		var count float64
		if err := rows.Scan(&path, &artist, &count); err != nil {
			return nil, nil, nil, fmt.Errorf("扫描用户播放历史统计失败: %w", err)
		}
		songScore[path] += count * 1.2
		artistScore[normalizeArtist(artist)] += count
		playedSet[path] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}

	favQuery := fmt.Sprintf(`
		SELECT music_path, COALESCE(artist, '')
		FROM %s
		WHERE user_account = ?
	`, r.favoriteTable)
	favRows, err := r.db.QueryContext(ctx, favQuery, userID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("查询用户收藏统计失败: %w", err)
	}
	defer favRows.Close()
	for favRows.Next() {
		var path, artist string
		if err := favRows.Scan(&path, &artist); err != nil {
			return nil, nil, nil, fmt.Errorf("扫描用户收藏统计失败: %w", err)
		}
		songScore[path] += 3.0
		artistScore[normalizeArtist(artist)] += 1.5
	}
	if err := favRows.Err(); err != nil {
		return nil, nil, nil, err
	}

	return songScore, artistScore, playedSet, nil
}

func (r *RecommendRepository) GetGlobalHotScore(ctx context.Context, limit int) (map[string]float64, error) {
	if limit <= 0 {
		limit = 2000
	}
	query := fmt.Sprintf(`
		SELECT music_path, COUNT(*) AS c
		FROM %s
		GROUP BY music_path
		ORDER BY c DESC
		LIMIT ?
	`, r.historyTable)
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("查询全局热门统计失败: %w", err)
	}
	defer rows.Close()

	type pair struct {
		path  string
		count float64
	}
	tmp := make([]pair, 0, limit)
	maxCount := 0.0
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.path, &p.count); err != nil {
			return nil, fmt.Errorf("扫描全局热门统计失败: %w", err)
		}
		if p.count > maxCount {
			maxCount = p.count
		}
		tmp = append(tmp, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make(map[string]float64, len(tmp))
	if maxCount <= 0 {
		return out, nil
	}
	for _, p := range tmp {
		out[p.path] = p.count / maxCount
	}
	return out, nil
}

func (r *RecommendRepository) GetUserFeedbackAdjust(ctx context.Context, userID string) (map[string]float64, error) {
	query := fmt.Sprintf(`
		SELECT song_id, event_type, COUNT(*) AS c
		FROM %s
		WHERE user_account = ? AND event_at >= DATE_SUB(NOW(), INTERVAL 90 DAY)
		GROUP BY song_id, event_type
	`, r.feedbackTable)
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("查询推荐反馈统计失败: %w", err)
	}
	defer rows.Close()

	out := make(map[string]float64)
	for rows.Next() {
		var songID, eventType string
		var count float64
		if err := rows.Scan(&songID, &eventType, &count); err != nil {
			return nil, fmt.Errorf("扫描推荐反馈统计失败: %w", err)
		}
		out[songID] += feedbackWeight(eventType) * count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *RecommendRepository) InsertFeedback(ctx context.Context, rec model.FeedbackRecord) error {
	query := fmt.Sprintf(`
		INSERT INTO %s
		(user_account, song_id, event_type, play_ms, duration_ms, scene, request_id, model_version, event_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, r.feedbackTable)
	_, err := r.db.ExecContext(ctx, query,
		rec.UserID,
		rec.SongID,
		rec.EventType,
		rec.PlayMS,
		rec.DurationMS,
		rec.Scene,
		nullIfEmpty(rec.RequestID),
		nullIfEmpty(rec.ModelVersion),
		rec.EventAt,
	)
	if err != nil {
		return fmt.Errorf("写入推荐反馈失败: %w", err)
	}
	return nil
}

func (r *RecommendRepository) TriggerRetrain(ctx context.Context, modelName string, forceFull bool, triggerBy string) (string, string, error) {
	now := time.Now()
	taskID := fmt.Sprintf("rec_task_%d", now.UnixNano())
	modelVersion := fmt.Sprintf("%s_%s", modelName, now.Format("20060102_150405"))

	insertJobSQL := fmt.Sprintf(`
		INSERT INTO %s
		(task_id, model_name, model_version, force_full, status, trigger_by, finished_at)
		VALUES (?, ?, ?, ?, 'success', ?, NOW())
	`, r.jobTable)
	if _, err := r.db.ExecContext(ctx, insertJobSQL, taskID, modelName, modelVersion, forceFull, triggerBy); err != nil {
		return "", "", fmt.Errorf("写入训练任务失败: %w", err)
	}

	metricsJSON := map[string]any{
		"algo":       "rule_hybrid",
		"refreshed":  true,
		"force_full": forceFull,
		"updated_at": now.Format(time.RFC3339),
	}
	metricsBytes, _ := json.Marshal(metricsJSON)

	upsertModelSQL := fmt.Sprintf(`
		INSERT INTO %s (model_name, model_version, status, metrics_json, trained_at)
		VALUES (?, ?, 'ready', ?, NOW())
		ON DUPLICATE KEY UPDATE
			model_version = VALUES(model_version),
			status = VALUES(status),
			metrics_json = VALUES(metrics_json),
			trained_at = VALUES(trained_at)
	`, r.modelTable)
	if _, err := r.db.ExecContext(ctx, upsertModelSQL, modelName, modelVersion, string(metricsBytes)); err != nil {
		return "", "", fmt.Errorf("更新模型状态失败: %w", err)
	}

	return taskID, modelVersion, nil
}

func (r *RecommendRepository) GetModelStatus(ctx context.Context, modelName string) (*model.ModelStatus, error) {
	if strings.TrimSpace(modelName) == "" {
		modelName = "rule_hybrid"
	}
	query := fmt.Sprintf(`
		SELECT model_name, model_version, status, metrics_json, trained_at
		FROM %s
		WHERE model_name = ?
		LIMIT 1
	`, r.modelTable)
	var (
		status      model.ModelStatus
		metricsRaw  sql.NullString
		trainedAtTS sql.NullTime
	)
	if err := r.db.QueryRowContext(ctx, query, modelName).Scan(
		&status.ModelName, &status.ModelVersion, &status.Status, &metricsRaw, &trainedAtTS,
	); err != nil {
		if err == sql.ErrNoRows {
			return &model.ModelStatus{
				ModelName:    modelName,
				ModelVersion: "rule_hybrid_v1",
				Status:       "ready",
				Metrics: map[string]any{
					"algo": "rule_hybrid",
					"note": "default_fallback",
				},
			}, nil
		}
		return nil, fmt.Errorf("查询模型状态失败: %w", err)
	}

	if metricsRaw.Valid && strings.TrimSpace(metricsRaw.String) != "" {
		var metrics map[string]any
		if err := json.Unmarshal([]byte(metricsRaw.String), &metrics); err == nil {
			status.Metrics = metrics
		}
	}
	if trainedAtTS.Valid {
		t := trainedAtTS.Time.Format(time.RFC3339)
		status.TrainedAt = &t
	}
	return &status, nil
}

func feedbackWeight(eventType string) float64 {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "like":
		return 1.2
	case "finish":
		return 0.8
	case "play":
		return 0.3
	case "click":
		return 0.2
	case "share":
		return 1.0
	case "dislike":
		return -1.5
	case "skip":
		return -1.0
	default:
		return 0
	}
}

func normalizeArtist(artist string) string {
	v := strings.TrimSpace(artist)
	if v == "" {
		return "未知歌手"
	}
	return v
}

func normalizeSchema(schema, fallback string) string {
	s := strings.TrimSpace(schema)
	if s == "" {
		s = strings.TrimSpace(fallback)
	}
	if s == "" {
		s = "music_profile"
	}
	if !identifierPattern.MatchString(s) {
		return "music_profile"
	}
	return s
}

func quoteIdent(ident string) string {
	if !identifierPattern.MatchString(ident) {
		return "`music_profile`"
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

func nullIfEmpty(v string) interface{} {
	s := strings.TrimSpace(v)
	if s == "" {
		return nil
	}
	return s
}
