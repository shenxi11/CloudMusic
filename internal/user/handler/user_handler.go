package handler

import (
	"encoding/json"
	"io"
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

func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 GET")
		return
	}

	req := model.UserProfileRequest{
		Account:      r.URL.Query().Get("account"),
		SessionToken: r.URL.Query().Get("session_token"),
	}
	profile, err := h.userService.GetProfile(r.Context(), &req)
	if err != nil {
		writeProfileError(w, err)
		return
	}
	response.Success(w, profile)
}

func (h *UserHandler) UpdateUsername(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}

	var req model.UpdateUsernameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "请求参数错误")
		return
	}

	ret, err := h.userService.UpdateUsername(r.Context(), &req)
	if err != nil {
		writeProfileError(w, err)
		return
	}
	response.Success(w, ret)
}

func (h *UserHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}

	if err := r.ParseMultipartForm(6 << 20); err != nil {
		response.BadRequest(w, "表单解析失败")
		return
	}

	file, fileHeader, err := r.FormFile("avatar")
	if err != nil {
		response.BadRequest(w, "缺少头像文件")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 5<<20+1))
	if err != nil {
		response.InternalServerError(w, "读取头像文件失败")
		return
	}

	req := model.UploadAvatarRequest{
		Account:      r.FormValue("account"),
		SessionToken: r.FormValue("session_token"),
		Filename:     fileHeader.Filename,
		ContentType:  fileHeader.Header.Get("Content-Type"),
		Data:         data,
	}

	ret, err := h.userService.UploadAvatar(r.Context(), &req)
	if err != nil {
		writeProfileError(w, err)
		return
	}
	response.Success(w, ret)
}

// Ping 用户在线心跳
func (h *UserHandler) Ping(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 GET/POST")
		return
	}

	var req model.UserPingRequest
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.BadRequest(w, "请求参数错误")
			return
		}
	} else {
		req.Account = r.URL.Query().Get("account")
		req.Username = r.URL.Query().Get("username")
	}

	if err := h.userService.TouchOnline(r.Context(), &req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.Success(w, map[string]bool{"success": true})
}

// OnlineSessionStart 创建在线会话（新客户端推荐）
func (h *UserHandler) OnlineSessionStart(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}

	var req model.OnlineSessionStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "请求参数错误")
		return
	}

	session, err := h.userService.StartOnlineSession(r.Context(), &req)
	if err != nil {
		writeOnlineError(w, err)
		return
	}
	response.Success(w, session)
}

// OnlineHeartbeat 在线心跳（需要 session_token）
func (h *UserHandler) OnlineHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}

	var req model.OnlineHeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "请求参数错误")
		return
	}

	session, err := h.userService.HeartbeatOnline(r.Context(), &req)
	if err != nil {
		writeOnlineError(w, err)
		return
	}
	response.Success(w, session)
}

// OnlineStatus 查询在线状态（需要 session_token）
func (h *UserHandler) OnlineStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 GET")
		return
	}

	req := model.OnlineStatusRequest{
		Account:      r.URL.Query().Get("account"),
		Username:     r.URL.Query().Get("username"),
		SessionToken: r.URL.Query().Get("session_token"),
	}
	status, err := h.userService.GetOnlineStatus(r.Context(), &req)
	if err != nil {
		writeOnlineError(w, err)
		return
	}
	response.Success(w, status)
}

// OnlineLogout 主动下线（需要 session_token）
func (h *UserHandler) OnlineLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}

	var req model.OnlineLogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "请求参数错误")
		return
	}

	if err := h.userService.LogoutOnline(r.Context(), &req); err != nil {
		writeOnlineError(w, err)
		return
	}
	response.Success(w, map[string]bool{"success": true})
}

func writeOnlineError(w http.ResponseWriter, err error) {
	msg := err.Error()
	if strings.Contains(msg, "会话") || strings.Contains(msg, "token") {
		response.Unauthorized(w, msg)
		return
	}
	if strings.Contains(msg, "用户不存在") {
		response.NotFound(w, msg)
		return
	}
	response.BadRequest(w, msg)
}

func writeProfileError(w http.ResponseWriter, err error) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "会话") || strings.Contains(msg, "session_token") || strings.Contains(msg, "token"):
		response.Unauthorized(w, msg)
	case strings.Contains(msg, "用户不存在"):
		response.NotFound(w, msg)
	case strings.Contains(msg, "已被使用"):
		response.Error(w, http.StatusConflict, msg)
	case strings.Contains(msg, "查询用户失败") ||
		strings.Contains(msg, "更新用户名失败") ||
		strings.Contains(msg, "更新头像信息失败") ||
		strings.Contains(msg, "保存头像失败") ||
		strings.Contains(msg, "创建头像目录失败"):
		response.InternalServerError(w, msg)
	default:
		response.BadRequest(w, msg)
	}
}
