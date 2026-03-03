package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	adminHandler "music-platform/internal/admin/handler"
	artistHandler "music-platform/internal/artist/handler"
	artistRepo "music-platform/internal/artist/repository"
	artistService "music-platform/internal/artist/service"
	clientHandler "music-platform/internal/client/handler"
	"music-platform/internal/common/cache"
	"music-platform/internal/common/config"
	"music-platform/internal/common/database"
	"music-platform/internal/common/logger"
	"music-platform/internal/common/middleware"
	"music-platform/internal/media/handler"
	musicHandler "music-platform/internal/music/handler"
	musicRepo "music-platform/internal/music/repository"
	musicService "music-platform/internal/music/service"
	userHandler "music-platform/internal/user/handler"
	userRepo "music-platform/internal/user/repository"
	userService "music-platform/internal/user/service"
	usermusicHandler "music-platform/internal/usermusic/handler"
	usermusicRepo "music-platform/internal/usermusic/repository"
	usermusicService "music-platform/internal/usermusic/service"
	videoHandler "music-platform/internal/video/handler"
	videoService "music-platform/internal/video/service"
	"music-platform/pkg/response"
)

func main() {
	// 1. 初始化日志
	if err := logger.Init("server.log"); err != nil {
		fmt.Printf("日志初始化失败: %v\n", err)
		os.Exit(1)
	}
	logger.Info("服务器启动中...")

	// 2. 加载配置
	cfg := config.MustLoad("configs/config.yaml")
	logger.Info("配置加载成功")

	// 3. 初始化数据库
	if err := database.Init(&cfg.Database); err != nil {
		logger.Fatal("数据库初始化失败: %v", err)
	}
	defer database.Close()
	logger.Info("数据库连接成功")

	// 4. 初始化Redis
	if err := cache.Init(&cfg.Redis); err != nil {
		logger.Fatal("Redis初始化失败: %v", err)
	}
	defer cache.Close()
	logger.Info("Redis连接成功")

	// 5. 确保上传目录存在
	os.MkdirAll(cfg.Server.UploadDir, os.ModePerm)
	os.MkdirAll(cfg.Server.VideoDir, os.ModePerm)

	// 6. 初始化仓储层
	db := database.GetDB()
	userRepository := userRepo.NewUserRepository(db)
	userMusicRepository := userRepo.NewUserMusicRepository(db)
	musicRepository := musicRepo.NewMusicRepository(db)
	artistRepository := artistRepo.NewArtistRepository(db)
	profileSchema := strings.TrimSpace(cfg.Schemas.Profile)
	if profileSchema == "" {
		profileSchema = cfg.Database.Name
	}
	catalogSchema := strings.TrimSpace(cfg.Schemas.Catalog)
	if catalogSchema == "" {
		catalogSchema = cfg.Database.Name
	}
	usermusicRepository := usermusicRepo.NewUserMusicRepository(db, profileSchema, catalogSchema)
	if err := usermusicRepository.EnsureTables(); err != nil {
		logger.Warn("初始化用户行为表失败，将继续启动: %v", err)
	}

	// 7. 初始化服务层
	userSvc := userService.NewUserService(userRepository, userMusicRepository)
	musicSvc := musicService.NewMusicService(musicRepository)
	videoSvc := videoService.NewVideoService(cfg.Server.VideoDir)
	artistSvc := artistService.NewArtistService(artistRepository)

	// 8. 初始化处理器层
	// 根据 TLS 配置决定使用 http 还是 https
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
			baseURL = "http://localhost:8080" // 备用默认地址
		} else {
			baseURL = fmt.Sprintf("%s://%s:%d", protocol, cfg.Server.PublicHost, publicPort)
		}
	}

	mediaSchema := strings.TrimSpace(cfg.Schemas.Media)
	if mediaSchema == "" {
		mediaSchema = "music_media"
	}

	usermusicSvc := usermusicService.NewUserMusicService(usermusicRepository, baseURL, nil, nil)
	userH := userHandler.NewUserHandler(userSvc)
	musicH := musicHandler.NewMusicHandler(musicSvc, baseURL)
	mediaH := handler.NewMediaHandler(cfg.Server.UploadDir, db, mediaSchema, catalogSchema)
	if err := mediaH.EnsureTables(); err != nil {
		logger.Warn("初始化 media 表失败，将继续启动: %v", err)
	} else if err := mediaH.SyncLyricsMap(); err != nil {
		logger.Warn("初始化 media 歌词映射失败，将继续启动: %v", err)
	}
	videoH := videoHandler.NewVideoHandler(videoSvc, baseURL)
	artistH := artistHandler.NewArtistHandler(artistSvc)
	usermusicH := usermusicHandler.NewUserMusicHandler(usermusicSvc)
	adminH := adminHandler.NewAdminHandler(cfg, db)
	connectH := clientHandler.NewConnectHandler(cfg)

	// 9. 创建路由
	mux := http.NewServeMux()

	// 用户相关路由
	mux.HandleFunc("/client/ping", connectH.Ping)
	mux.HandleFunc("/client/bootstrap", connectH.Bootstrap)
	mux.HandleFunc("/users/register", userH.Register)
	mux.HandleFunc("/users/login", userH.Login)
	mux.HandleFunc("/users/ping", userH.Ping)
	mux.HandleFunc("/users/add_music", userH.AddMusic)

	// 音乐相关路由
	mux.HandleFunc("/files", musicH.GetFiles)
	mux.HandleFunc("/file", musicH.GetFiles) // 旧版兼容接口（与 /files 功能相同）
	mux.HandleFunc("/stream", musicH.Stream)
	mux.HandleFunc("/get_music", musicH.GetMusic)
	mux.HandleFunc("/music/artist", musicH.GetMusicByArtist) // 根据歌手查询音乐
	mux.HandleFunc("/music/search", musicH.SearchMusic)      // 关键词搜索音乐

	// 歌手相关路由
	mux.HandleFunc("/artist/search", artistH.SearchArtist) // 搜索歌手是否存在

	// 用户音乐相关路由（喜欢列表和播放历史）
	mux.HandleFunc("/user/favorites/add", usermusicH.AddFavorite)        // 添加喜欢
	mux.HandleFunc("/user/favorites/remove", usermusicH.RemoveFavorite)  // 移除喜欢
	mux.HandleFunc("/user/favorites", usermusicH.ListFavorites)          // 喜欢列表
	mux.HandleFunc("/user/history/add", usermusicH.AddPlayHistory)       // 添加播放历史
	mux.HandleFunc("/user/history/delete", usermusicH.DeletePlayHistory) // 删除播放历史（批量）
	mux.HandleFunc("/user/history/clear", usermusicH.ClearPlayHistory)   // 清空播放历史
	mux.HandleFunc("/user/history", usermusicH.ListPlayHistory)          // 播放历史

	// 视频相关路由
	mux.HandleFunc("/videos", videoH.GetVideoList)
	mux.HandleFunc("/video/stream", videoH.GetVideoStream)

	// 媒体文件服务路由（注意：带斜杠的路径要在精确路径之后注册）
	mux.HandleFunc("/upload", mediaH.Upload)
	mux.HandleFunc("/lrc", mediaH.LRC)
	mux.HandleFunc("/uploads/", mediaH.ServeFile)
	mux.HandleFunc("/files/", mediaH.Download)
	mux.HandleFunc("/download", mediaH.DownloadQuery) // 旧版兼容接口（处理 ?path= 参数）

	// 管理后台（仅管理员）
	mux.HandleFunc("/admin", adminH.AdminPage)
	mux.HandleFunc("/admin/", adminH.AdminPage)
	mux.HandleFunc("/admin/api/login", adminH.Login)
	mux.HandleFunc("/admin/api/logout", adminH.Logout)
	mux.HandleFunc("/admin/api/session", adminH.Session)
	mux.HandleFunc("/admin/api/users", adminH.ListUsers)
	mux.HandleFunc("/admin/api/media", adminH.ListMedia)
	mux.HandleFunc("/admin/api/media/delete", adminH.BatchDeleteMedia)
	mux.HandleFunc("/admin/api/upload/song", adminH.UploadSong)

	// 视频文件服务路由
	mux.HandleFunc("/video/", func(w http.ResponseWriter, r *http.Request) {
		// 提取视频路径
		videoPath := strings.TrimPrefix(r.URL.Path, "/video/")
		if videoPath == "" {
			response.BadRequest(w, "视频路径不能为空")
			return
		}

		// 构建完整文件路径
		fullPath := filepath.Join(cfg.Server.VideoDir, videoPath)

		// 安全检查
		cleanPath := filepath.Clean(fullPath)
		if !strings.HasPrefix(cleanPath, cfg.Server.VideoDir) {
			response.BadRequest(w, "非法的视频路径")
			return
		}

		// 检查文件是否存在
		if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}

		// 设置响应头支持视频流
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Accept-Ranges", "bytes")

		// 服务文件
		http.ServeFile(w, r, cleanPath)
	})
	// 遗留接口（兼容旧版，返回空数据）
	mux.HandleFunc("/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})
	mux.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
		response.Success(w, map[string]interface{}{"success": true})
	})

	// 健康检查和工具接口（与旧版完全一致）
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// 检查Redis连接（与旧版一致）
		rdb := cache.GetClient()
		_, redisErr := rdb.Ping(r.Context()).Result()

		// 检查数据库连接（与旧版一致）
		db := database.GetDB()
		dbErr := db.Ping()

		status := map[string]interface{}{
			"database":  dbErr == nil,
			"redis":     redisErr == nil,
			"status":    "ok",
			"timestamp": time.Now().Unix(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	// 统计信息（旧版接口，返回基础信息）
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stats := map[string]interface{}{
			"status":    "ok",
			"version":   "2.0",
			"timestamp": time.Now().Unix(),
		}
		json.NewEncoder(w).Encode(stats)
	})

	// ACK端点（兼容）
	mux.HandleFunc("/ack", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 根路径（欢迎页，旧版兼容）
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Music Server v2.0</title>
</head>
<body>
    <h1>🎵 Music Platform - Microservice Architecture</h1>
    <p>Server Status: <strong style="color: green;">Running</strong></p>
    <p>Version: <strong>v2.0</strong></p>
    <p>Architecture: <strong>Domain-Driven Design</strong></p>
    <hr>
    <h2>API Endpoints:</h2>
    <ul>
        <li><a href="/health">/health</a> - Health Check</li>
        <li><a href="/files">/files</a> - Music List</li>
        <li><a href="/stats">/stats</a> - Server Stats</li>
    </ul>
</body>
</html>`))
	})

	// 静态文件服务
	if cfg.Server.StaticDir != "" {
		fs := http.FileServer(http.Dir(cfg.Server.StaticDir))
		mux.Handle("/static/", http.StripPrefix("/static/", fs))
	}

	// 10. 应用中间件
	handler := middleware.CORS(middleware.Logging(mux))

	// 11. 启动服务器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  0,                // 不限制读取超时（适合大文件）
		WriteTimeout: 0,                // 不限制写入超时（防止流媒体被截断）
		IdleTimeout:  10 * time.Minute, // 允许长连接
	}

	// 使用之前定义的 protocol 和 baseURL
	displayURL := baseURL

	logger.Info("========================================")
	logger.Info("音乐平台服务器启动成功")
	logger.Info("监听地址: %s://%s", protocol, addr)
	logger.Info("公共访问: %s", displayURL)
	logger.Info("========================================")
	logger.Info("可用路由:")
	logger.Info("  客户端启动:")
	logger.Info("    GET  /client/ping           - 服务器连通性检查")
	logger.Info("    GET  /client/bootstrap      - 客户端引导信息")
	logger.Info("  用户服务:")
	logger.Info("    POST /users/register        - 用户注册")
	logger.Info("    POST /users/login           - 用户登录")
	logger.Info("    POST /users/add_music       - 添加收藏")
	logger.Info("  音乐服务:")
	logger.Info("    GET  /files                 - 获取音乐列表")
	logger.Info("    POST /stream                - 获取流媒体URL")
	logger.Info("    GET  /get_music?path=xxx    - 获取音乐详情")
	logger.Info("    POST /music/artist          - 根据歌手查询音乐")
	logger.Info("    POST /music/search          - 关键词搜索音乐")
	logger.Info("  歌手服务:")
	logger.Info("    POST /artist/search         - 搜索歌手是否存在")
	logger.Info("  视频服务:")
	logger.Info("    GET  /videos                - 获取视频列表")
	logger.Info("    POST /video/stream          - 获取视频流URL")
	logger.Info("    GET  /video/<path>          - 视频文件服务")
	logger.Info("  媒体服务:")
	logger.Info("    GET  /uploads/<path>        - 流媒体传输")
	logger.Info("    GET  /uploads/<folder>/lrc  - 获取歌词")
	logger.Info("    POST /upload                - 上传文件")
	logger.Info("    GET  /files/<path>          - 下载文件")
	logger.Info("  其他:")
	logger.Info("    GET  /health                - 健康检查")
	logger.Info("    GET  /static/<file>         - 静态文件")
	logger.Info("========================================")

	// 根据配置启动 HTTP 或 HTTPS 服务器
	var err error
	if cfg.Server.EnableTLS {
		logger.Info("启动 HTTPS 服务器...")
		logger.Info("证书文件: %s", cfg.Server.CertFile)
		logger.Info("私钥文件: %s", cfg.Server.KeyFile)
		err = server.ListenAndServeTLS(cfg.Server.CertFile, cfg.Server.KeyFile)
	} else {
		logger.Info("启动 HTTP 服务器...")
		err = server.ListenAndServe()
	}

	if err != nil {
		logger.Fatal("服务器启动失败: %v", err)
	}
}
