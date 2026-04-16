package service

import (
	"context"
	"testing"
	"time"

	"music-platform/internal/music/model"
	playlistmodel "music-platform/internal/playlist/model"
)

type fakePlaylistRepository struct {
	playlist *playlistmodel.Playlist
	items    []playlistmodel.PlaylistItemRecord
}

func (f *fakePlaylistRepository) CreatePlaylist(userAccount string, req playlistmodel.CreatePlaylistRequest) (int64, error) {
	return 0, nil
}

func (f *fakePlaylistRepository) ListPlaylists(userAccount string, page, pageSize int) ([]playlistmodel.Playlist, int, error) {
	return nil, 0, nil
}

func (f *fakePlaylistRepository) GetPlaylistDetail(userAccount string, playlistID int64) (*playlistmodel.Playlist, []playlistmodel.PlaylistItemRecord, error) {
	return f.playlist, f.items, nil
}

func (f *fakePlaylistRepository) UpdatePlaylist(userAccount string, playlistID int64, req playlistmodel.UpdatePlaylistRequest) error {
	return nil
}

func (f *fakePlaylistRepository) DeletePlaylist(userAccount string, playlistID int64) error {
	return nil
}

func (f *fakePlaylistRepository) AddPlaylistItems(userAccount string, playlistID int64, items []playlistmodel.PlaylistTrackInput) (int64, int64, error) {
	return 0, 0, nil
}

func (f *fakePlaylistRepository) RemovePlaylistItems(userAccount string, playlistID int64, musicPaths []string) (int64, error) {
	return 0, nil
}

func (f *fakePlaylistRepository) ReorderPlaylistItems(userAccount string, playlistID int64, items []playlistmodel.PlaylistReorderItem) error {
	return nil
}

type fakePlaylistJamendoService struct {
	configured bool
	track      *model.ExternalMusicTrack
}

func (f *fakePlaylistJamendoService) IsConfigured() bool {
	return f.configured
}

func (f *fakePlaylistJamendoService) Search(ctx context.Context, keyword string, limit int) ([]*model.ExternalMusicTrack, error) {
	return nil, nil
}

func (f *fakePlaylistJamendoService) GetTrack(ctx context.Context, id string) (*model.ExternalMusicTrack, error) {
	return f.track, nil
}

func TestGetPlaylistDetailBackfillsJamendoCover(t *testing.T) {
	repo := &fakePlaylistRepository{
		playlist: &playlistmodel.Playlist{
			ID:          1,
			UserAccount: "u1",
			Name:        "mix",
			CreatedAt:   time.Unix(0, 0),
			UpdatedAt:   time.Unix(0, 0),
		},
		items: []playlistmodel.PlaylistItemRecord{{
			ID:          1,
			PlaylistID:  1,
			UserAccount: "u1",
			Position:    1,
			MusicPath:   "Mayday [jamendo-1218138].mp3",
			MusicTitle:  "Mayday",
			Artist:      "Hasenchat",
			IsLocal:     false,
			CreatedAt:   time.Unix(0, 0),
			UpdatedAt:   time.Unix(0, 0),
		}},
	}
	svc := NewPlaylistService(repo, "http://127.0.0.1:8080", nil, nil, &fakePlaylistJamendoService{
		configured: true,
		track: &model.ExternalMusicTrack{
			SourceID:    "1218138",
			CoverArtURL: "https://img.example/1218138.jpg",
		},
	})

	detail, err := svc.GetPlaylistDetail("u1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(detail.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(detail.Items))
	}
	if detail.Items[0].CoverArtURL != "https://img.example/1218138.jpg" {
		t.Fatalf("unexpected cover url: %q", detail.Items[0].CoverArtURL)
	}
}
