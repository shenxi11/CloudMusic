package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	"music-platform/internal/recommend/model"
	"music-platform/internal/recommend/repository"
)

const defaultModelName = "rule_hybrid"

type RecommendService struct {
	repo    *repository.RecommendRepository
	baseURL string
}

func NewRecommendService(repo *repository.RecommendRepository, baseURL string) *RecommendService {
	return &RecommendService{
		repo:    repo,
		baseURL: strings.TrimSuffix(baseURL, "/"),
	}
}

func (s *RecommendService) GetRecommendations(ctx context.Context, req model.RecommendQuery) (*model.RecommendationListData, error) {
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return nil, fmt.Errorf("user_id不能为空")
	}

	scene := normalizeScene(req.Scene)
	limit := normalizeLimit(req.Limit, 20, 100)
	excludePlayed := req.ExcludePlayed

	candidateLimit := limit * 40
	if candidateLimit < 400 {
		candidateLimit = 400
	}
	if candidateLimit > 3000 {
		candidateLimit = 3000
	}

	candidates, err := s.repo.ListCandidates(ctx, candidateLimit)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return &model.RecommendationListData{
			RequestID:    newRequestID(),
			UserID:       userID,
			Scene:        scene,
			ModelVersion: defaultModelName + "_v1",
			Items:        []model.RecommendationItem{},
		}, nil
	}

	userSong, userArtist, playedSet, err := s.repo.GetUserHistoryStats(ctx, userID)
	if err != nil {
		return nil, err
	}
	globalHot, err := s.repo.GetGlobalHotScore(ctx, 2000)
	if err != nil {
		return nil, err
	}
	feedbackAdjust, err := s.repo.GetUserFeedbackAdjust(ctx, userID)
	if err != nil {
		return nil, err
	}

	maxUserSong := maxMapValue(userSong)
	maxUserArtist := maxMapValue(userArtist)

	type scored struct {
		item model.RecommendationItem
	}
	scoredList := make([]scored, 0, len(candidates))
	for _, cand := range candidates {
		if excludePlayed {
			if _, ok := playedSet[cand.Path]; ok {
				continue
			}
		}

		artistKey := normalizeArtist(cand.Artist)
		cfScore := normalize(userSong[cand.Path], maxUserSong)
		contentScore := normalize(userArtist[artistKey], maxUserArtist)
		hotScore := globalHot[cand.Path]
		adjScore := feedbackAdjust[cand.Path]

		score := 0.55*cfScore + 0.30*contentScore + 0.15*hotScore + 0.12*adjScore
		if score < -0.5 {
			continue
		}
		source, reason := resolveReason(cfScore, contentScore, hotScore, adjScore)
		scoredList = append(scoredList, scored{
			item: s.toRecommendationItem(cand, score, source, reason),
		})
	}

	if len(scoredList) == 0 {
		for _, cand := range candidates {
			hotScore := globalHot[cand.Path]
			scoredList = append(scoredList, scored{
				item: s.toRecommendationItem(cand, 0.15*hotScore, "hot", "trending_now"),
			})
		}
	}

	sort.SliceStable(scoredList, func(i, j int) bool {
		if scoredList[i].item.Score == scoredList[j].item.Score {
			return scoredList[i].item.Path < scoredList[j].item.Path
		}
		return scoredList[i].item.Score > scoredList[j].item.Score
	})

	out := make([]model.RecommendationItem, 0, limit)
	seen := make(map[string]struct{}, limit)
	for _, x := range scoredList {
		if _, ok := seen[x.item.Path]; ok {
			continue
		}
		seen[x.item.Path] = struct{}{}
		out = append(out, x.item)
		if len(out) >= limit {
			break
		}
	}

	return &model.RecommendationListData{
		RequestID:    newRequestID(),
		UserID:       userID,
		Scene:        scene,
		ModelVersion: defaultModelName + "_v1",
		NextCursor:   "",
		Items:        out,
	}, nil
}

