package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"music-platform/internal/common/logger"
	"music-platform/internal/music/compat"
	"music-platform/internal/music/external"
	"music-platform/pkg/response"
)

var mediaIdentifierPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// MediaHandler 媒体文件处理器
type MediaHandler struct {
	uploadDir  string
	db         *sql.DB
	httpClient *http.Client

	jamendoService external.JamendoService
	seekIndexCache *seekIndexCacheStore
	ffprobeBinary  string

	mediaSchema       string
	catalogSchema     string
	mediaLyricsTable  string
	catalogMusicTable string
}

// NewMediaHandler 创建媒体处理器
func NewMediaHandler(uploadDir string, db *sql.DB, mediaSchema, catalogSchema string, jamendoService external.JamendoService) *MediaHandler {
	mSchema := normalizeMediaSchema(mediaSchema, "music_media")
	cSchema := normalizeMediaSchema(catalogSchema, "music_users")
	return &MediaHandler{
		uploadDir:         uploadDir,
		db:                db,
		httpClient:        http.DefaultClient,
		jamendoService:    jamendoService,
		seekIndexCache:    newSeekIndexCacheStore(),
		ffprobeBinary:     "ffprobe",
		mediaSchema:       mSchema,
		catalogSchema:     cSchema,
		mediaLyricsTable:  qualifiedMediaTable(mSchema, "media_lyrics_map"),
		catalogMusicTable: qualifiedMediaTable(cSchema, "music_files"),
	}
}

// EnsureTables 初始化媒体服务私有表
func (h *MediaHandler) EnsureTables() error {
	createSchema := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s DEFAULT CHARSET=utf8mb4", quoteMediaIdent(h.mediaSchema))
	if _, err := h.db.Exec(createSchema); err != nil {
		return fmt.Errorf("创建 media schema 失败: %w", err)
	}

	createTable := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGINT NOT NULL AUTO_INCREMENT,
			music_path VARCHAR(500) NOT NULL,
			lrc_path VARCHAR(500) NOT NULL,
			source VARCHAR(32) NOT NULL DEFAULT 'catalog',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uk_music_path (music_path),
			KEY idx_lrc_path (lrc_path),
			KEY idx_updated_at (updated_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='媒体服务歌词索引'
	`, h.mediaLyricsTable)
	if _, err := h.db.Exec(createTable); err != nil {
		return fmt.Errorf("创建 media_lyrics_map 失败: %w", err)
	}
	return nil
}

// SyncLyricsMap 从 catalog 元数据同步歌词映射（幂等）
func (h *MediaHandler) SyncLyricsMap() error {
	query := fmt.Sprintf(`
		INSERT INTO %s (music_path, lrc_path, source)
		SELECT m.path, m.lrc_path, 'catalog'
		FROM %s m
		WHERE m.lrc_path IS NOT NULL AND m.lrc_path <> ''
		ON DUPLICATE KEY UPDATE
			lrc_path = VALUES(lrc_path),
			source = 'catalog',
			updated_at = CURRENT_TIMESTAMP
	`, h.mediaLyricsTable, h.catalogMusicTable)
	if _, err := h.db.Exec(query); err != nil {
		return fmt.Errorf("同步歌词映射失败: %w", err)
	}
	return nil
}

// ServeFile 服务文件（音频、封面、歌词）
func (h *MediaHandler) ServeFile(w http.ResponseWriter, r *http.Request) {
	urlPath := strings.TrimPrefix(r.URL.Path, "/uploads/")
	if urlPath == "" {
		response.BadRequest(w, "文件路径不能为空")
		return
	}

	if strings.HasSuffix(urlPath, "/lrc") {
		h.serveLRC(w, r, urlPath)
		return
	}

	h.serveRegularFile(w, r, urlPath)
}

// serveLRC 服务歌词文件
func (h *MediaHandler) serveLRC(w http.ResponseWriter, r *http.Request, urlPath string) {
	folderPath := strings.TrimSuffix(urlPath, "/lrc")

	lrcPath, err := h.findLRCPath(r.Context(), folderPath)
	if err != nil {
		response.NotFound(w, "歌词文件不存在")
		return
	}

	fullPath := filepath.Join(h.uploadDir, lrcPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		logger.Error("读取歌词文件失败 '%s': %v", fullPath, err)
		response.InternalServerError(w, "读取歌词文件失败")
		return
	}

	lines := strings.Split(string(content), "\n")
	var cleanLines []string
	for _, line := range lines {
		cleanLines = append(cleanLines, strings.TrimSpace(line))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cleanLines)
}

func (h *MediaHandler) findLRCPath(ctx context.Context, folderPath string) (string, error) {
	var lrcPath string
	queryMedia := fmt.Sprintf(`
		SELECT lrc_path
		FROM %s
		WHERE music_path LIKE ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, h.mediaLyricsTable)
	err := h.db.QueryRowContext(ctx, queryMedia, folderPath+"%").Scan(&lrcPath)
	if err == nil && strings.TrimSpace(lrcPath) != "" {
		return lrcPath, nil
	}
	if err != nil && err != sql.ErrNoRows {
		logger.Warn("查询 media_lyrics_map 失败，回退 catalog: %v", err)
	}

	var musicPath string
	queryCatalog := fmt.Sprintf(`
		SELECT path, lrc_path
		FROM %s
		WHERE path LIKE ? AND lrc_path IS NOT NULL AND lrc_path <> ''
		LIMIT 1
	`, h.catalogMusicTable)
	err = h.db.QueryRowContext(ctx, queryCatalog, folderPath+"%").Scan(&musicPath, &lrcPath)
	if err != nil {
		return "", err
	}

	upsert := fmt.Sprintf(`
		INSERT INTO %s (music_path, lrc_path, source)
		VALUES (?, ?, 'catalog')
		ON DUPLICATE KEY UPDATE
			lrc_path = VALUES(lrc_path),
			source = 'catalog',
			updated_at = CURRENT_TIMESTAMP
	`, h.mediaLyricsTable)
	if _, upErr := h.db.ExecContext(ctx, upsert, musicPath, lrcPath); upErr != nil {
		logger.Warn("回填 media_lyrics_map 失败: %v", upErr)
	}

	return lrcPath, nil
}

