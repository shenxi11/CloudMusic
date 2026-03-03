package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"music-platform/internal/common/cache"
	"music-platform/internal/common/config"
	"music-platform/internal/common/database"
	"music-platform/pkg/response"
)

const clientAPIVersion = "2026-03-03"

type ConnectHandler struct {
	cfg *config.Config
}

func NewConnectHandler(cfg *config.Config) *ConnectHandler {
	return &ConnectHandler{cfg: cfg}
}

func (h *ConnectHandler) Ping(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 GET")
		return
	}

	now := time.Now()
	response.Success(w, map[string]any{
		"service":     "cloudmusic-server",
		"status":      "ok",
		"api_version": clientAPIVersion,
		"timestamp":   now.Unix(),
		"server_time": now.Format(time.RFC3339),
	})
}

func (h *ConnectHandler) Bootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 GET")
		return
	}

	dbReady := false
	db := database.GetDB()
	if db != nil {
		dbCtx, cancel := context.WithTimeout(r.Context(), time.Second)
		dbReady = db.PingContext(dbCtx) == nil
		cancel()
	}

	redisReady := false
	rdb := cache.GetClient()
	if rdb != nil {
		redisCtx, cancel := context.WithTimeout(r.Context(), time.Second)
		redisReady = rdb.Ping(redisCtx).Err() == nil
		cancel()
	}

	baseURL := h.resolveBaseURL(r)
	now := time.Now()

	response.Success(w, map[string]any{
		"service":         "cloudmusic-server",
		"api_version":     clientAPIVersion,
		"ready":           dbReady && redisReady,
		"timestamp":       now.Unix(),
		"server_time":     now.Format(time.RFC3339),
		"public_base_url": baseURL,
		"checks": map[string]bool{
			"database": dbReady,
			"redis":    redisReady,
		},
		"endpoints": map[string]string{
			"ping":      baseURL + "/client/ping",
			"bootstrap": baseURL + "/client/bootstrap",
			"register":  baseURL + "/users/register",
			"login":     baseURL + "/users/login",
		},
	})
}

func (h *ConnectHandler) resolveBaseURL(r *http.Request) string {
	baseURL := strings.TrimSpace(h.cfg.Server.PublicBaseURL)
	if baseURL != "" {
		return strings.TrimSuffix(baseURL, "/")
	}

	proto := "http"
	if h.cfg.Server.EnableTLS {
		proto = "https"
	}
	if forwardedProto := firstHeaderValue(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		proto = forwardedProto
	}

	host := firstHeaderValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" {
		publicHost := strings.TrimSpace(h.cfg.Server.PublicHost)
		publicPort := h.cfg.Server.PublicPort
		if publicPort <= 0 {
			publicPort = h.cfg.Server.Port
		}
		if publicHost == "" {
			publicHost = "localhost"
		}
		host = publicHost
		if !strings.Contains(host, ":") && publicPort > 0 {
			host = host + ":" + strconv.Itoa(publicPort)
		}
	}

	return proto + "://" + host
}

func firstHeaderValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if idx := strings.Index(v, ","); idx >= 0 {
		v = v[:idx]
	}
	return strings.TrimSpace(v)
}
