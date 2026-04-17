package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	commentmodel "music-platform/internal/comment/model"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

type CreateCommentInput struct {
	UserAccount        string
	UsernameSnapshot   string
	AvatarPathSnapshot string
	Content            string
}

type CreateReplyInput struct {
	CreateCommentInput
	ReplyToCommentID        int64
	ReplyToUserAccount      string
	ReplyToUsernameSnapshot string
}

type CommentRepository struct {
	db *sql.DB

	profileSchema string
	catalogSchema string

	threadTable     string
	commentTable    string
	musicFilesTable string
}

func NewCommentRepository(db *sql.DB, profileSchema, catalogSchema string) *CommentRepository {
	pSchema := normalizeSchema(profileSchema, "music_profile")
	cSchema := normalizeSchema(catalogSchema, pSchema)
	return &CommentRepository{
		db:              db,
		profileSchema:   pSchema,
		catalogSchema:   cSchema,
		threadTable:     qualifiedTable(pSchema, "music_comment_threads"),
		commentTable:    qualifiedTable(pSchema, "music_comments"),
		musicFilesTable: qualifiedTable(cSchema, "music_files"),
	}
}

func (r *CommentRepository) EnsureTables() error {
	if _, err := r.db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s DEFAULT CHARSET=utf8mb4", quoteIdent(r.profileSchema))); err != nil {
		return fmt.Errorf("创建 profile schema 失败: %w", err)
	}

	threadDDL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGINT NOT NULL AUTO_INCREMENT,
			music_path VARCHAR(500) NOT NULL COMMENT '歌曲线程主标识',
			source VARCHAR(32) NOT NULL DEFAULT 'catalog' COMMENT '歌曲来源',
			source_id VARCHAR(64) DEFAULT NULL COMMENT '外部源真实ID',
			music_title VARCHAR(255) DEFAULT NULL COMMENT '歌曲标题',
			artist VARCHAR(255) DEFAULT NULL COMMENT '歌手',
			cover_art_path VARCHAR(500) DEFAULT NULL COMMENT '封面路径',
			root_comment_count INT NOT NULL DEFAULT 0 COMMENT '主评论数',
			total_comment_count INT NOT NULL DEFAULT 0 COMMENT '总评论数',
			last_commented_at TIMESTAMP NULL DEFAULT NULL COMMENT '最后评论时间',
			created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
			updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
			PRIMARY KEY (id),
			UNIQUE KEY uk_music_path (music_path),
			KEY idx_source_source_id (source, source_id),
			KEY idx_last_commented_at (last_commented_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='歌曲评论线程'
	`, r.threadTable)
	if _, err := r.db.Exec(threadDDL); err != nil {
		return fmt.Errorf("创建评论线程表失败: %w", err)
	}

	commentDDL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGINT NOT NULL AUTO_INCREMENT,
			thread_id BIGINT NOT NULL COMMENT '评论线程ID',
			root_comment_id BIGINT NOT NULL DEFAULT 0 COMMENT '所属主评论ID，主评论固定为0',
			reply_to_comment_id BIGINT DEFAULT NULL COMMENT '本次回复目标评论ID',
			user_account VARCHAR(50) NOT NULL COMMENT '评论用户账号',
			username_snapshot VARCHAR(100) NOT NULL COMMENT '评论用户名快照',
			avatar_path_snapshot VARCHAR(500) DEFAULT NULL COMMENT '评论头像快照',
			reply_to_user_account VARCHAR(50) DEFAULT NULL COMMENT '被回复用户账号',
			reply_to_username_snapshot VARCHAR(100) DEFAULT NULL COMMENT '被回复用户名快照',
			content TEXT NOT NULL COMMENT '评论内容',
			is_deleted TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否已删除',
			created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
			updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
			deleted_at TIMESTAMP NULL DEFAULT NULL COMMENT '删除时间',
			PRIMARY KEY (id),
			KEY idx_thread_root_created (thread_id, root_comment_id, created_at),
			KEY idx_root_created (root_comment_id, created_at),
			KEY idx_user_created (user_account, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='歌曲评论与回复'
	`, r.commentTable)
	if _, err := r.db.Exec(commentDDL); err != nil {
		return fmt.Errorf("创建评论表失败: %w", err)
	}

	return nil
}

func (r *CommentRepository) ResolveCatalogTrack(musicPath string) (*commentmodel.TrackMeta, error) {
	query := fmt.Sprintf(`
		SELECT path, COALESCE(title, ''), COALESCE(artist, ''), COALESCE(cover_art_path, '')
		FROM %s
		WHERE path = ?
	`, r.musicFilesTable)

	var meta commentmodel.TrackMeta
	meta.Source = "catalog"
	if err := r.db.QueryRow(query, musicPath).Scan(&meta.MusicPath, &meta.MusicTitle, &meta.Artist, &meta.CoverArtPath); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, commentmodel.ErrOnlineMusicOnly
		}
		return nil, fmt.Errorf("查询歌曲元数据失败: %w", err)
	}
	return &meta, nil
}