// serveRegularFile 服务普通文件（音频、图片）
func (h *MediaHandler) serveRegularFile(w http.ResponseWriter, r *http.Request, urlPath string) {
	fullPath := filepath.Join(h.uploadDir, urlPath)

	fi, err := os.Stat(fullPath)
	if err != nil || fi.IsDir() {
		response.NotFound(w, "文件不存在")
		return
	}

	f, err := os.Open(fullPath)
	if err != nil {
		response.InternalServerError(w, "无法打开文件")
		return
	}
	defer f.Close()

	rs := io.NewSectionReader(f, 0, fi.Size())
	ext := strings.ToLower(filepath.Ext(urlPath))
	contentType := getContentType(ext)
	filename := filepath.Base(urlPath)

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Disposition", "inline; filename="+filename)

	http.ServeContent(w, r, filename, fi.ModTime(), rs)
}

func getContentType(ext string) string {
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".flac":
		return "audio/flac"
	case ".ogg":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	case ".m4a", ".aac":
		return "audio/aac"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		return "application/octet-stream"
	}
}

// Upload 上传文件
func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		response.BadRequest(w, "获取文件失败")
		return
	}
	defer file.Close()

	filePath := filepath.Join(h.uploadDir, handler.Filename)
	dst, err := os.Create(filePath)
	if err != nil {
		response.InternalServerError(w, "创建文件失败")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		response.InternalServerError(w, "保存文件失败")
		return
	}

	response.Success(w, map[string]string{"filePath": filePath})
}

// Download 下载文件（支持断点续传）
func (h *MediaHandler) Download(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/files/")
	if filePath == "" {
		response.BadRequest(w, "文件路径不能为空")
		return
	}

	fullPath := filepath.Join(h.uploadDir, filePath)

	absUploadDir, _ := filepath.Abs(h.uploadDir)
	absFilePath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absFilePath, absUploadDir) {
		response.BadRequest(w, "无效的文件路径")
		return
	}

	fileInfo, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		response.NotFound(w, "文件不存在")
		return
	}
	if err != nil {
		response.InternalServerError(w, "无法访问文件")
		return
	}

	file, err := os.Open(fullPath)
	if err != nil {
		response.InternalServerError(w, "无法打开文件")
		return
	}
	defer file.Close()

	filename := filepath.Base(filePath)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Header().Set("Content-Type", getContentType(strings.ToLower(filepath.Ext(filename))))

	http.ServeContent(w, r, filename, fileInfo.ModTime(), file)
}

