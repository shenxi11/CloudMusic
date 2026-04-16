package service

import (
	"context"
	"errors"
	"testing"

	"music-platform/internal/music/model"
)

type fakeMusicRepository struct {
	searchResult []*model.MusicFile
	searchErr    error
}

func (f *fakeMusicRepository) FindAll(ctx context.Context) ([]*model.MusicFile, error) {
	return nil, nil
}

func (f *fakeMusicRepository) FindByPath(ctx context.Context, path string) (*model.MusicFile, error) {
	return nil, nil
}

func (f *fakeMusicRepository) FindByPathLike(ctx context.Context, filename string) (*model.MusicFile, error) {
	return nil, nil
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
