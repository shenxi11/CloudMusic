package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"music-platform/internal/common/cache"
	"music-platform/internal/common/config"
	"music-platform/internal/common/database"
	"music-platform/internal/common/logger"
	"music-platform/internal/common/middleware"
	userHandler "music-platform/internal/user/handler"
	userRepo "music-platform/internal/user/repository"
	userService "music-platform/internal/user/service"
)

func main() {
	if err := logger.Init("auth_server.log"); err != nil {
		fmt.Printf("认证服务日志初始化失败: %v\n", err)
		os.Exit(1)
	}
	logger.Info("认证服务启动中...")

	cfg := config.MustLoad("configs/config.yaml")

	if err := database.Init(&cfg.Database); err != nil {
		logger.Fatal("认证服务数据库初始化失败: %v", err)
	}
	defer database.Close()

	if err := cache.Init(&cfg.Redis); err != nil {
		logger.Fatal("认证服务Redis初始化失败: %v", err)
	}
	defer cache.Close()

	db := database.GetDB()
	userRepository := userRepo.NewUserRepository(db)
	userMusicRepository := userRepo.NewUserMusicRepository(db)
	userSvc := userService.NewUserService(userRepository, userMusicRepository)
	userH := userHandler.NewUserHandler(userSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("/users/register", userH.Register)
	mux.HandleFunc("/users/login", userH.Login)
	mux.HandleFunc("/users/ping", userH.Ping)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]interface{}{
			"service": "auth-service",
			"status":  "ok",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	handler := middleware.CORS(middleware.Logging(mux))

	host := os.Getenv("AUTH_SERVICE_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := 18081
	if raw := os.Getenv("AUTH_SERVICE_PORT"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			port = v
		}
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	logger.Info("认证服务监听地址: %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Fatal("认证服务启动失败: %v", err)
	}
}