func (r *CommentRepository) UpsertThread(meta commentmodel.TrackMeta) (*commentmodel.CommentThread, error) {
	now := time.Now()
	query := fmt.Sprintf(`
		INSERT INTO %s
		(music_path, source, source_id, music_title, artist, cover_art_path, root_comment_count, total_comment_count, last_commented_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, 0, NULL, ?, ?)
		ON DUPLICATE KEY UPDATE
			source = CASE WHEN VALUES(source) <> '' THEN VALUES(source) ELSE source END,
			source_id = CASE WHEN VALUES(source_id) <> '' THEN VALUES(source_id) ELSE source_id END,
			music_title = CASE WHEN VALUES(music_title) <> '' THEN VALUES(music_title) ELSE music_title END,
			artist = CASE WHEN VALUES(artist) <> '' THEN VALUES(artist) ELSE artist END,
			cover_art_path = CASE WHEN VALUES(cover_art_path) <> '' THEN VALUES(cover_art_path) ELSE cover_art_path END,
			updated_at = VALUES(updated_at)
	`, r.threadTable)
	if _, err := r.db.Exec(query,
		meta.MusicPath,
		meta.Source,
		nullableString(meta.SourceID),
		nullableString(meta.MusicTitle),
		nullableString(meta.Artist),
		nullableString(meta.CoverArtPath),
		now,
		now,
	); err != nil {
		return nil, fmt.Errorf("写入评论线程失败: %w", err)
	}
	return r.FindThreadByMusicPath(meta.MusicPath)
}

func (r *CommentRepository) FindThreadByMusicPath(musicPath string) (*commentmodel.CommentThread, error) {
	query := fmt.Sprintf(`
		SELECT id, music_path, source, COALESCE(source_id, ''), COALESCE(music_title, ''), COALESCE(artist, ''), COALESCE(cover_art_path, ''),
		       root_comment_count, total_comment_count, last_commented_at, created_at, updated_at
		FROM %s
		WHERE music_path = ?
	`, r.threadTable)

	var thread commentmodel.CommentThread
	var lastCommentedAt sql.NullTime
	if err := r.db.QueryRow(query, musicPath).Scan(
		&thread.ID,
		&thread.MusicPath,
		&thread.Source,
		&thread.SourceID,
		&thread.MusicTitle,
		&thread.Artist,
		&thread.CoverArtPath,
		&thread.RootCommentCount,
		&thread.TotalCommentCount,
		&lastCommentedAt,
		&thread.CreatedAt,
		&thread.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, commentmodel.ErrCommentNotFound
		}
		return nil, fmt.Errorf("查询评论线程失败: %w", err)
	}
	if lastCommentedAt.Valid {
		thread.LastCommentedAt = &lastCommentedAt.Time
	}
	return &thread, nil
}

func (r *CommentRepository) GetCommentByID(commentID int64) (*commentmodel.CommentRecord, error) {
	query := fmt.Sprintf(`
		SELECT id, thread_id, root_comment_id, reply_to_comment_id, user_account,
		       COALESCE(username_snapshot, ''), COALESCE(avatar_path_snapshot, ''),
		       COALESCE(reply_to_user_account, ''), COALESCE(reply_to_username_snapshot, ''),
		       content, is_deleted, created_at, updated_at, deleted_at
		FROM %s
		WHERE id = ?
	`, r.commentTable)
	return r.scanCommentRow(r.db.QueryRow(query, commentID))
}

