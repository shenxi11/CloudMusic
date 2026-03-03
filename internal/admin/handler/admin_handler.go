package handler

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"music-platform/internal/common/cache"
	"music-platform/internal/common/config"
	"music-platform/internal/common/logger"
	"music-platform/pkg/response"
)

const (
	adminCookieName     = "admin_session"
	defaultAdminUser    = "admin"
	defaultAdminPass    = "admin123456"
	defaultSessionMins  = 24 * 60
	defaultUploadMaxMB  = 300
	userOnlineTTL       = 10 * time.Minute
	userOnlineKeyPrefix = "user:online:account:"
)

var (
	rePathUnsafe      = regexp.MustCompile(`[\\/:*?"<>|]`)
	validSchemaIdent  = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	audioExtAllowList = map[string]struct{}{
		".mp3":  {},
		".flac": {},
		".wav":  {},
		".ogg":  {},
		".m4a":  {},
		".aac":  {},
	}
)

//go:embed web/admin.html
var adminWebFS embed.FS

type AdminHandler struct {
	db            *sql.DB
	uploadDir     string
	videoDir      string
	catalogSchema string
	mediaSchema   string

	adminUsername string
	adminPassword string
	sessionTTL    time.Duration
	secureCookie  bool
}

type adminLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type adminUserItem struct {
	Account    string `json:"account"`
	Username   string `json:"username"`
	Online     bool   `json:"online"`
	LastSeenAt int64  `json:"last_seen_at"`
}

