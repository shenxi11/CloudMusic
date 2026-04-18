package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	chartmodel "music-platform/internal/chart/model"
	chartstore "music-platform/internal/chart/store"
	"music-platform/internal/common/cache"
	"music-platform/internal/music/compat"
	"music-platform/internal/music/external"
)

const (
	dailyBucketTTL = 45 * 24 * time.Hour
	metaTTL        = 30 * 24 * time.Hour
	tempUnionTTL   = 30 * time.Second
)

var (
	ErrLeaderboardUnavailable = errors.New("chart leaderboard store unavailable")
	ErrInvalidRebuildWindow   = errors.New("invalid chart rebuild window")
)

type chartRepository interface {
	ResolveCatalogTrackMeta(ctx context.Context, musicPath string) (*chartmodel.HotTrackMeta, error)
	ListAllHotTrackStats(ctx context.Context) ([]chartmodel.HotTrackStat, error)
	ListDailyHotTrackStats(ctx context.Context, start time.Time) ([]chartmodel.DailyHotTrackStat, error)
}

type leaderboardStore interface {
	Available() bool
	IncrementPlay(ctx context.Context, totalKey, dayKey, musicPath string, dayTTL time.Duration) error
	UpsertMeta(ctx context.Context, key string, meta *chartmodel.HotTrackMeta, ttl time.Duration) error
	GetMeta(ctx context.Context, key string) (*chartmodel.HotTrackMeta, bool, error)
	TopN(ctx context.Context, key string, limit int64) ([]chartmodel.ScoredMusicPath, error)
	UnionInto(ctx context.Context, dest string, keys []string, ttl time.Duration) error
	ReplaceLeaderboard(ctx context.Context, key string, scores map[string]float64, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

type ChartService struct {
	repo           chartRepository
	store          leaderboardStore
	baseURL        string
	jamendoService external.JamendoService
	now            func() time.Time
}

func NewChartService(repo chartRepository, store leaderboardStore, baseURL string, jamendoService external.JamendoService) *ChartService {
	return &ChartService{
		repo:           repo,
		store:          store,
		baseURL:        strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		jamendoService: jamendoService,
		now:            time.Now,
	}
}

func (s *ChartService) GetHotChart(ctx context.Context, query chartmodel.HotChartQuery) (*chartmodel.HotChartResponse, error) {
	if !s.storeAvailable() {
		return nil, ErrLeaderboardUnavailable
	}

	window := normalizeWindow(query.Window)
	limit := normalizeLimit(query.Limit)

	rows, err := s.loadLeaderboard(ctx, window, limit)
	if err != nil {
		return nil, normalizeStoreErr(err)
	}

	resp := &chartmodel.HotChartResponse{
		ChartID:     chartmodel.HotChartID,
		Title:       chartmodel.HotChartTitle,
		Window:      window,
		GeneratedAt: s.now().Format(time.RFC3339),
		Items:       make([]chartmodel.HotChartItem, 0, len(rows)),
	}

	for i, row := range rows {
		musicPath := normalizeMusicPath(row.MusicPath)
		if musicPath == "" {
			continue
		}
		meta, err := s.hydrateMeta(ctx, musicPath)
		if err != nil {
			return nil, err
		}
		resp.Items = append(resp.Items, s.toHotChartItem(musicPath, meta, i+1, int64(math.Round(row.Score))))
	}

	return resp, nil
}

func (s *ChartService) RecordOnlinePlay(ctx context.Context, play chartmodel.HotTrackPlay) error {
	if play.IsLocal {
		return nil
	}
	if !s.storeAvailable() {
		return ErrLeaderboardUnavailable
	}

	musicPath := normalizeMusicPath(play.MusicPath)
	if musicPath == "" {
		return nil
	}

	now := s.now()
	if err := s.store.IncrementPlay(ctx, s.totalKey(), s.dailyKey(now), musicPath, dailyBucketTTL); err != nil {
		return normalizeStoreErr(err)
	}

	meta, err := s.buildMetaFromPlay(ctx, musicPath, play)
	if err != nil {
		return err
	}
	if meta == nil {
		return nil
	}

	if err := s.store.UpsertMeta(ctx, s.metaKey(musicPath), meta, metaTTL); err != nil {
		return normalizeStoreErr(err)
	}
	return nil
}

func (s *ChartService) RebuildHotChart(ctx context.Context, query chartmodel.HotChartRebuildQuery) (*chartmodel.HotChartRebuildResponse, error) {
	if !s.storeAvailable() {
		return nil, ErrLeaderboardUnavailable
	}

	window, ok := normalizeRebuildWindow(query.Window)
	if !ok {
		return nil, ErrInvalidRebuildWindow
	}

	switch window {
	case chartmodel.WindowAll:
		return s.rebuildAllTime(ctx)
	case chartmodel.Window7d, chartmodel.Window30d:
		return s.rebuildDailyWindow(ctx, window)
	default:
		return nil, ErrInvalidRebuildWindow
	}
}

func (s *ChartService) rebuildAllTime(ctx context.Context) (*chartmodel.HotChartRebuildResponse, error) {
	stats, err := s.repo.ListAllHotTrackStats(ctx)
	if err != nil {
		return nil, err
	}

	scores := make(map[string]float64, len(stats))
	for _, stat := range stats {
		musicPath := normalizeMusicPath(stat.MusicPath)
		if musicPath == "" {
			continue
		}
		scores[musicPath] += float64(stat.PlayCount)

		meta := s.buildMetaFromStat(musicPath, stat)
		if err := s.store.UpsertMeta(ctx, s.metaKey(musicPath), meta, metaTTL); err != nil {
			return nil, normalizeStoreErr(err)
		}
	}

	if err := s.store.ReplaceLeaderboard(ctx, s.totalKey(), scores, 0); err != nil {
		return nil, normalizeStoreErr(err)
	}

	return &chartmodel.HotChartRebuildResponse{
		Window:         chartmodel.WindowAll,
		RebuiltBuckets: 1,
		RebuiltItems:   int64(len(scores)),
		GeneratedAt:    s.now().Format(time.RFC3339),
	}, nil
}

func (s *ChartService) rebuildDailyWindow(ctx context.Context, window string) (*chartmodel.HotChartRebuildResponse, error) {
	start := s.windowStart(window, s.now())
	stats, err := s.repo.ListDailyHotTrackStats(ctx, start)
	if err != nil {
		return nil, err
	}

	dayKeys := s.recentDayKeys(window, s.now())
	if err := s.store.Delete(ctx, dayKeys...); err != nil {
		return nil, normalizeStoreErr(err)
	}

	grouped := make(map[string]map[string]float64, len(dayKeys))
	for _, stat := range stats {
		musicPath := normalizeMusicPath(stat.MusicPath)
		if musicPath == "" || strings.TrimSpace(stat.Day) == "" {
			continue
		}

		redisKey := s.dailyKeyFromBucket(stat.Day)
		if _, ok := grouped[redisKey]; !ok {
			grouped[redisKey] = make(map[string]float64)
		}
		grouped[redisKey][musicPath] += float64(stat.PlayCount)

		meta := s.buildMetaFromStat(musicPath, stat.HotTrackStat)
		if err := s.store.UpsertMeta(ctx, s.metaKey(musicPath), meta, metaTTL); err != nil {
			return nil, normalizeStoreErr(err)
		}
	}

	for _, dayKey := range dayKeys {
		if err := s.store.ReplaceLeaderboard(ctx, dayKey, grouped[dayKey], dailyBucketTTL); err != nil {
			return nil, normalizeStoreErr(err)
		}
	}

	return &chartmodel.HotChartRebuildResponse{
		Window:         window,
		RebuiltBuckets: len(dayKeys),
		RebuiltItems:   int64(len(stats)),
		GeneratedAt:    s.now().Format(time.RFC3339),
	}, nil
}

func (s *ChartService) loadLeaderboard(ctx context.Context, window string, limit int) ([]chartmodel.ScoredMusicPath, error) {
	switch window {
	case chartmodel.WindowAll:
		return s.store.TopN(ctx, s.totalKey(), int64(limit))
	case chartmodel.Window7d, chartmodel.Window30d:
		dest := s.tempUnionKey(window)
		if err := s.store.UnionInto(ctx, dest, s.recentDayKeys(window, s.now()), tempUnionTTL); err != nil {
			return nil, err
		}
		return s.store.TopN(ctx, dest, int64(limit))
	default:
		return s.store.TopN(ctx, s.totalKey(), int64(limit))
	}
}

func (s *ChartService) hydrateMeta(ctx context.Context, musicPath string) (*chartmodel.HotTrackMeta, error) {
	if !s.storeAvailable() {
		return nil, ErrLeaderboardUnavailable
	}

	if meta, ok, err := s.store.GetMeta(ctx, s.metaKey(musicPath)); err != nil {
		return nil, normalizeStoreErr(err)
	} else if ok && meta != nil {
		s.ensureMetaDefaults(meta, musicPath)
		return meta, nil
	}

	meta, err := s.fetchFallbackMeta(ctx, musicPath)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		meta = &chartmodel.HotTrackMeta{
			MusicPath: musicPath,
		}
	}
	s.ensureMetaDefaults(meta, musicPath)

	if err := s.store.UpsertMeta(ctx, s.metaKey(musicPath), meta, metaTTL); err != nil {
		return nil, normalizeStoreErr(err)
	}
	return meta, nil
}

func (s *ChartService) buildMetaFromPlay(ctx context.Context, musicPath string, play chartmodel.HotTrackPlay) (*chartmodel.HotTrackMeta, error) {
	meta := &chartmodel.HotTrackMeta{
		MusicPath:   musicPath,
		Title:       strings.TrimSpace(play.Title),
		Artist:      strings.TrimSpace(play.Artist),
		Album:       strings.TrimSpace(play.Album),
		DurationSec: play.DurationSec,
	}

	fallback, err := s.fetchFallbackMeta(ctx, musicPath)
	if err != nil {
		return nil, err
	}
	meta = mergeMeta(meta, fallback)
	s.ensureMetaDefaults(meta, musicPath)
	return meta, nil
}

func (s *ChartService) fetchFallbackMeta(ctx context.Context, musicPath string) (*chartmodel.HotTrackMeta, error) {
	if sourceID, ok := compat.ParseJamendoSourceID(musicPath); ok {
		meta := &chartmodel.HotTrackMeta{
			MusicPath: musicPath,
			Source:    chartmodel.SourceJamendo,
			SourceID:  sourceID,
		}
		if s.jamendoService == nil || !s.jamendoService.IsConfigured() {
			return meta, nil
		}

		track, err := s.jamendoService.GetTrack(ctx, sourceID)
		if err != nil {
			return meta, nil
		}
		if track == nil {
			return meta, nil
		}

		meta.Title = strings.TrimSpace(track.Title)
		meta.Artist = strings.TrimSpace(track.Artist)
		meta.Album = strings.TrimSpace(track.Album)
		meta.DurationSec = track.DurationSec
		meta.CoverArtURL = strings.TrimSpace(track.CoverArtURL)
		return meta, nil
	}

	if s.repo == nil {
		return &chartmodel.HotTrackMeta{
			MusicPath: musicPath,
			Source:    chartmodel.SourceCatalog,
		}, nil
	}

	meta, err := s.repo.ResolveCatalogTrackMeta(ctx, musicPath)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return &chartmodel.HotTrackMeta{
			MusicPath: musicPath,
			Source:    chartmodel.SourceCatalog,
		}, nil
	}
	s.ensureMetaDefaults(meta, musicPath)
	if cover := strings.TrimSpace(meta.CoverArtURL); cover != "" {
		meta.CoverArtURL = s.buildCatalogCoverURL(cover)
	}
	return meta, nil
}