func (r *CommentRepository) ListRootComments(threadID int64, page, pageSize int) ([]commentmodel.CommentRecord, int, error) {
	totalQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE thread_id = ? AND root_comment_id = 0`, r.commentTable)
	var total int
	if err := r.db.QueryRow(totalQuery, threadID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询主评论总数失败: %w", err)
	}

	offset := (page - 1) * pageSize
	query := fmt.Sprintf(`
		SELECT c.id, c.thread_id, c.root_comment_id, c.reply_to_comment_id, c.user_account,
		       COALESCE(c.username_snapshot, ''), COALESCE(c.avatar_path_snapshot, ''),
		       COALESCE(c.reply_to_user_account, ''), COALESCE(c.reply_to_username_snapshot, ''),
		       c.content, c.is_deleted, c.created_at, c.updated_at, c.deleted_at,
		       (SELECT COUNT(*) FROM %s r WHERE r.root_comment_id = c.id) AS reply_count
		FROM %s c
		WHERE c.thread_id = ? AND c.root_comment_id = 0
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT ? OFFSET ?
	`, r.commentTable, r.commentTable)
	rows, err := r.db.Query(query, threadID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询主评论列表失败: %w", err)
	}
	defer rows.Close()

	items := make([]commentmodel.CommentRecord, 0, pageSize)
	for rows.Next() {
		item, err := r.scanCommentRows(rows, true)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, nil
}

func (r *CommentRepository) ListReplies(rootCommentID int64, page, pageSize int) ([]commentmodel.CommentRecord, int, error) {
	totalQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE root_comment_id = ?`, r.commentTable)
	var total int
	if err := r.db.QueryRow(totalQuery, rootCommentID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("查询回复总数失败: %w", err)
	}

	offset := (page - 1) * pageSize
	query := fmt.Sprintf(`
		SELECT id, thread_id, root_comment_id, reply_to_comment_id, user_account,
		       COALESCE(username_snapshot, ''), COALESCE(avatar_path_snapshot, ''),
		       COALESCE(reply_to_user_account, ''), COALESCE(reply_to_username_snapshot, ''),
		       content, is_deleted, created_at, updated_at, deleted_at
		FROM %s
		WHERE root_comment_id = ?
		ORDER BY created_at ASC, id ASC
		LIMIT ? OFFSET ?
	`, r.commentTable)
	rows, err := r.db.Query(query, rootCommentID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询回复列表失败: %w", err)
	}
	defer rows.Close()

	items := make([]commentmodel.CommentRecord, 0, pageSize)
	for rows.Next() {
		item, err := r.scanCommentRows(rows, false)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, nil
}

