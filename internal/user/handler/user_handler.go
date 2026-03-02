package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"music-platform/internal/user/model"
	"music-platform/internal/user/service"
	"music-platform/pkg/response"
)

// UserHandler 用户处理器
type UserHandler struct {
	userService service.UserService
}

// NewUserHandler 创建用户处理器
func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// Register 注册处理
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ERROR] 注册请求解析失败: %v", err)
		response.BadRequest(w, "请求参数错误")
		return
	}

	log.Printf("[INFO] 注册请求: account=%s, username=%s", req.Account, req.Username)
	err := h.userService.Register(r.Context(), &req)
	if err != nil {
		log.Printf("[ERROR] 注册失败: %v", err)
		if strings.Contains(err.Error(), "已被注册") {
			http.Error(w, err.Error(), http.StatusConflict)
		} else {
			response.InternalServerError(w, err.Error())
		}
		return
	}

	log.Printf("[INFO] 注册成功: account=%s", req.Account)
	response.Success(w, map[string]bool{"success": true})
}

// Login 登录处理
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ERROR] 登录请求解析失败: %v", err)
		response.BadRequest(w, "请求参数错误")
		return
	}

	log.Printf("[INFO] 登录请求: account=%s", req.Account)
	loginResp, err := h.userService.Login(r.Context(), &req)
	if err != nil {
		log.Printf("[ERROR] 登录失败: %v", err)
		response.InternalServerError(w, err.Error())
		return
	}

	log.Printf("[INFO] 登录成功: account=%s, username=%s", req.Account, loginResp.Username)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loginResp)
}

// AddMusic 添加音乐处理
func (h *UserHandler) AddMusic(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req model.AddMusicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "请求参数错误")
		return
	}

	err := h.userService.AddMusic(r.Context(), &req)
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			response.NotFound(w, err.Error())
		} else {
			response.InternalServerError(w, err.Error())
		}
		return
	}

	response.Success(w, map[string]bool{"success": true})
}
