package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"music-platform/internal/music/model"
	"music-platform/internal/music/service"
)

type fakeMusicService struct{}

func (f *fakeMusicService) GetAllMusic(ctx context.Context, baseURL string) ([]*model.FileListItem, error) {
	return nil, nil
}

func (f *fakeMusicService) GetMusicByPath(ctx context.Context, path string, baseURL string) (*model.MusicResponse, error) {
	return nil, nil
}

func (f *fakeMusicService) GetMusicByFilename(ctx context.Context, filename string, baseURL string) (*model.MusicResponse, error) {
	return nil, nil
}

func (f *fakeMusicService) GetMusicByFilenameWithOptions(ctx context.Context, filename string, baseURL string, options service.PlaybackOptions) (*model.MusicResponse, error) {
	return nil, nil
}

func (f *fakeMusicService) GetMusicByArtist(ctx context.Context, artist string, baseURL string) ([]*model.FileListItem, error) {
	return nil, nil
}

func (f *fakeMusicService) SearchMusic(ctx context.Context, keyword string, baseURL string) ([]*model.FileListItem, error) {
	return nil, nil
}

func TestMusicHandlerStreamSupportsJamendoVirtualPath(t *testing.T) {
	h := NewMusicHandler(&fakeMusicService{}, &fakeJamendoService{
		getTrack: &model.ExternalMusicTrack{
			Source:      "jamendo",
			SourceID:    "1218138",
			Title:       "Mayday",
			Artist:      "Hasenchat",
			Album:       "Mayday Album",
			DurationSec: 252,
			StreamURL:   "https://audio.example/1218138.mp3",
			CoverArtURL: "https://img.example/1218138.jpg",
		},
	}, "http://127.0.0.1:8080")

	req := httptest.NewRequest(http.MethodPost, "/stream",
		strings.NewReader(`{"filename":"Mayday [jamendo-1218138].mp3"}`))
	rec := httptest.NewRecorder()

	h.Stream(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"stream_url":"https://audio.example/1218138.mp3"`) {
		t.Fatalf("unexpected body: %s", body)
	}
	if !strings.Contains(body, `"title":"Mayday"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}