func (s *RecommendService) GetSimilarBySong(ctx context.Context, songID string, limit int) (*model.RecommendationListData, error) {
	anchorPath := decodeSongID(songID)
	if strings.TrimSpace(anchorPath) == "" {
		return nil, fmt.Errorf("song_id不能为空")
	}
	limit = normalizeLimit(limit, 20, 100)

	anchor, err := s.repo.GetSongByPath(ctx, anchorPath)
	if err != nil {
		return nil, fmt.Errorf("未找到锚点歌曲: %w", err)
	}

	candidates, err := s.repo.ListCandidates(ctx, 2500)
	if err != nil {
		return nil, err
	}
	artistKey := normalizeArtist(anchor.Artist)
	albumKey := normalizeAlbum(anchor.Album)

	type scored struct {
		item model.RecommendationItem
	}
	tmp := make([]scored, 0, len(candidates))
	for _, cand := range candidates {
		if cand.Path == anchor.Path {
			continue
		}
		score := 0.0
		source := "content"
		reason := "same_style"
		if normalizeArtist(cand.Artist) == artistKey {
			score += 0.9
			reason = "same_artist"
		}
		if albumKey != "" && normalizeAlbum(cand.Album) == albumKey {
			score += 0.4
			reason = "same_album"
		}
		if hasCommonToken(cand.Title, anchor.Title) {
			score += 0.2
		}
		if score <= 0 {
			continue
		}
		tmp = append(tmp, scored{
			item: s.toRecommendationItem(cand, score, source, reason),
		})
	}

	sort.SliceStable(tmp, func(i, j int) bool {
		if tmp[i].item.Score == tmp[j].item.Score {
			return tmp[i].item.Path < tmp[j].item.Path
		}
		return tmp[i].item.Score > tmp[j].item.Score
	})

	items := make([]model.RecommendationItem, 0, limit)
	for i := 0; i < len(tmp) && len(items) < limit; i++ {
		items = append(items, tmp[i].item)
	}

	if len(items) < limit {
		hotScore, _ := s.repo.GetGlobalHotScore(ctx, 2000)
		seen := make(map[string]struct{}, len(items)+1)
		seen[anchor.Path] = struct{}{}
		for _, it := range items {
			seen[it.Path] = struct{}{}
		}
		for _, cand := range candidates {
			if len(items) >= limit {
				break
			}
			if _, ok := seen[cand.Path]; ok {
				continue
			}
			if hotScore[cand.Path] <= 0 {
				continue
			}
			items = append(items, s.toRecommendationItem(cand, 0.15*hotScore[cand.Path], "hot", "trending_now"))
			seen[cand.Path] = struct{}{}
		}

		for _, cand := range candidates {
			if len(items) >= limit {
				break
			}
			if _, ok := seen[cand.Path]; ok {
				continue
			}
			items = append(items, s.toRecommendationItem(cand, 0.01, "hot", "fallback_catalog"))
			seen[cand.Path] = struct{}{}
		}
	}

	return &model.RecommendationListData{
		RequestID:    newRequestID(),
		UserID:       "",
		Scene:        "detail",
		ModelVersion: defaultModelName + "_v1",
		Items:        items,
	}, nil
}

func (s *RecommendService) SaveFeedback(ctx context.Context, req model.FeedbackRequest) error {
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return fmt.Errorf("user_id不能为空")
	}
	songID := decodeSongID(req.SongID)
	if strings.TrimSpace(songID) == "" {
		return fmt.Errorf("song_id不能为空")
	}
	eventType := normalizeEventType(req.EventType)
	if eventType == "" {
		return fmt.Errorf("event_type不合法")
	}

	eventAt := time.Now()
	if raw := strings.TrimSpace(req.EventAt); raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			eventAt = parsed
		}
	}

	rec := model.FeedbackRecord{
		UserID:       userID,
		SongID:       songID,
		EventType:    eventType,
		PlayMS:       maxInt64(req.PlayMS, 0),
		DurationMS:   maxInt64(req.DurationMS, 0),
		Scene:        normalizeScene(req.Scene),
		RequestID:    strings.TrimSpace(req.RequestID),
		ModelVersion: strings.TrimSpace(req.ModelVersion),
		EventAt:      eventAt,
	}
	return s.repo.InsertFeedback(ctx, rec)
}

func (s *RecommendService) TriggerRetrain(ctx context.Context, req model.TrainRequest, triggerBy string) (*model.TrainAccepted, error) {
	modelName := strings.TrimSpace(req.ModelName)
	if modelName == "" {
		modelName = defaultModelName
	}
	taskID, _, err := s.repo.TriggerRetrain(ctx, modelName, req.ForceFull, triggerBy)
	if err != nil {
		return nil, err
	}
	return &model.TrainAccepted{
		TaskID: taskID,
		Status: "queued",
	}, nil
}

