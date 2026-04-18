package service

import (
	"context"
	"testing"
	"time"

	chartmodel "music-platform/internal/chart/model"
	musicmodel "music-platform/internal/music/model"
)

type fakeChartRepo struct {
	catalogMeta map[string]*chartmodel.HotTrackMeta
	allStats    []chartmodel.HotTrackStat
	dailyStats  []chartmodel.DailyHotTrackStat
}

func (f *fakeChartRepo) ResolveCatalogTrackMeta(ctx context.Context, musicPath string) (*chartmodel.HotTrackMeta, error) {
	if meta, ok := f.catalogMeta[musicPath]; ok {
		cloned := *meta
		return &cloned, nil
	}
	return nil, nil
}

func (f *fakeChartRepo) ListAllHotTrackStats(ctx context.Context) ([]chartmodel.HotTrackStat, error) {
	return f.allStats, nil
}

func (f *fakeChartRepo) ListDailyHotTrackStats(ctx context.Context, start time.Time) ([]chartmodel.DailyHotTrackStat, error) {
	return f.dailyStats, nil
}

type fakeLeaderboardStore struct {
	scores map[string]map[string]float64
	metas  map[string]*chartmodel.HotTrackMeta
}

func newFakeLeaderboardStore() *fakeLeaderboardStore {
	return &fakeLeaderboardStore{
		scores: make(map[string]map[string]float64),
		metas:  make(map[string]*chartmodel.HotTrackMeta),
	}
}

func (f *fakeLeaderboardStore) Available() bool {
	return true
}

func (f *fakeLeaderboardStore) IncrementPlay(ctx context.Context, totalKey, dayKey, musicPath string, dayTTL time.Duration) error {
	f.ensureScoreMap(totalKey)[musicPath]++
	f.ensureScoreMap(dayKey)[musicPath]++
	return nil
}

func (f *fakeLeaderboardStore) UpsertMeta(ctx context.Context, key string, meta *chartmodel.HotTrackMeta, ttl time.Duration) error {
	if meta == nil {
		return nil
	}
	cloned := *meta
	f.metas[key] = &cloned
	return nil
}

func (f *fakeLeaderboardStore) GetMeta(ctx context.Context, key string) (*chartmodel.HotTrackMeta, bool, error) {
	meta, ok := f.metas[key]
	if !ok {
		return nil, false, nil
	}
	cloned := *meta
	return &cloned, true, nil
}

