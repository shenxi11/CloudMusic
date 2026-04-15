package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"music-platform/internal/music/external"
	"music-platform/internal/music/model"
	"music-platform/pkg/response"
)

type JamendoHandler struct {
	jamendoService external.JamendoService
}

func NewJamendoHandler(jamendoService external.JamendoService) *JamendoHandler {
	return &JamendoHandler{jamendoService: jamendoService}
}

func (h *JamendoHandler) Search(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req model.ExternalMusicSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "请求参数错误")
		return
	}

	keyword := strings.TrimSpace(req.Keyword)
	if keyword == "" {
		response.BadRequest(w, "搜索关键词不能为空")
		return
	}

	tracks, err := h.jamendoService.Search(r.Context(), keyword, req.Limit)
	if err != nil {
		h.writeJamendoError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tracks)
}

func (h *JamendoHandler) GetTrack(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持GET请求")
		return
	}

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		response.BadRequest(w, "id参数不能为空")
		return
	}

	track, err := h.jamendoService.GetTrack(r.Context(), id)
	if err != nil {
		h.writeJamendoError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(track)
}

func (h *JamendoHandler) writeJamendoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, external.ErrNotConfigured):
		response.Error(w, http.StatusServiceUnavailable, "Jamendo external music source is not configured")
	case errors.Is(err, external.ErrNotFound), errors.Is(err, sql.ErrNoRows):
		response.NotFound(w, "Jamendo track not found")
	case errors.Is(err, external.ErrUpstream):
		response.Error(w, http.StatusBadGateway, "Jamendo upstream request failed")
	default:
		response.InternalServerError(w, "Jamendo external music request failed")
	}
}