func (s *RecommendService) GetModelStatus(ctx context.Context, modelName string) (*model.ModelStatus, error) {
	return s.repo.GetModelStatus(ctx, modelName)
}

func (s *RecommendService) toRecommendationItem(c model.SongCandidate, score float64, source, reason string) model.RecommendationItem {
	score = math.Round(score*10000) / 10000
	if score < 0 {
		score = 0
	}
	if score > 1.5 {
		score = 1.5
	}

	title := strings.TrimSpace(c.Title)
	if title == "" {
		title = guessTitleFromPath(c.Path)
	}
	artist := normalizeArtist(c.Artist)
	album := strings.TrimSpace(c.Album)

	streamURL := fmt.Sprintf("%s/uploads/%s", s.baseURL, encodePath(c.Path))
	var coverURL *string
	if strings.TrimSpace(c.CoverArtPath) != "" {
		v := fmt.Sprintf("%s/uploads/%s", s.baseURL, encodePath(c.CoverArtPath))
		coverURL = &v
	}
	var lrcURL *string
	if strings.TrimSpace(c.LrcPath) != "" {
		v := fmt.Sprintf("%s/uploads/%s", s.baseURL, encodePath(c.LrcPath))
		lrcURL = &v
	}

	return model.RecommendationItem{
		SongID:      c.Path,
		Path:        c.Path,
		Title:       title,
		Artist:      artist,
		Album:       album,
		DurationSec: c.DurationSec,
		CoverArtURL: coverURL,
		StreamURL:   streamURL,
		LrcURL:      lrcURL,
		Score:       score,
		Reason:      reason,
		Source:      source,
	}
}

func resolveReason(cfScore, contentScore, hotScore, adjust float64) (string, string) {
	if adjust < -0.5 {
		return "hybrid", "avoid_recent_dislike"
	}
	if cfScore <= 0 && contentScore <= 0 && hotScore <= 0 {
		return "hot", "trending_now"
	}
	if cfScore >= contentScore && cfScore >= hotScore {
		return "cf", "based_on_play_history"
	}
	if contentScore >= hotScore {
		return "content", "similar_artist_preference"
	}
	return "hot", "trending_now"
}

func normalize(v, maxV float64) float64 {
	if maxV <= 0 || v <= 0 {
		return 0
	}
	out := v / maxV
	if out < 0 {
		return 0
	}
	if out > 1 {
		return 1
	}
	return out
}

func maxMapValue(m map[string]float64) float64 {
	maxV := 0.0
	for _, v := range m {
		if v > maxV {
			maxV = v
		}
	}
	return maxV
}

func normalizeLimit(v, d, maxV int) int {
	if v <= 0 {
		v = d
	}
	if v > maxV {
		v = maxV
	}
	return v
}

func normalizeScene(scene string) string {
	switch strings.ToLower(strings.TrimSpace(scene)) {
	case "radio":
		return "radio"
	case "detail":
		return "detail"
	default:
		return "home"
	}
}

func normalizeEventType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "impression", "click", "play", "finish", "like", "skip", "share", "dislike":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return ""
	}
}

func normalizeArtist(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return "未知歌手"
	}
	return s
}

func normalizeAlbum(v string) string {
	return strings.TrimSpace(strings.ToLower(v))
}

func hasCommonToken(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	return strings.Contains(a, b) || strings.Contains(b, a)
}

func decodeSongID(v string) string {
	raw := strings.TrimSpace(v)
	if raw == "" {
		return ""
	}
	if decoded, err := url.PathUnescape(raw); err == nil {
		raw = decoded
	}
	return strings.TrimSpace(raw)
}

func encodePath(p string) string {
	raw := strings.TrimSpace(p)
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func guessTitleFromPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	parts := strings.Split(p, "/")
	last := parts[len(parts)-1]
	last = strings.TrimSpace(last)
	if dot := strings.LastIndex(last, "."); dot > 0 {
		last = last[:dot]
	}
	return last
}

func newRequestID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("rec_%d", time.Now().UnixNano())
	}
	return "rec_" + hex.EncodeToString(buf)
}

func maxInt64(v, d int64) int64 {
	if v < d {
		return d
	}
	return v
}