func (f *fakeLeaderboardStore) TopN(ctx context.Context, key string, limit int64) ([]chartmodel.ScoredMusicPath, error) {
	source := f.scores[key]
	items := make([]chartmodel.ScoredMusicPath, 0, len(source))
	for musicPath, score := range source {
		items = append(items, chartmodel.ScoredMusicPath{
			MusicPath: musicPath,
			Score:     score,
		})
	}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Score > items[i].Score {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	if limit > 0 && int64(len(items)) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (f *fakeLeaderboardStore) UnionInto(ctx context.Context, dest string, keys []string, ttl time.Duration) error {
	unioned := make(map[string]float64)
	for _, key := range keys {
		for musicPath, score := range f.scores[key] {
			unioned[musicPath] += score
		}
	}
	f.scores[dest] = unioned
	return nil
}

func (f *fakeLeaderboardStore) ReplaceLeaderboard(ctx context.Context, key string, scores map[string]float64, ttl time.Duration) error {
	cloned := make(map[string]float64, len(scores))
	for musicPath, score := range scores {
		cloned[musicPath] = score
	}
	f.scores[key] = cloned
	return nil
}

func (f *fakeLeaderboardStore) Delete(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		delete(f.scores, key)
	}
	return nil
}

func (f *fakeLeaderboardStore) ensureScoreMap(key string) map[string]float64 {
	if _, ok := f.scores[key]; !ok {
		f.scores[key] = make(map[string]float64)
	}
	return f.scores[key]
}

type fakeJamendoService struct {
	track *musicmodel.ExternalMusicTrack
}

func (f *fakeJamendoService) IsConfigured() bool {
	return true
}

func (f *fakeJamendoService) Search(ctx context.Context, keyword string, limit int) ([]*musicmodel.ExternalMusicTrack, error) {
	return nil, nil
}

func (f *fakeJamendoService) GetTrack(ctx context.Context, id string) (*musicmodel.ExternalMusicTrack, error) {
	return f.track, nil
}

func TestRecordOnlinePlayWritesRedisScoresAndMeta(t *testing.T) {
	store := newFakeLeaderboardStore()
	repo := &fakeChartRepo{
		catalogMeta: map[string]*chartmodel.HotTrackMeta{
			"jay/七里香.mp3": {
				MusicPath:   "jay/七里香.mp3",
				Title:       "七里香",
				Artist:      "周杰伦",
				Album:       "七里香",
				DurationSec: 294.2,
				CoverArtURL: "jay/七里香.jpg",
				Source:      chartmodel.SourceCatalog,
			},
		},
	}

	fixedNow := time.Date(2026, 4, 18, 12, 0, 0, 0, time.Local)
	svc := NewChartService(repo, store, "http://127.0.0.1:8080", nil)
	svc.now = func() time.Time { return fixedNow }

	err := svc.RecordOnlinePlay(context.Background(), chartmodel.HotTrackPlay{
		MusicPath:   "http://127.0.0.1:8080/uploads/jay/七里香.mp3",
		Title:       "七里香",
		Artist:      "周杰伦",
		Album:       "七里香",
		DurationSec: 294.2,
	})
	if err != nil {
		t.Fatalf("RecordOnlinePlay error: %v", err)
	}

	if score := store.scores[svc.totalKey()]["jay/七里香.mp3"]; score != 1 {
		t.Fatalf("expected total score 1, got %v", score)
	}
	if score := store.scores[svc.dailyKey(fixedNow)]["jay/七里香.mp3"]; score != 1 {
		t.Fatalf("expected daily score 1, got %v", score)
	}

	meta, ok := store.metas[svc.metaKey("jay/七里香.mp3")]
	if !ok {
		t.Fatalf("expected metadata to be written")
	}
	if meta.CoverArtURL != "http://127.0.0.1:8080/uploads/jay/七里香.jpg" {
		t.Fatalf("unexpected cover url: %q", meta.CoverArtURL)
	}
}

func TestGetHotChartUsesRedisAndJamendoFallback(t *testing.T) {
	store := newFakeLeaderboardStore()
	repo := &fakeChartRepo{}
	svc := NewChartService(repo, store, "http://127.0.0.1:8080", &fakeJamendoService{
		track: &musicmodel.ExternalMusicTrack{
			Source:      "jamendo",
			SourceID:    "1218138",
			Title:       "Mayday",
			Artist:      "Hasenchat",
			Album:       "Mayday Album",
			DurationSec: 252,
			CoverArtURL: "https://img.example/mayday.jpg",
		},
	})

	store.scores[svc.totalKey()] = map[string]float64{
		"Mayday [jamendo-1218138].mp3": 9,
	}

	resp, err := svc.GetHotChart(context.Background(), chartmodel.HotChartQuery{
		Window: chartmodel.WindowAll,
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("GetHotChart error: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	item := resp.Items[0]
	if item.Source != chartmodel.SourceJamendo || item.SourceID != "1218138" {
		t.Fatalf("unexpected source: %+v", item)
	}
	if item.Title != "Mayday" || item.Artist != "Hasenchat" {
		t.Fatalf("unexpected jamendo item: %+v", item)
	}
}

func TestRebuildHotChartAllTimeReplacesRedisLeaderboard(t *testing.T) {
	store := newFakeLeaderboardStore()
	repo := &fakeChartRepo{
		allStats: []chartmodel.HotTrackStat{
			{MusicPath: "jay/七里香.mp3", MusicTitle: "七里香", Artist: "周杰伦", CoverArtPath: "jay/七里香.jpg", PlayCount: 18},
			{MusicPath: "Mayday [jamendo-1218138].mp3", MusicTitle: "Mayday", Artist: "Hasenchat", PlayCount: 9},
		},
	}
	svc := NewChartService(repo, store, "http://127.0.0.1:8080", nil)

	resp, err := svc.RebuildHotChart(context.Background(), chartmodel.HotChartRebuildQuery{Window: chartmodel.WindowAll})
	if err != nil {
		t.Fatalf("RebuildHotChart error: %v", err)
	}
	if resp.Window != chartmodel.WindowAll || resp.RebuiltItems != 2 {
		t.Fatalf("unexpected rebuild response: %+v", resp)
	}
	if score := store.scores[svc.totalKey()]["jay/七里香.mp3"]; score != 18 {
		t.Fatalf("expected rebuilt score 18, got %v", score)
	}
}

func TestRebuildHotChartThirtyDaysReplacesDailyBuckets(t *testing.T) {
	store := newFakeLeaderboardStore()
	repo := &fakeChartRepo{
		dailyStats: []chartmodel.DailyHotTrackStat{
			{
				Day: "20260418",
				HotTrackStat: chartmodel.HotTrackStat{
					MusicPath:  "jay/七里香.mp3",
					MusicTitle: "七里香",
					Artist:     "周杰伦",
					PlayCount:  3,
				},
			},
			{
				Day: "20260417",
				HotTrackStat: chartmodel.HotTrackStat{
					MusicPath:  "Mayday [jamendo-1218138].mp3",
					MusicTitle: "Mayday",
					Artist:     "Hasenchat",
					PlayCount:  5,
				},
			},
		},
	}

	fixedNow := time.Date(2026, 4, 18, 9, 0, 0, 0, time.Local)
	svc := NewChartService(repo, store, "http://127.0.0.1:8080", nil)
	svc.now = func() time.Time { return fixedNow }

	resp, err := svc.RebuildHotChart(context.Background(), chartmodel.HotChartRebuildQuery{Window: chartmodel.Window30d})
	if err != nil {
		t.Fatalf("RebuildHotChart error: %v", err)
	}
	if resp.RebuiltBuckets != 30 || resp.RebuiltItems != 2 {
		t.Fatalf("unexpected rebuild response: %+v", resp)
	}
	if score := store.scores[svc.dailyKeyFromBucket("20260418")]["jay/七里香.mp3"]; score != 3 {
		t.Fatalf("expected day score 3, got %v", score)
	}
	if score := store.scores[svc.dailyKeyFromBucket("20260417")]["Mayday [jamendo-1218138].mp3"]; score != 5 {
		t.Fatalf("expected day score 5, got %v", score)
	}
}