func (r *CommentRepository) CreateRootComment(threadID int64, input CreateCommentInput) (*commentmodel.CommentRecord, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("开启事务失败: %w", err)
	}
	defer rollbackQuietly(tx)

	now := time.Now()
	insertQuery := fmt.Sprintf(`
		INSERT INTO %s
		(thread_id, root_comment_id, reply_to_comment_id, user_account, username_snapshot, avatar_path_snapshot, reply_to_user_account, reply_to_username_snapshot, content, is_deleted, created_at, updated_at, deleted_at)
		VALUES (?, 0, NULL, ?, ?, ?, NULL, NULL, ?, 0, ?, ?, NULL)
	`, r.commentTable)
	result, err := tx.Exec(insertQuery, threadID, input.UserAccount, input.UsernameSnapshot, nullableString(input.AvatarPathSnapshot), input.Content, now, now)
	if err != nil {
		return nil, fmt.Errorf("创建主评论失败: %w", err)
	}
	commentID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("获取评论ID失败: %w", err)
	}

	updateThread := fmt.Sprintf(`
		UPDATE %s
		SET root_comment_count = root_comment_count + 1,
		    total_comment_count = total_comment_count + 1,
		    last_commented_at = ?,
		    updated_at = ?
		WHERE id = ?
	`, r.threadTable)
	if _, err := tx.Exec(updateThread, now, now, threadID); err != nil {
		return nil, fmt.Errorf("更新评论线程统计失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}
	return r.GetCommentByID(commentID)
}

func (r *CommentRepository) CreateReply(threadID, rootCommentID int64, input CreateReplyInput) (*commentmodel.CommentRecord, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("开启事务失败: %w", err)
	}
	defer rollbackQuietly(tx)

	now := time.Now()
	insertQuery := fmt.Sprintf(`
		INSERT INTO %s
		(thread_id, root_comment_id, reply_to_comment_id, user_account, username_snapshot, avatar_path_snapshot, reply_to_user_account, reply_to_username_snapshot, content, is_deleted, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, NULL)
	`, r.commentTable)
	result, err := tx.Exec(
		insertQuery,
		threadID,
		rootCommentID,
		input.ReplyToCommentID,
		input.UserAccount,
		input.UsernameSnapshot,
		nullableString(input.AvatarPathSnapshot),
		nullableString(input.ReplyToUserAccount),
		nullableString(input.ReplyToUsernameSnapshot),
		input.Content,
		now,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("创建回复失败: %w", err)
	}
	commentID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("获取回复ID失败: %w", err)
	}

	updateThread := fmt.Sprintf(`
		UPDATE %s
		SET total_comment_count = total_comment_count + 1,
		    last_commented_at = ?,
		    updated_at = ?
		WHERE id = ?
	`, r.threadTable)
	if _, err := tx.Exec(updateThread, now, now, threadID); err != nil {
		return nil, fmt.Errorf("更新评论线程统计失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}
	return r.GetCommentByID(commentID)
}

func (r *CommentRepository) SoftDeleteComment(commentID int64, userAccount string) (*commentmodel.CommentRecord, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("开启事务失败: %w", err)
	}
	defer rollbackQuietly(tx)

	query := fmt.Sprintf(`
		SELECT id, thread_id, root_comment_id, reply_to_comment_id, user_account,
		       COALESCE(username_snapshot, ''), COALESCE(avatar_path_snapshot, ''),
		       COALESCE(reply_to_user_account, ''), COALESCE(reply_to_username_snapshot, ''),
		       content, is_deleted, created_at, updated_at, deleted_at
		FROM %s
		WHERE id = ?
	`, r.commentTable)
	record, err := r.scanCommentRow(tx.QueryRow(query, commentID))
	if err != nil {
		if errors.Is(err, commentmodel.ErrCommentNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("查询评论失败: %w", err)
	}

	if record.UserAccount != userAccount {
		return nil, commentmodel.ErrDeleteForbidden
	}
	if record.IsDeleted {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("提交事务失败: %w", err)
		}
		return record, nil
	}

	now := time.Now()
	updateComment := fmt.Sprintf(`
		UPDATE %s
		SET is_deleted = 1, deleted_at = ?, updated_at = ?
		WHERE id = ?
	`, r.commentTable)
	if _, err := tx.Exec(updateComment, now, now, commentID); err != nil {
		return nil, fmt.Errorf("删除评论失败: %w", err)
	}

	rootDelta := 0
	if record.RootCommentID == 0 {
		rootDelta = 1
	}
	updateThread := fmt.Sprintf(`
		UPDATE %s
		SET root_comment_count = GREATEST(root_comment_count - ?, 0),
		    total_comment_count = GREATEST(total_comment_count - 1, 0),
		    updated_at = ?
		WHERE id = ?
	`, r.threadTable)
	if _, err := tx.Exec(updateThread, rootDelta, now, record.ThreadID); err != nil {
		return nil, fmt.Errorf("更新评论线程统计失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}
	return r.GetCommentByID(commentID)
}

func (r *CommentRepository) scanCommentRows(rows *sql.Rows, withReplyCount bool) (*commentmodel.CommentRecord, error) {
	var record commentmodel.CommentRecord
	var replyToCommentID sql.NullInt64
	var deletedAt sql.NullTime
	if withReplyCount {
		if err := rows.Scan(
			&record.ID,
			&record.ThreadID,
			&record.RootCommentID,
			&replyToCommentID,
			&record.UserAccount,
			&record.UsernameSnapshot,
			&record.AvatarPathSnapshot,
			&record.ReplyToUserAccount,
			&record.ReplyToUsername,
			&record.Content,
			&record.IsDeleted,
			&record.CreatedAt,
			&record.UpdatedAt,
			&deletedAt,
			&record.ReplyCount,
		); err != nil {
			return nil, fmt.Errorf("扫描评论失败: %w", err)
		}
	} else {
		if err := rows.Scan(
			&record.ID,
			&record.ThreadID,
			&record.RootCommentID,
			&replyToCommentID,
			&record.UserAccount,
			&record.UsernameSnapshot,
			&record.AvatarPathSnapshot,
			&record.ReplyToUserAccount,
			&record.ReplyToUsername,
			&record.Content,
			&record.IsDeleted,
			&record.CreatedAt,
			&record.UpdatedAt,
			&deletedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描评论失败: %w", err)
		}
	}
	if replyToCommentID.Valid {
		value := replyToCommentID.Int64
		record.ReplyToCommentID = &value
	}
	if deletedAt.Valid {
		record.DeletedAt = &deletedAt.Time
	}
	return &record, nil
}

func (r *CommentRepository) scanCommentRow(row *sql.Row) (*commentmodel.CommentRecord, error) {
	var record commentmodel.CommentRecord
	var replyToCommentID sql.NullInt64
	var deletedAt sql.NullTime
	if err := row.Scan(
		&record.ID,
		&record.ThreadID,
		&record.RootCommentID,
		&replyToCommentID,
		&record.UserAccount,
		&record.UsernameSnapshot,
		&record.AvatarPathSnapshot,
		&record.ReplyToUserAccount,
		&record.ReplyToUsername,
		&record.Content,
		&record.IsDeleted,
		&record.CreatedAt,
		&record.UpdatedAt,
		&deletedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, commentmodel.ErrCommentNotFound
		}
		return nil, err
	}
	if replyToCommentID.Valid {
		value := replyToCommentID.Int64
		record.ReplyToCommentID = &value
	}
	if deletedAt.Valid {
		record.DeletedAt = &deletedAt.Time
	}
	return &record, nil
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
