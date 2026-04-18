package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	chartmodel "music-platform/internal/chart/model"
	chartservice "music-platform/internal/chart/service"
	"music-platform/pkg/response"
)

type hotChartService interface {
	GetHotChart(ctx context.Context, query chartmodel.HotChartQuery) (*chartmodel.HotChartResponse, error)
	RebuildHotChart(ctx context.Context, query chartmodel.HotChartRebuildQuery) (*chartmodel.HotChartRebuildResponse, error)
}

type ChartHandler struct {
	svc hotChartService
}

func NewChartHandler(svc hotChartService) *ChartHandler {
	return &ChartHandler{svc: svc}
}

// GetHotChart GET /music/charts/hot
func (h *ChartHandler) GetHotChart(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持GET请求")
		return
	}

	data, err := h.svc.GetHotChart(r.Context(), chartmodel.HotChartQuery{
		Window: strings.TrimSpace(r.URL.Query().Get("window")),
		Limit:  parseLimit(r.URL.Query().Get("limit")),
	})
	if err != nil {
		if errors.Is(err, chartservice.ErrLeaderboardUnavailable) {
			response.Error(w, http.StatusServiceUnavailable, "热歌榜暂不可用")
			return
		}
		response.InternalServerError(w, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, data)
}

// RebuildHotChart POST /admin/charts/hot/rebuild
func (h *ChartHandler) RebuildHotChart(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	data, err := h.svc.RebuildHotChart(r.Context(), chartmodel.HotChartRebuildQuery{
		Window: strings.TrimSpace(r.URL.Query().Get("window")),
	})
	if err != nil {
		switch {
		case errors.Is(err, chartservice.ErrInvalidRebuildWindow):
			response.BadRequest(w, "window 仅支持 all、7d、30d")
		case errors.Is(err, chartservice.ErrLeaderboardUnavailable):
			response.Error(w, http.StatusServiceUnavailable, "热歌榜 Redis 不可用")
		default:
			response.InternalServerError(w, err.Error())
		}
		return
	}

	response.Success(w, data)
}

func parseLimit(raw string) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return 20
	}
	return v
}
