package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"music-platform/internal/usermusic/model"
	"music-platform/internal/usermusic/service"
)

type UserMusicHandler struct {
	service *service.UserMusicService
}

func NewUserMusicHandler(service *service.UserMusicService) *UserMusicHandler {
	return &UserMusicHandler{service: service}
}

// AddFavorite 添加喜欢的音乐
func (h *UserMusicHandler) AddFavorite(w http.ResponseWriter, r *http.Request) {
	// 获取用户账号（从请求中获取，暂时从header或query获取）
	userAccount := r.Header.Get("X-User-Account")
	if userAccount == "" {
		userAccount = r.URL.Query().Get("user_account")
	}
	if userAccount == "" {
		http.Error(w, "缺少用户账号", http.StatusBadRequest)
		return
	}

	var req model.AddFavoriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.service.AddFavorite(userAccount, req); err != nil {
		log.Printf("添加喜欢音乐失败: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "添加成功",
	})
}

// RemoveFavorite 移除喜欢的音乐
func (h *UserMusicHandler) RemoveFavorite(w http.ResponseWriter, r *http.Request) {
	// 获取用户账号
	userAccount := r.Header.Get("X-User-Account")
	if userAccount == "" {
		userAccount = r.URL.Query().Get("user_account")
	}

	// 如果还没有从header和query获取到，尝试从body获取
	musicPath := r.URL.Query().Get("music_path")
	if userAccount == "" || musicPath == "" {
		var req struct {
			UserAccount string `json:"user_account"`
			MusicPath   string `json:"music_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if userAccount == "" {
				userAccount = req.UserAccount
			}
			if musicPath == "" {
				musicPath = req.MusicPath
			}
		}
	}

	if userAccount == "" {
		http.Error(w, "缺少用户账号", http.StatusBadRequest)
		return
	}

	if musicPath == "" {
		http.Error(w, "缺少音乐路径参数", http.StatusBadRequest)
		return
	}

	if err := h.service.RemoveFavorite(userAccount, musicPath); err != nil {
		log.Printf("移除喜欢音乐失败: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "移除成功",
	})
}

// ListFavorites 获取喜欢的音乐列表
func (h *UserMusicHandler) ListFavorites(w http.ResponseWriter, r *http.Request) {
	// 获取用户账号
	userAccount := r.Header.Get("X-User-Account")
	if userAccount == "" {
		userAccount = r.URL.Query().Get("user_account")
	}
	if userAccount == "" {
		http.Error(w, "缺少用户账号", http.StatusBadRequest)
		return
	}

	items, err := h.service.ListFavorites(userAccount)
	if err != nil {
		log.Printf("获取喜欢列表失败: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// AddPlayHistory 添加播放历史
func (h *UserMusicHandler) AddPlayHistory(w http.ResponseWriter, r *http.Request) {
	// 获取用户账号
	userAccount := r.Header.Get("X-User-Account")
	if userAccount == "" {
		userAccount = r.URL.Query().Get("user_account")
	}
	if userAccount == "" {
		log.Printf("[ERROR] AddPlayHistory: 缺少用户账号, Header: %v, Query: %v", r.Header.Get("X-User-Account"), r.URL.Query().Get("user_account"))
		http.Error(w, "缺少用户账号", http.StatusBadRequest)
		return
	}

	var req model.AddPlayHistoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ERROR] AddPlayHistory: 请求参数解析失败 - %v, user_account: %s", err, userAccount)
		http.Error(w, "请求参数错误: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("[DEBUG] AddPlayHistory: user_account=%s, music_path=%s, title=%s, is_local=%v",
		userAccount, req.MusicPath, req.MusicTitle, req.IsLocal)

	if err := h.service.AddPlayHistory(userAccount, req); err != nil {
		log.Printf("[ERROR] AddPlayHistory: 添加播放历史失败 - %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "记录成功",
	})
}

// ListPlayHistory 获取播放历史
func (h *UserMusicHandler) ListPlayHistory(w http.ResponseWriter, r *http.Request) {
	// 获取用户账号
	userAccount := r.Header.Get("X-User-Account")
	if userAccount == "" {
		userAccount = r.URL.Query().Get("user_account")
	}
	if userAccount == "" {
		http.Error(w, "缺少用户账号", http.StatusBadRequest)
		return
	}

	// 获取limit参数（默认50）
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// 检查是否需要去重（默认去重）
	distinct := true
	if distinctStr := r.URL.Query().Get("distinct"); distinctStr != "" {
		distinct = distinctStr != "false" && distinctStr != "0"
	}

	var items []model.MusicItem
	var err error

	if distinct {
		// 去重查询（每首歌只显示最近一次播放）
		items, err = h.service.ListPlayHistoryDistinct(userAccount, limit)
	} else {
		// 完整历史（允许重复）
		items, err = h.service.ListPlayHistory(userAccount, limit)
	}

	if err != nil {
		log.Printf("获取播放历史失败: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// DeletePlayHistory 删除指定的播放历史记录（支持批量删除）
func (h *UserMusicHandler) DeletePlayHistory(w http.ResponseWriter, r *http.Request) {
	// 获取用户账号
	userAccount := r.Header.Get("X-User-Account")
	if userAccount == "" {
		userAccount = r.URL.Query().Get("user_account")
	}
	if userAccount == "" {
		http.Error(w, "缺少用户账号", http.StatusBadRequest)
		return
	}

	var req struct {
		MusicPaths []string `json:"music_paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.MusicPaths) == 0 {
		http.Error(w, "音乐路径列表不能为空", http.StatusBadRequest)
		return
	}

	deletedCount, err := h.service.DeletePlayHistory(userAccount, req.MusicPaths)
	if err != nil {
		log.Printf("删除播放历史失败: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"message":       "删除成功",
		"deleted_count": deletedCount,
	})
}

// ClearPlayHistory 清空全部播放历史
func (h *UserMusicHandler) ClearPlayHistory(w http.ResponseWriter, r *http.Request) {
	// 获取用户账号
	userAccount := r.Header.Get("X-User-Account")
	if userAccount == "" {
		userAccount = r.URL.Query().Get("user_account")
	}
	if userAccount == "" {
		http.Error(w, "缺少用户账号", http.StatusBadRequest)
		return
	}

	deletedCount, err := h.service.ClearPlayHistory(userAccount)
	if err != nil {
		log.Printf("清空播放历史失败: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"message":       "清空成功",
		"deleted_count": deletedCount,
	})
}