type mediaItem struct {
	Path        string  `json:"path"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Album       string  `json:"album"`
	DurationSec float64 `json:"duration_sec"`
	SizeBytes   int64   `json:"size_bytes"`
	FileType    string  `json:"file_type"`
	IsAudio     bool    `json:"is_audio"`
	LrcPath     string  `json:"lrc_path,omitempty"`
	CoverPath   string  `json:"cover_art_path,omitempty"`
	UpdatedAt   string  `json:"updated_at"`
}

type mediaDeleteRequest struct {
	Paths       []string `json:"paths"`
	DeleteFiles bool     `json:"delete_files"`
}

type mediaDeleteResponse struct {
	DeletedRows  int64    `json:"deleted_rows"`
	DeletedFiles []string `json:"deleted_files"`
	Warnings     []string `json:"warnings"`
}

type mediaArtistGroup struct {
	Artist string      `json:"artist"`
	Count  int         `json:"count"`
	Items  []mediaItem `json:"items"`
}

type ffprobeFormat struct {
	Duration string            `json:"duration"`
	Tags     map[string]string `json:"tags"`
}

type ffprobeResult struct {
	Format ffprobeFormat `json:"format"`
}

type mediaRecordForDelete struct {
	Path      string
	IsAudio   bool
	LrcPath   string
	CoverPath string
}

func NewAdminHandler(cfg *config.Config, db *sql.DB) *AdminHandler {
	adminUser := strings.TrimSpace(cfg.Admin.Username)
	if adminUser == "" {
		adminUser = defaultAdminUser
	}
	adminPass := strings.TrimSpace(cfg.Admin.Password)
	if adminPass == "" {
		adminPass = defaultAdminPass
	}
	sessionMins := cfg.Admin.SessionTTLMinutes
	if sessionMins <= 0 {
		sessionMins = defaultSessionMins
	}

	catalogSchema := strings.TrimSpace(cfg.Schemas.Catalog)
	if catalogSchema == "" {
		catalogSchema = strings.TrimSpace(cfg.Database.Name)
	}
	if catalogSchema == "" {
		catalogSchema = "music_users"
	}

	mediaSchema := strings.TrimSpace(cfg.Schemas.Media)
	if mediaSchema == "" {
		mediaSchema = "music_media"
	}

	return &AdminHandler{
		db:            db,
		uploadDir:     cfg.Server.UploadDir,
		videoDir:      cfg.Server.VideoDir,
		catalogSchema: normalizeSchema(catalogSchema, "music_users"),
		mediaSchema:   normalizeSchema(mediaSchema, "music_media"),
		adminUsername: adminUser,
		adminPassword: adminPass,
		sessionTTL:    time.Duration(sessionMins) * time.Minute,
		secureCookie:  cfg.Server.EnableTLS,
	}
}

func (h *AdminHandler) AdminPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 GET")
		return
	}
	if r.URL.Path != "/admin" && r.URL.Path != "/admin/" {
		http.NotFound(w, r)
		return
	}
	content, err := adminWebFS.ReadFile("web/admin.html")
	if err != nil {
		response.InternalServerError(w, "加载后台页面失败")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(content)
}

func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}

	var req adminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "请求参数错误")
		return
	}
	if req.Username != h.adminUsername || req.Password != h.adminPassword {
		response.Unauthorized(w, "管理员账号或密码错误")
		return
	}

	token, err := generateSessionToken()
	if err != nil {
		response.InternalServerError(w, "创建会话失败")
		return
	}

	rdb := cache.GetClient()
	if rdb == nil {
		response.InternalServerError(w, "Redis 未初始化")
		return
	}
	ctx := cache.GetContext()
	if err := rdb.Set(ctx, h.adminSessionKey(token), h.adminUsername, h.sessionTTL).Err(); err != nil {
		response.InternalServerError(w, "保存会话失败")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     adminCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.sessionTTL.Seconds()),
	})

	response.Success(w, map[string]any{
		"username":    h.adminUsername,
		"session_ttl": int(h.sessionTTL.Seconds()),
	})
}

func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}
	token := h.readSessionToken(r)
	if token != "" {
		rdb := cache.GetClient()
		if rdb != nil {
			_ = rdb.Del(cache.GetContext(), h.adminSessionKey(token)).Err()
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	response.Success(w, map[string]bool{"success": true})
}

func (h *AdminHandler) Session(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 GET")
		return
	}
	adminName, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	response.Success(w, map[string]any{"username": adminName})
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 GET")
		return
	}
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}

	ctx := r.Context()
	query := fmt.Sprintf("SELECT account, username FROM %s ORDER BY username ASC", h.catalogUsersTable())
	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		response.InternalServerError(w, "查询用户失败")
		return
	}
	defer rows.Close()

	users := make([]adminUserItem, 0, 128)
	keys := make([]string, 0, 128)
	for rows.Next() {
		var u adminUserItem
		if err := rows.Scan(&u.Account, &u.Username); err != nil {
			response.InternalServerError(w, "读取用户失败")
			return
		}
		users = append(users, u)
		keys = append(keys, userOnlineKeyPrefix+u.Account)
	}
	if err := rows.Err(); err != nil {
		response.InternalServerError(w, "读取用户失败")
		return
	}

	rdb := cache.GetClient()
	if rdb != nil && len(keys) > 0 {
		vals, err := rdb.MGet(cache.GetContext(), keys...).Result()
		if err == nil {
			nowUnix := time.Now().Unix()
			for i, raw := range vals {
				if raw == nil {
					continue
				}
				s, ok := raw.(string)
				if !ok {
					continue
				}
				ts, err := strconv.ParseInt(s, 10, 64)
				if err != nil {
					continue
				}
				users[i].LastSeenAt = ts
				users[i].Online = nowUnix-ts <= int64(userOnlineTTL.Seconds())
			}
		}
	}

	response.Success(w, users)
}

func (h *AdminHandler) ListMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 GET")
		return
	}
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}

	mediaType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	limit := parsePositiveIntWithDefault(r.URL.Query().Get("limit"), 200)
	if limit > 1000 {
		limit = 1000
	}

	conds := make([]string, 0, 3)
	args := make([]any, 0, 6)
	if mediaType == "audio" {
		conds = append(conds, "is_audio = 1")
	} else if mediaType == "video" {
		conds = append(conds, "is_audio = 0")
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		conds = append(conds, "(path LIKE ? OR title LIKE ? OR artist LIKE ? OR album LIKE ?)")
		args = append(args, like, like, like, like)
	}

	query := fmt.Sprintf("SELECT path, title, artist, album, duration_sec, size_bytes, file_type, is_audio, COALESCE(lrc_path,''), COALESCE(cover_art_path,''), updated_at FROM %s", h.catalogMusicTable())
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		response.InternalServerError(w, "查询媒体失败")
		return
	}
	defer rows.Close()

	items := make([]mediaItem, 0, limit)
	for rows.Next() {
		var m mediaItem
		var tiny int
		var updated time.Time
		if err := rows.Scan(&m.Path, &m.Title, &m.Artist, &m.Album, &m.DurationSec, &m.SizeBytes, &m.FileType, &tiny, &m.LrcPath, &m.CoverPath, &updated); err != nil {
			response.InternalServerError(w, "读取媒体失败")
			return
		}
		m.IsAudio = tiny == 1
		m.UpdatedAt = updated.Format(time.RFC3339)
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		response.InternalServerError(w, "读取媒体失败")
		return
	}

	response.Success(w, items)
}

func (h *AdminHandler) ListMediaByArtist(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 GET")
		return
	}
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}

	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	limit := parsePositiveIntWithDefault(r.URL.Query().Get("limit"), 2000)
	if limit > 8000 {
		limit = 8000
	}

	conds := []string{"is_audio = 1"}
	args := make([]any, 0, 6)
	if keyword != "" {
		like := "%" + keyword + "%"
		conds = append(conds, "(path LIKE ? OR title LIKE ? OR artist LIKE ? OR album LIKE ?)")
		args = append(args, like, like, like, like)
	}

	query := fmt.Sprintf("SELECT path, title, artist, album, duration_sec, size_bytes, file_type, is_audio, COALESCE(lrc_path,''), COALESCE(cover_art_path,''), updated_at FROM %s WHERE %s ORDER BY artist ASC, updated_at DESC LIMIT ?", h.catalogMusicTable(), strings.Join(conds, " AND "))
	args = append(args, limit)

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		response.InternalServerError(w, "查询媒体失败")
		return
	}
	defer rows.Close()

	groupMap := make(map[string][]mediaItem, 128)
	for rows.Next() {
		var m mediaItem
		var tiny int
		var updated time.Time
		if err := rows.Scan(&m.Path, &m.Title, &m.Artist, &m.Album, &m.DurationSec, &m.SizeBytes, &m.FileType, &tiny, &m.LrcPath, &m.CoverPath, &updated); err != nil {
			response.InternalServerError(w, "读取媒体失败")
			return
		}
		m.IsAudio = tiny == 1
		m.UpdatedAt = updated.Format(time.RFC3339)

		artistKey := strings.TrimSpace(m.Artist)
		if artistKey == "" {
			artistKey = "未标注歌手"
			m.Artist = artistKey
		}
		groupMap[artistKey] = append(groupMap[artistKey], m)
	}
	if err := rows.Err(); err != nil {
		response.InternalServerError(w, "读取媒体失败")
		return
	}

	groups := make([]mediaArtistGroup, 0, len(groupMap))
	for artist, items := range groupMap {
		groups = append(groups, mediaArtistGroup{
			Artist: artist,
			Count:  len(items),
			Items:  items,
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Count != groups[j].Count {
			return groups[i].Count > groups[j].Count
		}
		return groups[i].Artist < groups[j].Artist
	})

	response.Success(w, groups)
}

func (h *AdminHandler) BatchDeleteMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}

	var req mediaDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "请求参数错误")
		return
	}
	paths := normalizePathList(req.Paths)
	if len(paths) == 0 {
		response.BadRequest(w, "paths 不能为空")
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		response.InternalServerError(w, "创建事务失败")
		return
	}
	defer func() { _ = tx.Rollback() }()

	records, err := h.fetchMediaRecordsForDelete(r.Context(), tx, paths)
	if err != nil {
		response.InternalServerError(w, "读取待删除媒体失败")
		return
	}

	placeholders := strings.Repeat("?,", len(paths))
	placeholders = strings.TrimSuffix(placeholders, ",")
	args := make([]any, 0, len(paths))
	for _, p := range paths {
		args = append(args, p)
	}

	delQuery := fmt.Sprintf("DELETE FROM %s WHERE path IN (%s)", h.catalogMusicTable(), placeholders)
	result, err := tx.ExecContext(r.Context(), delQuery, args...)
	if err != nil {
		response.InternalServerError(w, "删除媒体失败")
		return
	}
	deletedRows, _ := result.RowsAffected()

	lyricsQuery := fmt.Sprintf("DELETE FROM %s WHERE music_path IN (%s)", h.mediaLyricsTable(), placeholders)
	if _, err := tx.ExecContext(r.Context(), lyricsQuery, args...); err != nil && !strings.Contains(strings.ToLower(err.Error()), "doesn't exist") {
		response.InternalServerError(w, "删除歌词映射失败")
		return
	}

	if err := tx.Commit(); err != nil {
		response.InternalServerError(w, "提交事务失败")
		return
	}

	resp := mediaDeleteResponse{DeletedRows: deletedRows}
	if req.DeleteFiles {
		for _, rec := range records {
			if rec.IsAudio {
				if err := removeRelativeFile(h.uploadDir, rec.Path); err == nil {
					resp.DeletedFiles = append(resp.DeletedFiles, "uploads/"+filepath.ToSlash(rec.Path))
				} else {
					resp.Warnings = append(resp.Warnings, err.Error())
				}
				if rec.LrcPath != "" {
					if err := removeRelativeFile(h.uploadDir, rec.LrcPath); err != nil {
						resp.Warnings = append(resp.Warnings, err.Error())
					}
				}
				if rec.CoverPath != "" {
					if err := removeRelativeFile(h.uploadDir, rec.CoverPath); err != nil {
						resp.Warnings = append(resp.Warnings, err.Error())
					}
				}
			} else {
				if err := removeRelativeFile(h.videoDir, rec.Path); err == nil {
					resp.DeletedFiles = append(resp.DeletedFiles, "video/"+filepath.ToSlash(rec.Path))
				} else {
					resp.Warnings = append(resp.Warnings, err.Error())
				}
			}
		}
	}

	response.Success(w, resp)
}

func (h *AdminHandler) UploadSong(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}

	if err := r.ParseMultipartForm(defaultUploadMaxMB << 20); err != nil {
		response.BadRequest(w, "上传内容过大或格式错误")
		return
	}

	songFile, songHeader, err := openFirstFormFile(r, "song", "file", "audio")
	if err != nil {
		response.BadRequest(w, "请上传歌曲文件（字段: song）")
		return
	}
	defer songFile.Close()

	songFilename := sanitizeFilename(songHeader.Filename)
	ext := strings.ToLower(filepath.Ext(songFilename))
	if _, ok := audioExtAllowList[ext]; !ok {
		response.BadRequest(w, "仅支持 mp3/flac/wav/ogg/m4a/aac")
		return
	}

	baseName := strings.TrimSuffix(songFilename, filepath.Ext(songFilename))
	folder := fmt.Sprintf("%s_%d", sanitizePathPart(baseName), time.Now().Unix())
	folder = strings.Trim(folder, "._-")
	if folder == "" {
		folder = fmt.Sprintf("song_%d", time.Now().Unix())
	}

	if err := os.MkdirAll(filepath.Join(h.uploadDir, folder), os.ModePerm); err != nil {
		response.InternalServerError(w, "创建上传目录失败")
		return
	}

	songRelPath := filepath.ToSlash(filepath.Join(folder, songFilename))
	songAbsPath := filepath.Join(h.uploadDir, filepath.FromSlash(songRelPath))
	if err := saveMultipartFile(songFile, songAbsPath); err != nil {
		response.InternalServerError(w, "保存歌曲失败")
		return
	}

	var lrcRelPath string
	if lrcFile, lrcHeader, err := openFirstFormFile(r, "lrc", "lyrics"); err == nil {
		defer lrcFile.Close()
		lrcName := sanitizeFilename(lrcHeader.Filename)
		if strings.TrimSpace(lrcName) == "" {
			lrcName = sanitizePathPart(baseName) + ".lrc"
		}
		if !strings.HasSuffix(strings.ToLower(lrcName), ".lrc") {
			lrcName += ".lrc"
		}
		lrcRelPath = filepath.ToSlash(filepath.Join(folder, lrcName))
		if err := saveMultipartFile(lrcFile, filepath.Join(h.uploadDir, filepath.FromSlash(lrcRelPath))); err != nil {
			response.InternalServerError(w, "保存歌词失败")
			return
		}
	}

	var coverRelPath string
	if coverFile, coverHeader, err := openFirstFormFile(r, "cover", "cover_image"); err == nil {
		defer coverFile.Close()
		coverName := sanitizeFilename(coverHeader.Filename)
		if strings.TrimSpace(coverName) == "" {
			coverName = sanitizePathPart(baseName) + "_cover.jpg"
		}
		coverRelPath = filepath.ToSlash(filepath.Join(folder, coverName))
		if err := saveMultipartFile(coverFile, filepath.Join(h.uploadDir, filepath.FromSlash(coverRelPath))); err != nil {
			response.InternalServerError(w, "保存封面失败")
			return
		}
	}

	probeMeta, _ := probeAudio(songAbsPath)
	title := strings.TrimSpace(r.FormValue("title"))
	artist := strings.TrimSpace(r.FormValue("artist"))
	album := strings.TrimSpace(r.FormValue("album"))

	if title == "" {
		title = pickFirstNonEmpty(probeMeta["title"], baseName)
	}
	if artist == "" {
		artist = strings.TrimSpace(probeMeta["artist"])
	}
	if album == "" {
		album = strings.TrimSpace(probeMeta["album"])
	}

	duration := 0.0
	if rawDur := strings.TrimSpace(probeMeta["duration"]); rawDur != "" {
		if f, err := strconv.ParseFloat(rawDur, 64); err == nil && f > 0 {
			duration = f
		}
	}

	if coverRelPath == "" {
		coverRelPath = filepath.ToSlash(filepath.Join(folder, sanitizePathPart(baseName)+"_cover.jpg"))
		coverAbsPath := filepath.Join(h.uploadDir, filepath.FromSlash(coverRelPath))
		if err := extractCoverWithFFmpeg(songAbsPath, coverAbsPath); err != nil {
			coverRelPath = ""
		}
	}

	fileInfo, err := os.Stat(songAbsPath)
	if err != nil {
		response.InternalServerError(w, "读取上传文件失败")
		return
	}

	if len(songRelPath) > 255 {
		response.BadRequest(w, "歌曲路径过长，请缩短文件名")
		return
	}
	if len(lrcRelPath) > 255 {
		response.BadRequest(w, "歌词路径过长，请缩短文件名")
		return
	}
	if len(coverRelPath) > 255 {
		response.BadRequest(w, "封面路径过长，请缩短文件名")
		return
	}

	if err := h.upsertAudioRecord(r.Context(), songRelPath, title, artist, album, duration, fileInfo.Size(), ext, lrcRelPath, coverRelPath); err != nil {
		logger.Error("后台上传歌曲入库失败: %v", err)
		response.InternalServerError(w, "写入数据库失败")
		return
	}

	response.Success(w, map[string]any{
		"path":           songRelPath,
		"title":          title,
		"artist":         artist,
		"album":          album,
		"duration_sec":   duration,
		"lrc_path":       lrcRelPath,
		"cover_art_path": coverRelPath,
	})
}

func (h *AdminHandler) requireAdmin(w http.ResponseWriter, r *http.Request) (string, bool) {
	token := h.readSessionToken(r)
	if token == "" {
		response.Unauthorized(w, "请先登录管理员账号")
		return "", false
	}
	rdb := cache.GetClient()
	if rdb == nil {
		response.InternalServerError(w, "Redis 未初始化")
		return "", false
	}
	val, err := rdb.Get(cache.GetContext(), h.adminSessionKey(token)).Result()
	if err != nil {
		response.Unauthorized(w, "会话已失效，请重新登录")
		return "", false
	}
	_ = rdb.Expire(cache.GetContext(), h.adminSessionKey(token), h.sessionTTL).Err()
	return val, true
}

func (h *AdminHandler) readSessionToken(r *http.Request) string {
	cookie, err := r.Cookie(adminCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func (h *AdminHandler) adminSessionKey(token string) string {
	return "admin:session:" + token
}

func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func parsePositiveIntWithDefault(raw string, fallback int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && v > 0 {
		return v
	}
	return fallback
}

func normalizePathList(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(p)))
		if clean == "" || clean == "." || strings.HasPrefix(clean, "../") {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out
}

func (h *AdminHandler) fetchMediaRecordsForDelete(ctx context.Context, tx *sql.Tx, paths []string) ([]mediaRecordForDelete, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(paths))
	placeholders = strings.TrimSuffix(placeholders, ",")
	query := fmt.Sprintf("SELECT path, is_audio, COALESCE(lrc_path,''), COALESCE(cover_art_path,'') FROM %s WHERE path IN (%s)", h.catalogMusicTable(), placeholders)
	args := make([]any, 0, len(paths))
	for _, p := range paths {
		args = append(args, p)
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]mediaRecordForDelete, 0, len(paths))
	for rows.Next() {
		var rec mediaRecordForDelete
		var tiny int
		if err := rows.Scan(&rec.Path, &tiny, &rec.LrcPath, &rec.CoverPath); err != nil {
			return nil, err
		}
		rec.IsAudio = tiny == 1
		out = append(out, rec)
	}
	return out, rows.Err()
}

func removeRelativeFile(root, rel string) error {
	rel = filepath.Clean(strings.TrimSpace(rel))
	if rel == "" || rel == "." {
		return nil
	}
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("跳过非法路径: %s", rel)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absTarget, err := filepath.Abs(filepath.Join(root, rel))
	if err != nil {
		return err
	}
	prefix := absRoot + string(os.PathSeparator)
	if absTarget != absRoot && !strings.HasPrefix(absTarget, prefix) {
		return fmt.Errorf("跳过越界路径: %s", rel)
	}
	if err := os.Remove(absTarget); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("删除文件失败 %s: %w", rel, err)
	}
	return nil
}

func openFirstFormFile(r *http.Request, keys ...string) (multipart.File, *multipart.FileHeader, error) {
	for _, k := range keys {
		file, header, err := r.FormFile(k)
		if err == nil {
			return file, header, nil
		}
		if !errors.Is(err, http.ErrMissingFile) {
			return nil, nil, err
		}
	}
	return nil, nil, http.ErrMissingFile
}

func saveMultipartFile(src multipart.File, destAbs string) error {
	if err := os.MkdirAll(filepath.Dir(destAbs), os.ModePerm); err != nil {
		return err
	}
	dst, err := os.Create(destAbs)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func sanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = rePathUnsafe.ReplaceAllString(name, "_")
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	return name
}

func sanitizePathPart(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = rePathUnsafe.ReplaceAllString(s, "_")
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.Trim(s, "._-")
	if s == "" {
		return "item"
	}
	if len(s) > 64 {
		return s[:64]
	}
	return s
}

func probeAudio(path string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", path)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var result ffprobeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}
	meta := map[string]string{}
	if result.Format.Duration != "" {
		meta["duration"] = result.Format.Duration
	}
	for k, v := range result.Format.Tags {
		meta[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	return meta, nil
}

func extractCoverWithFFmpeg(inputAbs, outputAbs string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", inputAbs, "-an", "-vcodec", "mjpeg", "-frames:v", "1", outputAbs)
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(outputAbs)
		logger.Warn("ffmpeg 提取封面失败: %v, output=%s", err, string(output))
		return err
	}
	if fi, err := os.Stat(outputAbs); err != nil || fi.Size() == 0 {
		_ = os.Remove(outputAbs)
		return fmt.Errorf("封面文件为空")
	}
	return nil
}

func pickFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func normalizeSchema(schema, fallback string) string {
	s := strings.TrimSpace(schema)
	if s == "" {
		s = strings.TrimSpace(fallback)
	}
	if s == "" {
		s = "music_users"
	}
	if !validSchemaIdent.MatchString(s) {
		return fallback
	}
	return s
}

func quoteIdent(ident string) string {
	return "`" + strings.ReplaceAll(ident, "`", "``") + "`"
}

