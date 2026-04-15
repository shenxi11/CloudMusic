package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"music-platform/internal/music/external"
	"music-platform/internal/music/model"
)

type fakeJamendoService struct {
	searchTracks []*model.ExternalMusicTrack
	searchErr    error
	getTrack     *model.ExternalMusicTrack
	getErr       error
}

func (f *fakeJamendoService) IsConfigured() bool {
	return true
}

func (f *fakeJamendoService) Search(ctx context.Context, keyword string, limit int) ([]*model.ExternalMusicTrack, error) {
	return f.searchTracks, f.searchErr
}

func (f *fakeJamendoService) GetTrack(ctx context.Context, id string) (*model.ExternalMusicTrack, error) {
	return f.getTrack, f.getErr
}

func TestJamendoHandlerSearchValidationAndErrors(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		body       string
		serviceErr error
		wantStatus int
	}{
		{
			name:       "method not allowed",
			method:     http.MethodGet,
			body:       "",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "bad json",
			method:     http.MethodPost,
			body:       "{",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty keyword",
			method:     http.MethodPost,
			body:       `{"keyword":" "}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "not configured",
			method:     http.MethodPost,
			body:       `{"keyword":"rock"}`,
			serviceErr: external.ErrNotConfigured,
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "upstream",
			method:     http.MethodPost,
			body:       `{"keyword":"rock"}`,
			serviceErr: external.ErrUpstream,
			wantStatus: http.StatusBadGateway,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewJamendoHandler(&fakeJamendoService{searchErr: tc.serviceErr})
			req := httptest.NewRequest(tc.method, "/external/music/jamendo/search", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			h.Search(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d body=%s", tc.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestJamendoHandlerSearchSuccess(t *testing.T) {
	h := NewJamendoHandler(&fakeJamendoService{
		searchTracks: []*model.ExternalMusicTrack{{
			Source:    "jamendo",
			SourceID:  "1218138",
			Title:     "Mayday",
			Artist:    "Hasenchat",
			StreamURL: "https://audio.example/track.mp3",
		}},
	})
	req := httptest.NewRequest(http.MethodPost, "/external/music/jamendo/search", strings.NewReader(`{"keyword":"mayday","limit":2}`))
	rec := httptest.NewRecorder()

	h.Search(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"source_id":"1218138"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestJamendoHandlerGetTrackValidationAndErrors(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		target     string
		serviceErr error
		wantStatus int
	}{
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			target:     "/external/music/jamendo/track?id=1",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "missing id",
			method:     http.MethodGet,
			target:     "/external/music/jamendo/track",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "not found",
			method:     http.MethodGet,
			target:     "/external/music/jamendo/track?id=404",
			serviceErr: external.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "wrapped upstream",
			method:     http.MethodGet,
			target:     "/external/music/jamendo/track?id=1",
			serviceErr: errors.Join(external.ErrUpstream, errors.New("timeout")),
			wantStatus: http.StatusBadGateway,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewJamendoHandler(&fakeJamendoService{getErr: tc.serviceErr})
			req := httptest.NewRequest(tc.method, tc.target, nil)
			rec := httptest.NewRecorder()

			h.GetTrack(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d body=%s", tc.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestJamendoHandlerGetTrackSuccess(t *testing.T) {
	h := NewJamendoHandler(&fakeJamendoService{
		getTrack: &model.ExternalMusicTrack{
			Source:    "jamendo",
			SourceID:  "168",
			Title:     "Track 168",
			Artist:    "Artist",
			StreamURL: "https://audio.example/168.mp3",
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/external/music/jamendo/track?id=168", nil)
	rec := httptest.NewRecorder()

	h.GetTrack(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"source":"jamendo"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}
