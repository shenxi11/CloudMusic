package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"music-platform/internal/common/config"
	"music-platform/internal/common/database"
	"music-platform/internal/common/logger"
	"music-platform/internal/common/middleware"
	mediaHandler "music-platform/internal/media/handler"
	musicExternal "music-platform/internal/music/external"
)

func main() {
	if err := logger.Init("media_server.log"); err != nil {
		fmt.Printf("媒体服务日志初始化失败: %v\n", err)
		os.Exit(1)
	}
	logger.Info("媒体服务启动中...")

	cfg := config.MustLoad("configs/config.yaml")
	if err := database.Init(&cfg.Database); err != nil {
		logger.Fatal("媒体服务数据库初始化失败: %v", err)
	}
	defer database.Close()

	db := database.GetDB()
	mediaSchema := strings.TrimSpace(cfg.Schemas.Media)
	if mediaSchema == "" {
		mediaSchema = "music_media"
	}
	catalogSchema := strings.TrimSpace(cfg.Schemas.Catalog)
	if catalogSchema == "" {
		catalogSchema = cfg.Database.Name
	}

	jamendoSvc := musicExternal.NewJamendoClient(config.ResolveJamendoConfig(cfg))
	mediaH := mediaHandler.NewMediaHandler(cfg.Server.UploadDir, db, mediaSchema, catalogSchema, jamendoSvc)
	if err := mediaH.EnsureTables(); err != nil {
		logger.Fatal("媒体服务初始化数据表失败: %v", err)
	}
	if err := mediaH.SyncLyricsMap(); err != nil {
		logger.Warn("媒体服务初始化歌词映射失败: %v", err)
	}
	logger.Info("媒体服务数据 schema: media=%s, catalog=%s", mediaSchema, catalogSchema)

	mux := http.NewServeMux()
	mux.HandleFunc("/upload", mediaH.Upload)
	mux.HandleFunc("/lrc", mediaH.LRC)
	mux.HandleFunc("/download", mediaH.DownloadQuery)
	mux.HandleFunc("/files/", mediaH.Download)
	mux.HandleFunc("/uploads/", mediaH.ServeFile)
	mux.HandleFunc("/music/local/seek-index", mediaH.LocalSeekIndex)
	mux.HandleFunc("/music/local/playback-info", mediaH.LocalPlaybackInfo)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]interface{}{
			"service": "media-service",
			"status":  "ok",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	handler := middleware.CORS(middleware.Logging(mux))

	host := os.Getenv("MEDIA_SERVICE_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := 18084
	if raw := os.Getenv("MEDIA_SERVICE_PORT"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			port = v
		}
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	logger.Info("媒体服务监听地址: %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Fatal("媒体服务启动失败: %v", err)
	}
}
