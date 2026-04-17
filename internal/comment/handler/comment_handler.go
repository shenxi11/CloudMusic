package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	commentmodel "music-platform/internal/comment/model"
	commentservice "music-platform/internal/comment/service"
)

type CommentHandler struct {
	service commentservice.CommentService
}

func NewCommentHandler(service commentservice.CommentService) *CommentHandler {
	return &CommentHandler{service: service}
}

func (h *CommentHandler) HandleRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.ListThreadComments(w, r)
	case http.MethodPost:
		h.CreateComment(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *CommentHandler) HandleSubRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/music/comments/"), "/")
	if path == "" {
		h.HandleRoot(w, r)
		return
	}

	parts := strings.Split(path, "/")
	commentID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || commentID <= 0 {
		http.Error(w, "非法的评论ID", http.StatusBadRequest)
		return
	}

	switch {
	case len(parts) == 2 && parts[1] == "replies" && r.Method == http.MethodGet:
		h.ListReplies(w, r, commentID)
	case len(parts) == 2 && parts[1] == "replies" && r.Method == http.MethodPost:
		h.CreateReply(w, r, commentID)
	case len(parts) == 2 && parts[1] == "delete" && r.Method == http.MethodPost:
		h.DeleteComment(w, r, commentID)
	default:
		http.NotFound(w, r)
	}
}

func (h *CommentHandler) ListThreadComments(w http.ResponseWriter, r *http.Request) {
	musicPath := strings.TrimSpace(r.URL.Query().Get("music_path"))
	if musicPath == "" {
		http.Error(w, "缺少 music_path", http.StatusBadRequest)
		return
	}

	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntDefault(r.URL.Query().Get("page_size"), 20)

	resp, err := h.service.ListThreadComments(musicPath, page, pageSize)
	if err != nil {
		writeCommentError(w, err)
		return
	}
	writeJSON(w, resp)
}

func (h *CommentHandler) ListReplies(w http.ResponseWriter, r *http.Request, commentID int64) {
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntDefault(r.URL.Query().Get("page_size"), 50)

	resp, err := h.service.ListReplies(commentID, page, pageSize)
	if err != nil {
		writeCommentError(w, err)
		return
	}
	writeJSON(w, resp)
}

func (h *CommentHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	var req commentmodel.CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误: "+err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.service.CreateComment(r.Context(), req)
	if err != nil {
		writeCommentError(w, err)
		return
	}
	writeJSON(w, resp)
}

func (h *CommentHandler) CreateReply(w http.ResponseWriter, r *http.Request, rootCommentID int64) {
	var req commentmodel.CreateReplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误: "+err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.service.CreateReply(r.Context(), rootCommentID, req)
	if err != nil {
		writeCommentError(w, err)
		return
	}
	writeJSON(w, resp)
}

func (h *CommentHandler) DeleteComment(w http.ResponseWriter, r *http.Request, commentID int64) {
	var req commentmodel.DeleteCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误: "+err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.service.DeleteComment(r.Context(), commentID, req)
	if err != nil {
		writeCommentError(w, err)
		return
	}
	writeJSON(w, resp)
}

func parseIntDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func writeCommentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, commentmodel.ErrCommentNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, commentmodel.ErrRootCommentRequired),
		errors.Is(err, commentmodel.ErrReplyTargetInvalid),
		errors.Is(err, commentmodel.ErrOnlineMusicOnly),
		strings.Contains(err.Error(), "不能为空"):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, commentmodel.ErrInvalidSession):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case errors.Is(err, commentmodel.ErrDeleteForbidden):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, commentmodel.ErrAuthUnavailable):
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}
