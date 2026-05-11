package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"music-platform/internal/music/model"
)

type fakeMusicRepository struct {
	searchResult   []*model.MusicFile
	searchErr      error
	pathResult     *model.MusicFile
	filenameResult *model.MusicFile
}

func (f *fakeMusicRepository) FindAll(ctx context.Context) ([]*model.MusicFile, error) {
	return nil, nil
}

func (f *fakeMusicRepository) FindByPath(ctx context.Context, path string) (*model.MusicFile, error) {
	return f.pathResult, nil
}

func (f *fakeMusicRepository) FindByPathLike(ctx context.Context, filename string) (*model.MusicFile, error) {
	return f.filenameResult, nil
}

func (f *fakeMusicRepository) FindByArtist(ctx context.Context, artist string) ([]*model.MusicFile, error) {
	return nil, nil
}

func (f *fakeMusicRepository) SearchByKeyword(ctx context.Context, keyword string) ([]*model.MusicFile, error) {
	return f.searchResult, f.searchErr
}

type fakeJamendoService struct {
	configured bool
	search     []*model.ExternalMusicTrack
	searchErr  error
}

func (f *fakeJamendoService) IsConfigured() bool {
	return f.configured
}

func (f *fakeJamendoService) Search(ctx context.Context, keyword string, limit int) ([]*model.ExternalMusicTrack, error) {
	return f.search, f.searchErr
}

func (f *fakeJamendoService) GetTrack(ctx context.Context, id string) (*model.ExternalMusicTrack, error) {
	return nil, nil
}

func TestGetMusicByFilenamePrefersTranscodedAudioForLargeLossless(t *testing.T) {
	cacheDir := t.TempDir()
	outputPath := filepath.Join(cacheDir, "Jay", "Track.mp3")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}
	if err := os.WriteFile(outputPath, []byte("mp3"), 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	svc := NewMusicServiceWithPlaybackConfig(
		&fakeMusicRepository{filenameResult: &model.MusicFile{Path: "Jay/Track.flac"}},
		&fakeJamendoService{},
		PlaybackConfig{TranscodedAudioDir: cacheDir, TranscodedAudioBaseURL: "http://media.example"},
	)

	resp, err := svc.GetMusicByFilename(context.Background(), "Track.flac", "http://origin.example")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StreamURL != "http://media.example/audio-cache/Jay/Track.mp3" {
		t.Fatalf("unexpected stream url: %s", resp.StreamURL)
	}
}

func TestGetMusicByFilenameLosslessQualityUsesOriginal(t *testing.T) {
	cacheDir := t.TempDir()
	outputPath := filepath.Join(cacheDir, "Jay", "Track.mp3")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}
	if err := os.WriteFile(outputPath, []byte("mp3"), 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	svc := NewMusicServiceWithPlaybackConfig(
		&fakeMusicRepository{filenameResult: &model.MusicFile{Path: "Jay/Track.flac"}},
		&fakeJamendoService{},
		PlaybackConfig{TranscodedAudioDir: cacheDir, TranscodedAudioBaseURL: "http://media.example"},
	)

	resp, err := svc.GetMusicByFilenameWithOptions(context.Background(), "Track.flac", "http://origin.example", PlaybackOptions{Quality: "lossless"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StreamURL != "http://origin.example/uploads/Jay/Track.flac" {
		t.Fatalf("unexpected stream url: %s", resp.StreamURL)
	}
}

func TestGetMusicByFilenameFallsBackWhenTranscodedAudioMissing(t *testing.T) {
	svc := NewMusicServiceWithPlaybackConfig(
		&fakeMusicRepository{filenameResult: &model.MusicFile{Path: "Jay/Track.flac"}},
		&fakeJamendoService{},
		PlaybackConfig{TranscodedAudioDir: t.TempDir(), TranscodedAudioBaseURL: "http://media.example"},
	)

	resp, err := svc.GetMusicByFilename(context.Background(), "Track.flac", "http://origin.example")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StreamURL != "http://origin.example/uploads/Jay/Track.flac" {
		t.Fatalf("unexpected stream url: %s", resp.StreamURL)
	}
}

func TestGetMusicByFilenameKeepsMP3Original(t *testing.T) {
	svc := NewMusicServiceWithPlaybackConfig(
		&fakeMusicRepository{filenameResult: &model.MusicFile{Path: "Jay/Track.mp3"}},
		&fakeJamendoService{},
		PlaybackConfig{TranscodedAudioDir: t.TempDir(), TranscodedAudioBaseURL: "http://media.example"},
	)

	resp, err := svc.GetMusicByFilename(context.Background(), "Track.mp3", "http://origin.example")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StreamURL != "http://origin.example/uploads/Jay/Track.mp3" {
		t.Fatalf("unexpected stream url: %s", resp.StreamURL)
	}
}

func TestSearchMusicLocalFirstFallsBackToJamendo(t *testing.T) {
	svc := NewMusicService(
		&fakeMusicRepository{searchResult: []*model.MusicFile{}},
		&fakeJamendoService{
			configured: true,
			search: []*model.ExternalMusicTrack{{
				SourceID:    "1218138",
				Title:       "Mayday",
				Artist:      "Hasenchat",
				DurationSec: 252,
				CoverArtURL: "https://img.example/cover.jpg",
			}},
		},
	)

	items, err := svc.SearchMusic(context.Background(), "五月天", "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Path != "Mayday [jamendo-1218138].mp3" {
		t.Fatalf("unexpected virtual path: %q", items[0].Path)
	}
}

func TestSearchMusicJamendoFirstFallsBackToLocal(t *testing.T) {
	svc := NewMusicService(
		&fakeMusicRepository{searchResult: []*model.MusicFile{{
			Path:        "jay/hua_hai.mp3",
			Title:       "Hua Hai",
			Artist:      "Jay Chou",
			DurationSec: 260,
		}}},
		&fakeJamendoService{
			configured: true,
			searchErr:  errors.New("timeout"),
		},
	)

	items, err := svc.SearchMusic(context.Background(), "jay", "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Path != "jay/hua_hai.mp3" {
		t.Fatalf("unexpected local path: %q", items[0].Path)
	}
}
