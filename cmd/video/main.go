package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"music-platform/internal/common/config"
	"music-platform/internal/common/logger"
	"music-platform/internal/common/middleware"
	videoHandler "music-platform/internal/video/handler"
	videoService "music-platform/internal/video/service"
)

func main() {
	if err := logger.Init("video_server.log"); err != nil {
		fmt.Printf("视频服务日志初始化失败: %v\n", err)
		os.Exit(1)
	}
	logger.Info("视频服务启动中...")

	cfg := config.MustLoad("configs/config.yaml")

	protocol := "http"
	if cfg.Server.EnableTLS {
		protocol = "https"
	}
	publicPort := cfg.Server.PublicPort
	if publicPort == 0 {
		publicPort = cfg.Server.Port
	}
	baseURL := strings.TrimSuffix(cfg.Server.PublicBaseURL, "/")
	if baseURL == "" {
		if cfg.Server.PublicHost == "" {
			baseURL = "http://localhost:8080"
		} else {
			baseURL = fmt.Sprintf("%s://%s:%d", protocol, cfg.Server.PublicHost, publicPort)
		}
	}

	videoSvc := videoService.NewVideoServiceWithPlayback(cfg.Server.VideoDir, cfg.Server.VideoHLSDir, cfg.Server.FFmpegBinary, cfg.Server.FFprobeBinary)
	videoH := videoHandler.NewVideoHandler(videoSvc, baseURL)

	mux := http.NewServeMux()
	mux.HandleFunc("/videos", videoH.GetVideoList)
	mux.HandleFunc("/video/stream", videoH.GetVideoStream)
	mux.HandleFunc("/video/playback-info", videoH.GetVideoPlaybackInfo)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]interface{}{
			"service": "video-service",
			"status":  "ok",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	handler := middleware.CORS(middleware.Logging(mux))

	host := os.Getenv("VIDEO_SERVICE_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := 18085
	if raw := os.Getenv("VIDEO_SERVICE_PORT"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			port = v
		}
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	logger.Info("视频服务监听地址: %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Fatal("视频服务启动失败: %v", err)
	}
}
