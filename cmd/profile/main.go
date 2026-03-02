package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"music-platform/internal/common/config"
	"music-platform/internal/common/database"
	"music-platform/internal/common/eventbus"
	"music-platform/internal/common/logger"
	"music-platform/internal/common/middleware"
	"music-platform/internal/common/outbox"
	usermusicHandler "music-platform/internal/usermusic/handler"
	usermusicRepo "music-platform/internal/usermusic/repository"
	usermusicService "music-platform/internal/usermusic/service"
)

func main() {
	if err := logger.Init("profile_server.log"); err != nil {
		fmt.Printf("用户行为服务日志初始化失败: %v\n", err)
		os.Exit(1)
	}
	logger.Info("用户行为服务启动中...")

	cfg := config.MustLoad("configs/config.yaml")
	if err := database.Init(&cfg.Database); err != nil {
		logger.Fatal("用户行为服务数据库初始化失败: %v", err)
	}
	defer database.Close()

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

	db := database.GetDB()
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
		logger.Fatal("初始化用户行为表失败: %v", err)
	}
	logger.Info("用户行为数据 schema: profile=%s, catalog=%s", profileSchema, catalogSchema)
	outboxStore := outbox.NewStore(db)
	if err := outboxStore.EnsureTable(); err != nil {
		logger.Warn("初始化 event_outbox 失败，将跳过 outbox: %v", err)
		outboxStore = nil
	}

	var publisher eventbus.Publisher
	redisPublisher, err := eventbus.NewRedisPublisher(&cfg.Redis, eventbus.DefaultChannel)
	if err != nil {
		logger.Warn("事件发布器初始化失败，将跳过事件发布: %v", err)
	} else {
		publisher = redisPublisher
		defer publisher.Close()
	}

	usermusicSvc := usermusicService.NewUserMusicService(usermusicRepository, baseURL, publisher, outboxStore)

	if publisher != nil && outboxStore != nil {
		pollInterval := time.Duration(defaultInt(cfg.Event.Outbox.PollIntervalMs, 2000)) * time.Millisecond
		batchSize := defaultInt(cfg.Event.Outbox.BatchSize, 50)
		maxRetry := defaultInt(cfg.Event.Outbox.MaxRetry, 10)
		baseDelay := time.Duration(defaultInt(cfg.Event.Outbox.RetryBaseDelayMs, 1000)) * time.Millisecond
		go runOutboxRelay(outboxStore, publisher, pollInterval, batchSize, maxRetry, baseDelay)
		logger.Info("outbox 补偿投递已启用: poll=%s batch=%d max_retry=%d", pollInterval, batchSize, maxRetry)
	}

	usermusicH := usermusicHandler.NewUserMusicHandler(usermusicSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("/user/favorites/add", usermusicH.AddFavorite)
	mux.HandleFunc("/user/favorites/remove", usermusicH.RemoveFavorite)
	mux.HandleFunc("/user/favorites", usermusicH.ListFavorites)
	mux.HandleFunc("/user/history/add", usermusicH.AddPlayHistory)
	mux.HandleFunc("/user/history/delete", usermusicH.DeletePlayHistory)
	mux.HandleFunc("/user/history/clear", usermusicH.ClearPlayHistory)
	mux.HandleFunc("/user/history", usermusicH.ListPlayHistory)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]interface{}{
			"service": "profile-service",
			"status":  "ok",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	handler := middleware.CORS(middleware.Logging(mux))

	host := os.Getenv("PROFILE_SERVICE_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := 18083
	if raw := os.Getenv("PROFILE_SERVICE_PORT"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			port = v
		}
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	logger.Info("用户行为服务监听地址: %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Fatal("用户行为服务启动失败: %v", err)
	}
}

func runOutboxRelay(
	store *outbox.Store,
	publisher eventbus.Publisher,
	pollInterval time.Duration,
	batchSize int,
	maxRetry int,
	baseDelay time.Duration,
) {
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	if maxRetry <= 0 {
		maxRetry = 10
	}
	if baseDelay <= 0 {
		baseDelay = time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		if err := flushOutbox(store, publisher, batchSize, maxRetry, baseDelay); err != nil {
			logger.Warn("outbox 补偿投递失败: %v", err)
		}
		<-ticker.C
	}
}

func flushOutbox(store *outbox.Store, publisher eventbus.Publisher, batchSize, maxRetry int, baseDelay time.Duration) error {
	records, err := store.FetchPending(batchSize)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	for _, rec := range records {
		if rec.Event == nil {
			_ = store.MarkDead(rec.ID, "empty_event")
			continue
		}

		if err := publisher.Publish(context.Background(), rec.Event); err != nil {
			nextCount := rec.RetryCount + 1
			if nextCount >= maxRetry {
				if markErr := store.MarkDead(rec.ID, err.Error()); markErr != nil {
					logger.Warn("outbox 标记死信失败 id=%d: %v", rec.ID, markErr)
				}
				logger.Error("outbox 事件进入死信 id=%s type=%s err=%v", rec.Event.ID, rec.Event.Type, err)
				continue
			}

			nextRetry := time.Now().Add(backoff(baseDelay, nextCount))
			if markErr := store.MarkRetry(rec.ID, nextRetry, err.Error()); markErr != nil {
				logger.Warn("outbox 标记重试失败 id=%d: %v", rec.ID, markErr)
			}
			continue
		}

		if err := store.MarkPublished(rec.ID); err != nil {
			logger.Warn("outbox 标记发布成功失败 id=%d: %v", rec.ID, err)
		}
	}
	return nil
}

func defaultInt(v, d int) int {
	if v <= 0 {
		return d
	}
	return v
}

func backoff(base time.Duration, attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	if attempt > 6 {
		attempt = 6
	}
	return base * time.Duration(1<<(attempt-1))
}
