package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	artistHandler "music-platform/internal/artist/handler"
	artistRepo "music-platform/internal/artist/repository"
	artistService "music-platform/internal/artist/service"
	"music-platform/internal/common/config"
	"music-platform/internal/common/database"
	"music-platform/internal/common/logger"
	"music-platform/internal/common/middleware"
	musicExternal "music-platform/internal/music/external"
	musicHandler "music-platform/internal/music/handler"
	musicRepo "music-platform/internal/music/repository"
	musicService "music-platform/internal/music/service"
)

func main() {
	if err := logger.Init("catalog_server.log"); err != nil {
		fmt.Printf("内容服务日志初始化失败: %v\n", err)
		os.Exit(1)
	}
	logger.Info("内容服务启动中...")

	cfg := config.MustLoad("configs/config.yaml")
	if err := database.Init(&cfg.Database); err != nil {
		logger.Fatal("内容服务数据库初始化失败: %v", err)
	}
	defer database.Close()

	mediaBaseURL := config.ResolveMediaPublicBaseURL(cfg.Server)

	db := database.GetDB()
	musicRepository := musicRepo.NewMusicRepository(db)
	artistRepository := artistRepo.NewArtistRepository(db)
	jamendoSvc := musicExternal.NewJamendoClient(config.ResolveJamendoConfig(cfg))
	musicSvc := musicService.NewMusicService(musicRepository, jamendoSvc)
	artistSvc := artistService.NewArtistService(artistRepository)
	musicH := musicHandler.NewMusicHandler(musicSvc, jamendoSvc, mediaBaseURL)
	jamendoH := musicHandler.NewJamendoHandler(jamendoSvc)
	artistH := artistHandler.NewArtistHandler(artistSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("/files", musicH.GetFiles)
	mux.HandleFunc("/file", musicH.GetFiles)
	mux.HandleFunc("/stream", musicH.Stream)
	mux.HandleFunc("/get_music", musicH.GetMusic)
	mux.HandleFunc("/music/artist", musicH.GetMusicByArtist)
	mux.HandleFunc("/music/search", musicH.SearchMusic)
	mux.HandleFunc("/music/health-test", musicH.HealthTest)
	mux.HandleFunc("/external/music/jamendo/search", jamendoH.Search)
	mux.HandleFunc("/external/music/jamendo/track", jamendoH.GetTrack)
	mux.HandleFunc("/artist/search", artistH.SearchArtist)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]interface{}{
			"service": "catalog-service",
			"status":  "ok",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	handler := middleware.CORS(middleware.Logging(mux))

	host := os.Getenv("CATALOG_SERVICE_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := 18082
	if raw := os.Getenv("CATALOG_SERVICE_PORT"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			port = v
		}
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	logger.Info("内容服务监听地址: %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Fatal("内容服务启动失败: %v", err)
	}
}
