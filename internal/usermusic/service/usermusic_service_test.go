package service

import (
	"context"
	"testing"
	"time"

	chartmodel "music-platform/internal/chart/model"
	"music-platform/internal/music/model"
	usermodel "music-platform/internal/usermusic/model"
)

type fakeUserMusicRepository struct {
	favorites []usermodel.FavoriteMusic
	history   []usermodel.PlayHistory
}

func (f *fakeUserMusicRepository) AddFavorite(userAccount string, req usermodel.AddFavoriteRequest) error {
	return nil
}

func (f *fakeUserMusicRepository) RemoveFavorite(userAccount, musicPath string) error {
	return nil
}

func (f *fakeUserMusicRepository) ListFavorites(userAccount string) ([]usermodel.FavoriteMusic, error) {
	return f.favorites, nil
}

func (f *fakeUserMusicRepository) AddPlayHistory(userAccount string, req usermodel.AddPlayHistoryRequest) error {
	return nil
}

func (f *fakeUserMusicRepository) ListPlayHistory(userAccount string, limit int) ([]usermodel.PlayHistory, error) {
	return f.history, nil
}

func (f *fakeUserMusicRepository) ListPlayHistoryDistinct(userAccount string, limit int) ([]usermodel.PlayHistory, error) {
	return f.history, nil
}

func (f *fakeUserMusicRepository) DeletePlayHistory(userAccount string, musicPaths []string) (int64, error) {
	return 0, nil
}

func (f *fakeUserMusicRepository) ClearPlayHistory(userAccount string) (int64, error) {
	return 0, nil
}

type fakeJamendoService struct {
	configured bool
	track      *model.ExternalMusicTrack
}

type fakeChartWriter struct {
	lastPlay *chartmodel.HotTrackPlay
}

func (f *fakeJamendoService) IsConfigured() bool {
	return f.configured
}

func (f *fakeJamendoService) Search(ctx context.Context, keyword string, limit int) ([]*model.ExternalMusicTrack, error) {
	return nil, nil
}

func (f *fakeJamendoService) GetTrack(ctx context.Context, id string) (*model.ExternalMusicTrack, error) {
	return f.track, nil
}

func (f *fakeChartWriter) RecordOnlinePlay(ctx context.Context, play chartmodel.HotTrackPlay) error {
	cloned := play
	f.lastPlay = &cloned
	return nil
}

func TestListFavoritesBackfillsJamendoCover(t *testing.T) {
	repo := &fakeUserMusicRepository{
		favorites: []usermodel.FavoriteMusic{{
			UserAccount: "u1",
			MusicPath:   "Mayday [jamendo-1218138].mp3",
			MusicTitle:  "Mayday",
			Artist:      "Hasenchat",
			IsLocal:     false,
			CreatedAt:   time.Unix(0, 0),
		}},
	}
	svc := NewUserMusicService(repo, "http://127.0.0.1:8080", nil, nil, nil, &fakeJamendoService{
		configured: true,
		track: &model.ExternalMusicTrack{
			SourceID:    "1218138",
			CoverArtURL: "https://img.example/1218138.jpg",
		},
	})

	items, err := svc.ListFavorites("u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].CoverArtURL == nil {
		t.Fatalf("expected cover art url, got %+v", items)
	}
	if *items[0].CoverArtURL != "https://img.example/1218138.jpg" {
		t.Fatalf("unexpected cover: %q", *items[0].CoverArtURL)
	}
}

func TestAddPlayHistoryWritesHotChartForOnlineMusic(t *testing.T) {
	repo := &fakeUserMusicRepository{}
	writer := &fakeChartWriter{}
	svc := NewUserMusicService(repo, "http://127.0.0.1:8080", nil, nil, writer, &fakeJamendoService{})

	err := svc.AddPlayHistory("u1", usermodel.AddPlayHistoryRequest{
		MusicPath:   "jay/七里香.mp3",
		MusicTitle:  "七里香",
		Artist:      "周杰伦",
		Album:       "七里香",
		DurationSec: 294,
		IsLocal:     false,
	})
	if err != nil {
		t.Fatalf("AddPlayHistory error: %v", err)
	}
	if writer.lastPlay == nil {
		t.Fatalf("expected chart writer to be called")
	}
	if writer.lastPlay.MusicPath != "jay/七里香.mp3" || writer.lastPlay.Title != "七里香" {
		t.Fatalf("unexpected chart play payload: %+v", writer.lastPlay)
	}
}