func (s *ChartService) buildMetaFromStat(musicPath string, stat chartmodel.HotTrackStat) *chartmodel.HotTrackMeta {
	meta := &chartmodel.HotTrackMeta{
		MusicPath:   musicPath,
		Title:       strings.TrimSpace(stat.MusicTitle),
		Artist:      strings.TrimSpace(stat.Artist),
		Album:       strings.TrimSpace(stat.Album),
		DurationSec: stat.DurationSec,
	}
	if sourceID, ok := compat.ParseJamendoSourceID(musicPath); ok {
		meta.Source = chartmodel.SourceJamendo
		meta.SourceID = sourceID
	} else {
		meta.Source = chartmodel.SourceCatalog
		meta.CoverArtURL = s.buildCatalogCoverURL(stat.CoverArtPath)
	}
	s.ensureMetaDefaults(meta, musicPath)
	return meta
}

func (s *ChartService) ensureMetaDefaults(meta *chartmodel.HotTrackMeta, musicPath string) {
	if meta == nil {
		return
	}
	meta.MusicPath = normalizeMusicPath(firstNonEmpty(meta.MusicPath, musicPath))
	if sourceID, ok := compat.ParseJamendoSourceID(meta.MusicPath); ok {
		meta.Source = chartmodel.SourceJamendo
		meta.SourceID = firstNonEmpty(meta.SourceID, sourceID)
	} else if strings.TrimSpace(meta.Source) == "" {
		meta.Source = chartmodel.SourceCatalog
	}
	if strings.TrimSpace(meta.Title) == "" {
		meta.Title = deriveTitleFromPath(meta.MusicPath)
	}
	if meta.Source == chartmodel.SourceCatalog && strings.TrimSpace(meta.CoverArtURL) != "" {
		meta.CoverArtURL = s.buildCatalogCoverURL(meta.CoverArtURL)
	}
}

