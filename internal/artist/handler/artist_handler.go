package handler

import (
	"encoding/json"
	"net/http"

	"music-platform/internal/artist/model"
	"music-platform/internal/artist/service"
	"music-platform/internal/common/logger"
)

// ArtistHandler 歌手处理器
type ArtistHandler struct {
	service service.ArtistService
}

// NewArtistHandler 创建歌手处理器
func NewArtistHandler(service service.ArtistService) *ArtistHandler {
	return &ArtistHandler{
		service: service,
	}
}

// SearchArtist 搜索歌手接口
func (h *ArtistHandler) SearchArtist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req model.SearchArtistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("Failed to decode request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 验证参数
	if req.Artist == "" {
		http.Error(w, "Artist parameter is required", http.StatusBadRequest)
		return
	}

	// 调用服务层
	exists, err := h.service.SearchArtist(r.Context(), req.Artist)
	if err != nil {
		logger.Error("Failed to search artist: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 返回响应
	response := model.SearchArtistResponse{
		Exists: exists,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("Failed to encode response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Info("Artist search: %s, exists: %v", req.Artist, exists)
}
