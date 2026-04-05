package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"music-platform/internal/music/model"
	"music-platform/internal/music/service"
	"music-platform/pkg/response"
)

// MusicHandler 音乐处理器
type MusicHandler struct {
	musicService service.MusicService
	baseURL      string
}

// NewMusicHandler 创建音乐处理器
func NewMusicHandler(musicService service.MusicService, baseURL string) *MusicHandler {
	return &MusicHandler{
		musicService: musicService,
		baseURL:      baseURL,
	}
}

// GetFiles 获取文件列表（旧版格式兼容）
// 返回 map[string]string 格式：{"path": "duration"}
func (h *MusicHandler) GetFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	fileList, err := h.musicService.GetAllMusic(r.Context(), h.baseURL)
	if err != nil {
		response.InternalServerError(w, "无法获取文件列表")
		return
	}

	// 返回数组格式（与旧版100%兼容）
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileList)
}

// GetFilesDetailed 获取文件列表（新版详细格式）
// 返回数组格式，包含artist和cover_art_url
func (h *MusicHandler) GetFilesDetailed(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	fileList, err := h.musicService.GetAllMusic(r.Context(), h.baseURL)
	if err != nil {
		response.InternalServerError(w, "无法获取文件列表")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileList)
}

// Stream 流媒体处理
func (h *MusicHandler) Stream(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req model.StreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "请求参数错误")
		return
	}

	filename := strings.TrimSpace(req.Filename)
	if filename == "" {
		response.BadRequest(w, "文件名不能为空")
		return
	}

	musicResp, err := h.musicService.GetMusicByFilename(r.Context(), filename, h.baseURL)
	if err != nil {
		if err == sql.ErrNoRows {
			response.NotFound(w, "音乐文件不存在")
		} else {
			response.InternalServerError(w, "查询失败")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(musicResp)
}

// GetMusic 获取音乐详情
func (h *MusicHandler) GetMusic(w http.ResponseWriter, r *http.Request) {
	musicPath := r.URL.Query().Get("path")
	if musicPath == "" {
		response.BadRequest(w, "path参数不能为空")
		return
	}

	musicResp, err := h.musicService.GetMusicByPath(r.Context(), musicPath, h.baseURL)
	if err != nil {
		if err == sql.ErrNoRows {
			response.NotFound(w, "音乐不存在")
		} else {
			response.InternalServerError(w, "查询失败")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(musicResp)
}

// GetMusicByArtist 根据歌手获取音乐列表
// 返回格式与 /files 接口一致：FileListItem 数组
func (h *MusicHandler) GetMusicByArtist(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req model.ArtistMusicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "请求参数错误")
		return
	}

	artist := strings.TrimSpace(req.Artist)
	if artist == "" {
		response.BadRequest(w, "歌手名称不能为空")
		return
	}

	musicList, err := h.musicService.GetMusicByArtist(r.Context(), artist, h.baseURL)
	if err != nil {
		response.InternalServerError(w, "查询失败: "+err.Error())
		return
	}

	// 直接返回数组格式，与 /files 接口一致
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(musicList)
}

// SearchMusic 根据关键词搜索音乐
// 返回格式与 /files 接口一致：FileListItem 数组，按相关性排序
func (h *MusicHandler) SearchMusic(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req model.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "请求参数错误")
		return
	}

	keyword := strings.TrimSpace(req.Keyword)
	if keyword == "" {
		response.BadRequest(w, "搜索关键词不能为空")
		return
	}

	musicList, err := h.musicService.SearchMusic(r.Context(), keyword, h.baseURL)
	if err != nil {
		response.InternalServerError(w, "搜索失败: "+err.Error())
		return
	}

	// 直接返回数组格式，与 /files 接口一致，已按相关性排序
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(musicList)
}

// HealthTest 提供一个独立的轻量测试接口，便于联调和 PR 验证
func (h *MusicHandler) HealthTest(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持GET请求")
		return
	}

	response.Success(w, map[string]interface{}{
		"service": "music",
		"status":  "ok",
		"route":   "/music/health-test",
	})
}