func (s *ChartService) toHotChartItem(musicPath string, meta *chartmodel.HotTrackMeta, rank int, playCount int64) chartmodel.HotChartItem {
	item := chartmodel.HotChartItem{
		Rank:      rank,
		MusicPath: musicPath,
		Path:      musicPath,
		Source:    chartmodel.SourceCatalog,
		PlayCount: playCount,
	}
	if meta == nil {
		item.Title = deriveTitleFromPath(musicPath)
		return item
	}

	item.Title = strings.TrimSpace(meta.Title)
	item.Artist = strings.TrimSpace(meta.Artist)
	item.Album = strings.TrimSpace(meta.Album)
	item.DurationSec = meta.DurationSec
	item.Source = firstNonEmpty(strings.TrimSpace(meta.Source), chartmodel.SourceCatalog)
	item.SourceID = strings.TrimSpace(meta.SourceID)
	if cover := strings.TrimSpace(meta.CoverArtURL); cover != "" {
		item.CoverArtURL = &cover
	}
	if item.Title == "" {
		item.Title = deriveTitleFromPath(musicPath)
	}
	return item
}

func (s *ChartService) totalKey() string {
	return cache.PrefixMusic + "chart:hot:all"
}

func (s *ChartService) dailyKey(now time.Time) string {
	return s.dailyKeyFromBucket(now.In(time.Local).Format("20060102"))
}

