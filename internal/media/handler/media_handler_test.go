package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"music-platform/internal/music/model"
)

type fakeJamendoService struct {
	configured bool
	track      *model.ExternalMusicTrack
	err        error
}

func (f *fakeJamendoService) IsConfigured() bool {
	return f.configured
}

func (f *fakeJamendoService) Search(ctx context.Context, keyword string, limit int) ([]*model.ExternalMusicTrack, error) {
	return nil, nil
}

func (f *fakeJamendoService) GetTrack(ctx context.Context, id string) (*model.ExternalMusicTrack, error) {
	return f.track, f.err
}

func TestDownloadQuerySupportsJamendoVirtualPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("jamendo-audio"))
	}))
	defer upstream.Close()

	h := NewMediaHandler("", nil, "music_media", "music_users", &fakeJamendoService{
		configured: true,
		track: &model.ExternalMusicTrack{
			SourceID:        "1218138",
			StreamURL:       upstream.URL + "/track.mp3",
			DownloadAllowed: true,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/download",
		strings.NewReader(`{"filename":"Mayday [jamendo-1218138].mp3"}`))
	rec := httptest.NewRecorder()

	h.DownloadQuery(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "jamendo-audio" {
		t.Fatalf("unexpected body: %q", body)
	}
}