func (h *AdminHandler) catalogMusicTable() string {
	return quoteIdent(h.catalogSchema) + "." + quoteIdent("music_files")
}

func (h *AdminHandler) catalogUsersTable() string {
	return quoteIdent(h.catalogSchema) + "." + quoteIdent("users")
}

func (h *AdminHandler) catalogArtistsTable() string {
	return quoteIdent(h.catalogSchema) + "." + quoteIdent("artists")
}

func (h *AdminHandler) mediaLyricsTable() string {
	return quoteIdent(h.mediaSchema) + "." + quoteIdent("media_lyrics_map")
}

func (h *AdminHandler) upsertAudioRecord(ctx context.Context, relPath, title, artist, album string, duration float64, sizeBytes int64, fileType, lrcPath, coverPath string) error {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	upsertSQL := fmt.Sprintf(`
		INSERT INTO %s
			(path, title, artist, album, duration_sec, size_bytes, file_type, is_audio, lrc_path, cover_art_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
		ON DUPLICATE KEY UPDATE
			title = VALUES(title),
			artist = VALUES(artist),
			album = VALUES(album),
			duration_sec = VALUES(duration_sec),
			size_bytes = VALUES(size_bytes),
			file_type = VALUES(file_type),
			is_audio = 1,
			lrc_path = VALUES(lrc_path),
			cover_art_path = VALUES(cover_art_path),
			updated_at = CURRENT_TIMESTAMP
	`, h.catalogMusicTable())

	var lrcVal any
	if strings.TrimSpace(lrcPath) == "" {
		lrcVal = nil
	} else {
		lrcVal = lrcPath
	}
	var coverVal any
	if strings.TrimSpace(coverPath) == "" {
		coverVal = nil
	} else {
		coverVal = coverPath
	}

	if _, err := tx.ExecContext(ctx, upsertSQL, relPath, truncate(title, 255), truncate(artist, 255), truncate(album, 255), duration, sizeBytes, truncate(fileType, 32), lrcVal, coverVal); err != nil {
		return err
	}

	if strings.TrimSpace(artist) != "" {
		upsertArtistSQL := fmt.Sprintf(`
			INSERT INTO %s (name)
			VALUES (?)
			ON DUPLICATE KEY UPDATE updated_at = CURRENT_TIMESTAMP
		`, h.catalogArtistsTable())
		for _, name := range splitArtists(artist) {
			if _, err := tx.ExecContext(ctx, upsertArtistSQL, truncate(name, 255)); err != nil {
				return err
			}
		}
	}

	if strings.TrimSpace(lrcPath) != "" {
		upsertLyricsSQL := fmt.Sprintf(`
			INSERT INTO %s (music_path, lrc_path, source)
			VALUES (?, ?, 'catalog')
			ON DUPLICATE KEY UPDATE lrc_path = VALUES(lrc_path), source = 'catalog', updated_at = CURRENT_TIMESTAMP
		`, h.mediaLyricsTable())
		if _, err := tx.ExecContext(ctx, upsertLyricsSQL, relPath, lrcPath); err != nil && !strings.Contains(strings.ToLower(err.Error()), "doesn't exist") {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func splitArtists(artist string) []string {
	artist = strings.TrimSpace(artist)
	if artist == "" {
		return nil
	}
	replacer := strings.NewReplacer(
		" feat. ", ",",
		" feat ", ",",
		" ft. ", ",",
		" ft ", ",",
		"&", ",",
		"、", ",",
		"，", ",",
		"/", ",",
		"+", ",",
	)
	parts := strings.Split(replacer.Replace(artist), ",")
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}
