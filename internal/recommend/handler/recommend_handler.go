package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"music-platform/internal/recommend/model"
	"music-platform/internal/recommend/service"
	"music-platform/pkg/response"
)

type RecommendHandler struct {
	svc *service.RecommendService
}

func NewRecommendHandler(svc *service.RecommendService) *RecommendHandler {
	return &RecommendHandler{svc: svc}
}

// GetRecommendations GET /recommendations/audio
func (h *RecommendHandler) GetRecommendations(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持GET请求")
		return
	}

	userID := userIDFromRequest(r)
	if userID == "" {
		response.BadRequest(w, "缺少user_id")
		return
	}

	limit := parseInt(r.URL.Query().Get("limit"), 20)
	query := model.RecommendQuery{
		UserID:        userID,
		Scene:         r.URL.Query().Get("scene"),
		Limit:         limit,
		ExcludePlayed: parseBoolDefaultTrue(r.URL.Query().Get("exclude_played")),
		Cursor:        strings.TrimSpace(r.URL.Query().Get("cursor")),
	}

	data, err := h.svc.GetRecommendations(r.Context(), query)
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.Success(w, data)
}

// GetSimilar GET /recommendations/similar/{song_id}
func (h *RecommendHandler) GetSimilar(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持GET请求")
		return
	}

	songID := strings.TrimPrefix(r.URL.Path, "/recommendations/similar/")
	if strings.TrimSpace(songID) == "" {
		songID = r.URL.Query().Get("song_id")
	}
	if strings.TrimSpace(songID) == "" {
		response.BadRequest(w, "缺少song_id")
		return
	}

	limit := parseInt(r.URL.Query().Get("limit"), 20)
	data, err := h.svc.GetSimilarBySong(r.Context(), songID, limit)
	if err != nil {
		response.NotFound(w, err.Error())
		return
	}
	response.Success(w, data)
}

// PostFeedback POST /recommendations/feedback
func (h *RecommendHandler) PostFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req model.FeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "请求参数错误")
		return
	}
	if strings.TrimSpace(req.UserID) == "" {
		req.UserID = userIDFromRequest(r)
	}

	if err := h.svc.SaveFeedback(r.Context(), req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.Success(w, map[string]any{"success": true})
}

// TriggerRetrain POST /admin/recommend/retrain
func (h *RecommendHandler) TriggerRetrain(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req model.TrainRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	triggerBy := strings.TrimSpace(r.Header.Get("X-Admin-User"))
	if triggerBy == "" {
		triggerBy = strings.TrimSpace(r.URL.Query().Get("admin"))
	}
	if triggerBy == "" {
		triggerBy = "admin"
	}

	data, err := h.svc.TriggerRetrain(r.Context(), req, triggerBy)
	if err != nil {
		response.InternalServerError(w, err.Error())
		return
	}
	response.JSON(w, http.StatusAccepted, response.Response{
		Code:    0,
		Message: "accepted",
		Data:    data,
	})
}

// ModelStatus GET /admin/recommend/model-status
func (h *RecommendHandler) ModelStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持GET请求")
		return
	}

	modelName := strings.TrimSpace(r.URL.Query().Get("model_name"))
	data, err := h.svc.GetModelStatus(r.Context(), modelName)
	if err != nil {
		response.InternalServerError(w, err.Error())
		return
	}
	response.Success(w, data)
}

func userIDFromRequest(r *http.Request) string {
	candidates := []string{
		r.URL.Query().Get("user_id"),
		r.URL.Query().Get("user_account"),
		r.Header.Get("X-User-Account"),
		r.Header.Get("X-User-ID"),
	}
	for _, c := range candidates {
		if v := strings.TrimSpace(c); v != "" {
			return v
		}
	}
	return ""
}

func parseInt(raw string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return def
	}
	return v
}

func parseBoolDefaultTrue(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "0", "false", "no":
		return false
	default:
		return true
	}
}
