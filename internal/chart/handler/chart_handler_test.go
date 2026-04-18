package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chartmodel "music-platform/internal/chart/model"
	chartservice "music-platform/internal/chart/service"
)

type fakeChartService struct {
	resp        *chartmodel.HotChartResponse
	rebuildResp *chartmodel.HotChartRebuildResponse
	err         error
}

func (f *fakeChartService) GetHotChart(ctx context.Context, query chartmodel.HotChartQuery) (*chartmodel.HotChartResponse, error) {
	return f.resp, f.err
}

func (f *fakeChartService) RebuildHotChart(ctx context.Context, query chartmodel.HotChartRebuildQuery) (*chartmodel.HotChartRebuildResponse, error) {
	return f.rebuildResp, f.err
}

func TestChartHandlerMethodValidation(t *testing.T) {
	h := &ChartHandler{}
	req := httptest.NewRequest(http.MethodPost, "/music/charts/hot", nil)
	rec := httptest.NewRecorder()

	h.GetHotChart(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChartHandlerSuccess(t *testing.T) {
	svc := &fakeChartService{
		resp: &chartmodel.HotChartResponse{
			ChartID: "hot_online",
			Title:   "在线热歌榜",
			Window:  "30d",
			Items: []chartmodel.HotChartItem{{
				Rank:      1,
				MusicPath: "jay/七里香.mp3",
				Path:      "jay/七里香.mp3",
				Title:     "七里香",
				Source:    "catalog",
				PlayCount: 18,
			}},
		},
	}
	h := NewChartHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/music/charts/hot?window=30d&limit=20", nil)
	rec := httptest.NewRecorder()
	h.GetHotChart(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"chart_id":"hot_online"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestChartHandlerServiceUnavailable(t *testing.T) {
	h := NewChartHandler(&fakeChartService{err: chartservice.ErrLeaderboardUnavailable})

	req := httptest.NewRequest(http.MethodGet, "/music/charts/hot", nil)
	rec := httptest.NewRecorder()
	h.GetHotChart(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRebuildHotChartSuccess(t *testing.T) {
	h := NewChartHandler(&fakeChartService{
		rebuildResp: &chartmodel.HotChartRebuildResponse{
			Window:         chartmodel.WindowAll,
			RebuiltBuckets: 1,
			RebuiltItems:   20,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/charts/hot/rebuild?window=all", nil)
	rec := httptest.NewRecorder()
	h.RebuildHotChart(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"window":"all"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestRebuildHotChartBadWindow(t *testing.T) {
	h := NewChartHandler(&fakeChartService{err: chartservice.ErrInvalidRebuildWindow})
	req := httptest.NewRequest(http.MethodPost, "/admin/charts/hot/rebuild?window=1d", nil)
	rec := httptest.NewRecorder()
	h.RebuildHotChart(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}