func (s *ChartService) dailyKeyFromBucket(bucket string) string {
	return cache.PrefixMusic + "chart:hot:day:" + strings.TrimSpace(bucket)
}

func (s *ChartService) metaKey(musicPath string) string {
	return cache.PrefixMusic + "chart:meta:" + url.QueryEscape(strings.TrimSpace(musicPath))
}

func (s *ChartService) tempUnionKey(window string) string {
	return fmt.Sprintf("%schart:hot:tmp:%s:%d", cache.PrefixMusic, window, s.now().UnixNano())
}

func (s *ChartService) recentDayKeys(window string, now time.Time) []string {
	days := 30
	if window == chartmodel.Window7d {
		days = 7
	}

	keys := make([]string, 0, days)
	start := dayStart(now)
	for i := 0; i < days; i++ {
		bucket := start.AddDate(0, 0, -i).Format("20060102")
		keys = append(keys, s.dailyKeyFromBucket(bucket))
	}
	return keys
}

func (s *ChartService) windowStart(window string, now time.Time) time.Time {
	start := dayStart(now)
	switch window {
	case chartmodel.Window7d:
		return start.AddDate(0, 0, -6)
	case chartmodel.Window30d:
		return start.AddDate(0, 0, -29)
	default:
		return time.Time{}
	}
}

