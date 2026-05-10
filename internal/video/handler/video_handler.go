package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"music-platform/internal/video/model"
	"music-platform/internal/video/service"
	"music-platform/pkg/response"
)

// VideoHandler 视频处理器
type VideoHandler struct {
	videoService service.VideoService
	baseURL      string // 完整的基础 URL（包含协议）
}

// NewVideoHandler 创建视频处理器
func NewVideoHandler(videoService service.VideoService, baseURL string) *VideoHandler {
	return &VideoHandler{
		videoService: videoService,
		baseURL:      baseURL,
	}
}

// GetVideoList 获取视频列表
func (h *VideoHandler) GetVideoList(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Printf("[INFO] 获取视频列表请求")
	videos, err := h.videoService.GetVideoList(r.Context())
	if err != nil {
		log.Printf("[ERROR] 获取视频列表失败: %v", err)
		response.InternalServerError(w, err.Error())
		return
	}

	log.Printf("[INFO] 找到 %d 个视频文件", len(videos))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(videos)
}

// GetVideoStream 获取视频流URL
func (h *VideoHandler) GetVideoStream(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req model.VideoStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ERROR] 视频流请求解析失败: %v", err)
		response.BadRequest(w, "请求参数错误")
		return
	}

	log.Printf("[INFO] 视频流请求: path=%s", req.Path)
	streamURL, err := h.videoService.GetVideoStreamURL(r.Context(), req.Path, h.baseURL)
	if err != nil {
		log.Printf("[ERROR] 获取视频流失败: %v", err)
		response.InternalServerError(w, err.Error())
		return
	}

	log.Printf("[INFO] 视频流URL生成成功: %s", streamURL)
	resp := &model.VideoStreamResponse{
		URL: streamURL,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetVideoPlaybackInfo 返回直链、HLS 与多清晰度播放信息。
func (h *VideoHandler) GetVideoPlaybackInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	videoPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if r.Method == http.MethodPost {
		var req model.VideoPlaybackInfoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[ERROR] 视频播放信息请求解析失败: %v", err)
			response.BadRequest(w, "请求参数错误")
			return
		}
		videoPath = strings.TrimSpace(req.Path)
	} else if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持GET或POST请求")
		return
	}

	log.Printf("[INFO] 视频播放信息请求: path=%s", videoPath)
	info, err := h.videoService.GetVideoPlaybackInfo(r.Context(), videoPath, h.baseURL)
	if err != nil {
		log.Printf("[ERROR] 获取视频播放信息失败: %v", err)
		response.BadRequest(w, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