// LRC 获取歌词（查询参数版本）
func (h *MediaHandler) LRC(w http.ResponseWriter, r *http.Request) {
	lrcPath := r.URL.Query().Get("path")
	if lrcPath == "" {
		response.BadRequest(w, "path参数不能为空")
		return
	}

	fullPath := filepath.Join(h.uploadDir, lrcPath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		response.NotFound(w, "歌词文件不存在")
		return
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		logger.Error("读取歌词文件失败 '%s': %v", fullPath, err)
		response.InternalServerError(w, "读取歌词文件失败")
		return
	}

	lines := strings.Split(string(content), "\n")
	var cleanLines []string
	for _, line := range lines {
		cleanLines = append(cleanLines, strings.TrimSpace(line))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cleanLines)
}

// DownloadQuery 通过查询参数下载文件（旧版兼容接口）
func (h *MediaHandler) DownloadQuery(w http.ResponseWriter, r *http.Request) {
	var path string

	if r.Method == http.MethodPost {
		var req struct {
			Filename string `json:"filename"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Filename != "" {
			path = req.Filename
		}
	}

	if path == "" {
		path = r.URL.Query().Get("path")
	}

	if path == "" {
		response.BadRequest(w, "path参数或filename参数不能为空")
		return
	}

	if sourceID, ok := compat.ParseJamendoSourceID(path); ok {
		h.downloadJamendo(w, r, path, sourceID)
		return
	}

	r.URL.Path = "/files/" + path
	h.Download(w, r)
}

func (h *MediaHandler) downloadJamendo(w http.ResponseWriter, r *http.Request, virtualPath, sourceID string) {
	if h.jamendoService == nil || !h.jamendoService.IsConfigured() {
		response.Error(w, http.StatusServiceUnavailable, "Jamendo external music source is not configured")
		return
	}

	track, err := h.jamendoService.GetTrack(r.Context(), sourceID)
	if err != nil {
		h.writeJamendoError(w, err)
		return
	}
	if !track.DownloadAllowed {
		response.Error(w, http.StatusForbidden, "Jamendo track download is not allowed")
		return
	}
	if strings.TrimSpace(track.StreamURL) == "" {
		response.Error(w, http.StatusBadGateway, "Jamendo upstream request failed")
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, track.StreamURL, nil)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "Jamendo upstream request failed")
		return
	}

	res, err := h.httpClient.Do(req)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "Jamendo upstream request failed")
		return
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		response.Error(w, http.StatusBadGateway, "Jamendo upstream request failed")
		return
	}

	contentType := res.Header.Get("Content-Type")
	if strings.TrimSpace(contentType) == "" {
		contentType = "audio/mpeg"
	}

	filename := filepath.Base(virtualPath)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Cache-Control", "no-store")
	if contentLength := res.Header.Get("Content-Length"); contentLength != "" {
		w.Header().Set("Content-Length", contentLength)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, res.Body)
}

func (h *MediaHandler) writeJamendoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, external.ErrNotConfigured):
		response.Error(w, http.StatusServiceUnavailable, "Jamendo external music source is not configured")
	case errors.Is(err, external.ErrNotFound), errors.Is(err, sql.ErrNoRows):
		response.NotFound(w, "Jamendo track not found")
	case errors.Is(err, external.ErrUpstream):
		response.Error(w, http.StatusBadGateway, "Jamendo upstream request failed")
	default:
		response.InternalServerError(w, "Jamendo external music request failed")
	}
}

func normalizeMediaSchema(schema, fallback string) string {
	s := strings.TrimSpace(schema)
	if s == "" {
		s = strings.TrimSpace(fallback)
	}
	if s == "" {
		s = "music_users"
	}
	if !mediaIdentifierPattern.MatchString(s) {
		return "music_users"
	}
	return s
}

func quoteMediaIdent(ident string) string {
	if !mediaIdentifierPattern.MatchString(ident) {
		return "`music_users`"
	}
	return "`" + ident + "`"
}

func qualifiedMediaTable(schema, table string) string {
	t := strings.TrimSpace(table)
	if !mediaIdentifierPattern.MatchString(t) {
		t = "unknown_table"
	}
	return quoteMediaIdent(schema) + "." + "`" + t + "`"
}