func (s *ChartService) buildCatalogCoverURL(coverPath string) string {
	trimmed := strings.TrimSpace(coverPath)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	if s.baseURL == "" {
		return trimmed
	}
	return fmt.Sprintf("%s/uploads/%s", s.baseURL, strings.TrimLeft(trimmed, "/"))
}

func (s *ChartService) storeAvailable() bool {
	return s != nil && s.store != nil && s.store.Available()
}

func normalizeStoreErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, chartstore.ErrUnavailable) {
		return ErrLeaderboardUnavailable
	}
	return err
}

func normalizeWindow(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case chartmodel.Window7d:
		return chartmodel.Window7d
	case chartmodel.WindowAll:
		return chartmodel.WindowAll
	default:
		return chartmodel.Window30d
	}
}

func normalizeRebuildWindow(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case chartmodel.WindowAll:
		return chartmodel.WindowAll, true
	case chartmodel.Window7d:
		return chartmodel.Window7d, true
	case chartmodel.Window30d:
		return chartmodel.Window30d, true
	default:
		return "", false
	}
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func normalizeMusicPath(musicPath string) string {
	trimmed := strings.TrimSpace(musicPath)
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, "/uploads/") {
		return strings.TrimPrefix(trimmed, "/uploads/")
	}

	if idx := strings.Index(trimmed, "/uploads/"); idx >= 0 {
		return strings.TrimPrefix(trimmed[idx+len("/uploads/"):], "/")
	}

	return trimmed
}

func deriveTitleFromPath(musicPath string) string {
	trimmed := strings.TrimSpace(musicPath)
	if trimmed == "" {
		return ""
	}
	filename := filepath.Base(trimmed)
	ext := filepath.Ext(filename)
	return strings.TrimSpace(strings.TrimSuffix(filename, ext))
}

func mergeMeta(primary, fallback *chartmodel.HotTrackMeta) *chartmodel.HotTrackMeta {
	if primary == nil && fallback == nil {
		return nil
	}
	if primary == nil {
		cloned := *fallback
		return &cloned
	}

	out := *primary
	if fallback == nil {
		return &out
	}
	if strings.TrimSpace(out.MusicPath) == "" {
		out.MusicPath = fallback.MusicPath
	}
	if strings.TrimSpace(out.Title) == "" {
		out.Title = fallback.Title
	}
	if strings.TrimSpace(out.Artist) == "" {
		out.Artist = fallback.Artist
	}
	if strings.TrimSpace(out.Album) == "" {
		out.Album = fallback.Album
	}
	if out.DurationSec <= 0 {
		out.DurationSec = fallback.DurationSec
	}
	if strings.TrimSpace(out.CoverArtURL) == "" {
		out.CoverArtURL = fallback.CoverArtURL
	}
	if strings.TrimSpace(out.Source) == "" {
		out.Source = fallback.Source
	}
	if strings.TrimSpace(out.SourceID) == "" {
		out.SourceID = fallback.SourceID
	}
	return &out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func dayStart(ts time.Time) time.Time {
	local := ts.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
}
