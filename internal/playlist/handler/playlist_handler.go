package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"music-platform/internal/playlist/model"
	"music-platform/internal/playlist/service"
)

type PlaylistHandler struct {
	service *service.PlaylistService
}

func NewPlaylistHandler(service *service.PlaylistService) *PlaylistHandler {
	return &PlaylistHandler{service: service}
}

func (h *PlaylistHandler) HandleRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.ListPlaylists(w, r)
	case http.MethodPost:
		h.CreatePlaylist(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *PlaylistHandler) HandleSubRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/user/playlists/"), "/")
	if path == "" {
		h.HandleRoot(w, r)
		return
	}

	parts := strings.Split(path, "/")
	playlistID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || playlistID <= 0 {
		http.Error(w, "非法的歌单ID", http.StatusBadRequest)
		return
	}

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		h.GetPlaylistDetail(w, r, playlistID)
	case len(parts) == 2 && parts[1] == "update" && r.Method == http.MethodPost:
		h.UpdatePlaylist(w, r, playlistID)
	case len(parts) == 2 && parts[1] == "delete" && r.Method == http.MethodPost:
		h.DeletePlaylist(w, r, playlistID)
	case len(parts) == 3 && parts[1] == "items" && parts[2] == "add" && r.Method == http.MethodPost:
		h.AddPlaylistItems(w, r, playlistID)
	case len(parts) == 3 && parts[1] == "items" && parts[2] == "remove" && r.Method == http.MethodPost:
		h.RemovePlaylistItems(w, r, playlistID)
	case len(parts) == 3 && parts[1] == "items" && parts[2] == "reorder" && r.Method == http.MethodPost:
		h.ReorderPlaylistItems(w, r, playlistID)
	default:
		http.NotFound(w, r)
	}
}

func (h *PlaylistHandler) ListPlaylists(w http.ResponseWriter, r *http.Request) {
	userAccount := getUserAccount(r)
	if userAccount == "" {
		http.Error(w, "缺少用户账号", http.StatusBadRequest)
		return
	}

	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntDefault(r.URL.Query().Get("page_size"), 20)

	resp, err := h.service.ListPlaylists(userAccount, page, pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, resp)
}

func (h *PlaylistHandler) CreatePlaylist(w http.ResponseWriter, r *http.Request) {
	userAccount := getUserAccount(r)
	if userAccount == "" {
		http.Error(w, "缺少用户账号", http.StatusBadRequest)
		return
	}

	var req model.CreatePlaylistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误: "+err.Error(), http.StatusBadRequest)
		return
	}

	playlistID, err := h.service.CreatePlaylist(userAccount, req)
	if err != nil {
		http.Error(w, err.Error(), statusCodeForError(err))
		return
	}

	writeJSON(w, map[string]interface{}{
		"success":     true,
		"message":     "创建成功",
		"playlist_id": playlistID,
	})
}

func (h *PlaylistHandler) GetPlaylistDetail(w http.ResponseWriter, r *http.Request, playlistID int64) {
	userAccount := getUserAccount(r)
	if userAccount == "" {
		http.Error(w, "缺少用户账号", http.StatusBadRequest)
		return
	}

	detail, err := h.service.GetPlaylistDetail(userAccount, playlistID)
	if err != nil {
		http.Error(w, err.Error(), statusCodeForError(err))
		return
	}
	writeJSON(w, detail)
}

func (h *PlaylistHandler) UpdatePlaylist(w http.ResponseWriter, r *http.Request, playlistID int64) {
	userAccount := getUserAccount(r)
	if userAccount == "" {
		http.Error(w, "缺少用户账号", http.StatusBadRequest)
		return
	}

	var req model.UpdatePlaylistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.service.UpdatePlaylist(userAccount, playlistID, req); err != nil {
		http.Error(w, err.Error(), statusCodeForError(err))
		return
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "更新成功",
	})
}

func (h *PlaylistHandler) DeletePlaylist(w http.ResponseWriter, r *http.Request, playlistID int64) {
	userAccount := getUserAccount(r)
	if userAccount == "" {
		http.Error(w, "缺少用户账号", http.StatusBadRequest)
		return
	}

	if err := h.service.DeletePlaylist(userAccount, playlistID); err != nil {
		http.Error(w, err.Error(), statusCodeForError(err))
		return
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "删除成功",
	})
}

func (h *PlaylistHandler) AddPlaylistItems(w http.ResponseWriter, r *http.Request, playlistID int64) {
	userAccount := getUserAccount(r)
	if userAccount == "" {
		http.Error(w, "缺少用户账号", http.StatusBadRequest)
		return
	}

	var req model.AddPlaylistItemsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误: "+err.Error(), http.StatusBadRequest)
		return
	}

	added, skipped, err := h.service.AddPlaylistItems(userAccount, playlistID, req)
	if err != nil {
		http.Error(w, err.Error(), statusCodeForError(err))
		return
	}

	writeJSON(w, map[string]interface{}{
		"success":       true,
		"message":       "添加成功",
		"added_count":   added,
		"skipped_count": skipped,
	})
}

func (h *PlaylistHandler) RemovePlaylistItems(w http.ResponseWriter, r *http.Request, playlistID int64) {
	userAccount := getUserAccount(r)
	if userAccount == "" {
		http.Error(w, "缺少用户账号", http.StatusBadRequest)
		return
	}

	var req model.RemovePlaylistItemsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误: "+err.Error(), http.StatusBadRequest)
		return
	}

	deleted, err := h.service.RemovePlaylistItems(userAccount, playlistID, req)
	if err != nil {
		http.Error(w, err.Error(), statusCodeForError(err))
		return
	}

	writeJSON(w, map[string]interface{}{
		"success":       true,
		"message":       "删除成功",
		"deleted_count": deleted,
	})
}

func (h *PlaylistHandler) ReorderPlaylistItems(w http.ResponseWriter, r *http.Request, playlistID int64) {
	userAccount := getUserAccount(r)
	if userAccount == "" {
		http.Error(w, "缺少用户账号", http.StatusBadRequest)
		return
	}

	var req model.ReorderPlaylistItemsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.service.ReorderPlaylistItems(userAccount, playlistID, req); err != nil {
		http.Error(w, err.Error(), statusCodeForError(err))
		return
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "排序成功",
	})
}

func getUserAccount(r *http.Request) string {
	userAccount := strings.TrimSpace(r.Header.Get("X-User-Account"))
	if userAccount == "" {
		userAccount = strings.TrimSpace(r.URL.Query().Get("user_account"))
	}
	return userAccount
}

func parseIntDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func statusCodeForError(err error) int {
	switch {
	case errors.Is(err, model.ErrPlaylistNotFound):
		return http.StatusNotFound
	case errors.Is(err, model.ErrInvalidReorderInput):
		return http.StatusBadRequest
	case strings.Contains(err.Error(), "不能为空"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}
