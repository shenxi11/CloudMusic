package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"music-platform/internal/common/logger"
	"music-platform/internal/music/compat"
	"music-platform/internal/music/external"
	"music-platform/internal/music/model"
	"music-platform/internal/music/repository"
)

// MusicService 音乐服务接口
type MusicService interface {
	GetAllMusic(ctx context.Context, baseURL string) ([]*model.FileListItem, error)
	GetMusicByPath(ctx context.Context, path string, baseURL string) (*model.MusicResponse, error)
	GetMusicByFilename(ctx context.Context, filename string, baseURL string) (*model.MusicResponse, error)
	GetMusicByFilenameWithOptions(ctx context.Context, filename string, baseURL string, options PlaybackOptions) (*model.MusicResponse, error)
	GetMusicByArtist(ctx context.Context, artist string, baseURL string) ([]*model.FileListItem, error)
	SearchMusic(ctx context.Context, keyword string, baseURL string) ([]*model.FileListItem, error)
}

// PlaybackConfig controls optional optimized playback resources.
type PlaybackConfig struct {
	TranscodedAudioDir     string
	TranscodedAudioBaseURL string
}

// PlaybackOptions controls per-request playback selection.
type PlaybackOptions struct {
	Quality string
}

type musicService struct {
	musicRepo              repository.MusicRepository
	jamendoService         external.JamendoService
	transcodedAudioDir     string
	transcodedAudioBaseURL string
}

// NewMusicService 创建音乐服务
func NewMusicService(musicRepo repository.MusicRepository, jamendoService external.JamendoService) MusicService {
	return NewMusicServiceWithPlaybackConfig(musicRepo, jamendoService, PlaybackConfig{})
}

// NewMusicServiceWithPlaybackConfig 创建带播放优化配置的音乐服务。
func NewMusicServiceWithPlaybackConfig(musicRepo repository.MusicRepository, jamendoService external.JamendoService, playback PlaybackConfig) MusicService {
	return &musicService{
		musicRepo:              musicRepo,
		jamendoService:         jamendoService,
		transcodedAudioDir:     strings.TrimSpace(playback.TranscodedAudioDir),
		transcodedAudioBaseURL: strings.TrimRight(strings.TrimSpace(playback.TranscodedAudioBaseURL), "/"),
	}
}

// GetAllMusic 获取所有音乐列表
func (s *musicService) GetAllMusic(ctx context.Context, baseURL string) ([]*model.FileListItem, error) {
	// 尝试从缓存获取
	// cacheKey := cache.PrefixMusic + "all_files"
	// if cachedResult, err := cache.Get(cacheKey); err == nil && cachedResult != "" {
	// 	// 缓存命中（实际应该反序列化）
	// }

	musicFiles, err := s.musicRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	var fileList []*model.FileListItem
	for _, mf := range musicFiles {
		item := &model.FileListItem{
			Path:     mf.Path,
			Duration: fmt.Sprintf("%.2f seconds", mf.DurationSec),
			Artist:   mf.Artist,
		}

		// 添加封面URL
		if mf.CoverArtPath != "" {
			coverURL := fmt.Sprintf("%s/uploads/%s", baseURL, mf.CoverArtPath)
			item.CoverArtURL = &coverURL
		}

		fileList = append(fileList, item)
	}

	// 缓存结果
	// cache.Set(cacheKey, fileList, cache.TTLMedium)

	return fileList, nil
}

// GetMusicByPath 根据路径获取音乐详情
func (s *musicService) GetMusicByPath(ctx context.Context, path string, baseURL string) (*model.MusicResponse, error) {
	mf, err := s.musicRepo.FindByPath(ctx, path)
	if err != nil {
		return nil, err
	}

	return s.buildMusicResponse(mf, baseURL, PlaybackOptions{}), nil
}

// GetMusicByFilename 根据文件名获取音乐详情
func (s *musicService) GetMusicByFilename(ctx context.Context, filename string, baseURL string) (*model.MusicResponse, error) {
	return s.GetMusicByFilenameWithOptions(ctx, filename, baseURL, PlaybackOptions{})
}

// GetMusicByFilenameWithOptions 根据文件名和播放参数获取音乐详情。
func (s *musicService) GetMusicByFilenameWithOptions(ctx context.Context, filename string, baseURL string, options PlaybackOptions) (*model.MusicResponse, error) {
	mf, err := s.musicRepo.FindByPathLike(ctx, filename)
	if err != nil {
		return nil, err
	}

	return s.buildMusicResponse(mf, baseURL, options), nil
}

// buildMusicResponse 构建音乐响应
func (s *musicService) buildMusicResponse(mf *model.MusicFile, baseURL string, options PlaybackOptions) *model.MusicResponse {
	response := &model.MusicResponse{
		StreamURL: s.resolveStreamURL(mf, baseURL, options),
		Duration:  &mf.DurationSec,
		Title:     mf.Title,
		Artist:    mf.Artist,
		Album:     mf.Album,
	}

	// 添加歌词URL
	if mf.LrcPath != "" {
		folderName := filepath.Dir(mf.LrcPath)
		lrcURL := fmt.Sprintf("%s/uploads/%s/lrc", baseURL, folderName)
		response.LrcURL = &lrcURL
	}

	// 添加封面URL
	if mf.CoverArtPath != "" {
		coverURL := fmt.Sprintf("%s/uploads/%s", baseURL, mf.CoverArtPath)
		response.AlbumCoverURL = &coverURL
	}

	return response
}

func (s *musicService) resolveStreamURL(mf *model.MusicFile, baseURL string, options PlaybackOptions) string {
	if rel, ok := s.resolveTranscodedAudioRelPath(mf, options); ok {
		transcodedBaseURL := s.transcodedAudioBaseURL
		if transcodedBaseURL == "" {
			transcodedBaseURL = strings.TrimRight(baseURL, "/")
		}
		return fmt.Sprintf("%s/audio-cache/%s", transcodedBaseURL, rel)
	}

	return fmt.Sprintf("%s/uploads/%s", baseURL, mf.Path)
}

func (s *musicService) resolveTranscodedAudioRelPath(mf *model.MusicFile, options PlaybackOptions) (string, bool) {
	if s.transcodedAudioDir == "" || isLosslessQuality(options.Quality) {
		return "", false
	}

	rel := cleanMediaRelPath(mf.Path)
	if rel == "" || !needsTranscodedAudio(rel) {
		return "", false
	}

	transcodedRel := replaceExtWithMP3(rel)
	transcodedPath := filepath.Join(s.transcodedAudioDir, filepath.FromSlash(transcodedRel))
	info, err := os.Stat(transcodedPath)
	if err != nil || info.IsDir() {
		return "", false
	}

	return transcodedRel, true
}

func isLosslessQuality(quality string) bool {
	switch strings.ToLower(strings.TrimSpace(quality)) {
	case "lossless", "original", "source", "flac":
		return true
	default:
		return false
	}
}

func needsTranscodedAudio(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".flac", ".dsf", ".ape", ".wav":
		return true
	default:
		return false
	}
}

func replaceExtWithMP3(rel string) string {
	ext := filepath.Ext(rel)
	if ext == "" {
		return rel + ".mp3"
	}
	return strings.TrimSuffix(rel, ext) + ".mp3"
}

func cleanMediaRelPath(path string) string {
	rel := filepath.ToSlash(strings.TrimSpace(path))
	rel = strings.TrimLeft(rel, "/")
	if rel == "" || strings.HasPrefix(rel, "../") || strings.Contains(rel, "/../") {
		return ""
	}
	return rel
}

// GetMusicByArtist 根据歌手获取音乐列表
// 返回格式与 GetAllMusic 一致
func (s *musicService) GetMusicByArtist(ctx context.Context, artist string, baseURL string) ([]*model.FileListItem, error) {
	musicFiles, err := s.musicRepo.FindByArtist(ctx, artist)
	if err != nil {
		return nil, fmt.Errorf("查询歌手音乐失败: %w", err)
	}

	// 初始化为空切片而不是 nil，确保返回 [] 而不是 null
	fileList := make([]*model.FileListItem, 0)

	for _, mf := range musicFiles {
		item := &model.FileListItem{
			Path:     mf.Path,
			Duration: fmt.Sprintf("%.2f seconds", mf.DurationSec),
			Artist:   mf.Artist,
		}

		// 添加封面URL
		if mf.CoverArtPath != "" {
			coverURL := fmt.Sprintf("%s/uploads/%s", baseURL, mf.CoverArtPath)
			item.CoverArtURL = &coverURL
		}

		fileList = append(fileList, item)
	}

	return fileList, nil
}

// SearchMusic 根据关键词搜索音乐，按相关性排序
func (s *musicService) SearchMusic(ctx context.Context, keyword string, baseURL string) ([]*model.FileListItem, error) {
	switch compat.DetectSearchPreference(keyword) {
	case compat.SearchPreferenceJamendoFirst:
		return s.searchJamendoThenLocal(ctx, keyword, baseURL)
	default:
		return s.searchLocalThenJamendo(ctx, keyword, baseURL)
	}
}

func (s *musicService) searchLocalThenJamendo(ctx context.Context, keyword string, baseURL string) ([]*model.FileListItem, error) {
	localItems, err := s.searchLocalMusic(ctx, keyword, baseURL)
	if err != nil {
		return nil, err
	}
	if len(localItems) > 0 {
		return localItems, nil
	}

	jamendoItems, err := s.searchJamendoMusic(ctx, keyword)
	if err != nil {
		logger.Warn("Jamendo fallback search failed for keyword %q: %v", keyword, err)
		return make([]*model.FileListItem, 0), nil
	}
	return jamendoItems, nil
}

func (s *musicService) searchJamendoThenLocal(ctx context.Context, keyword string, baseURL string) ([]*model.FileListItem, error) {
	jamendoItems, err := s.searchJamendoMusic(ctx, keyword)
	if err == nil && len(jamendoItems) > 0 {
		return jamendoItems, nil
	}
	if err != nil {
		logger.Warn("Jamendo primary search failed for keyword %q: %v", keyword, err)
	}

	return s.searchLocalMusic(ctx, keyword, baseURL)
}

func (s *musicService) searchLocalMusic(ctx context.Context, keyword string, baseURL string) ([]*model.FileListItem, error) {
	musicFiles, err := s.musicRepo.SearchByKeyword(ctx, keyword)
	if err != nil {
		return nil, fmt.Errorf("搜索音乐失败: %w", err)
	}

	// 初始化为空切片而不是 nil，确保返回 [] 而不是 null
	if len(musicFiles) == 0 {
		return make([]*model.FileListItem, 0), nil
	}

	// 计算相关性评分并排序
	scoredFiles := make([]*model.MusicFileWithScore, 0, len(musicFiles))
	lowerKeyword := strings.ToLower(keyword)

	for _, mf := range musicFiles {
		score := calculateRelevanceScore(mf, lowerKeyword)
		scoredFiles = append(scoredFiles, &model.MusicFileWithScore{
			MusicFile: *mf,
			Score:     score,
		})
	}

	// 按评分降序排序（评分越高越靠前）
	sort.Slice(scoredFiles, func(i, j int) bool {
		return scoredFiles[i].Score > scoredFiles[j].Score
	})

	// 转换为 FileListItem
	fileList := make([]*model.FileListItem, 0, len(scoredFiles))
	for _, scored := range scoredFiles {
		mf := scored.MusicFile
		item := &model.FileListItem{
			Path:     mf.Path,
			Duration: fmt.Sprintf("%.2f seconds", mf.DurationSec),
			Artist:   mf.Artist,
		}

		// 添加封面URL
		if mf.CoverArtPath != "" {
			coverURL := fmt.Sprintf("%s/uploads/%s", baseURL, mf.CoverArtPath)
			item.CoverArtURL = &coverURL
		}

		fileList = append(fileList, item)
	}

	return fileList, nil
}

func (s *musicService) searchJamendoMusic(ctx context.Context, keyword string) ([]*model.FileListItem, error) {
	if s.jamendoService == nil || !s.jamendoService.IsConfigured() {
		return make([]*model.FileListItem, 0), nil
	}

	tracks, err := s.jamendoService.Search(ctx, keyword, 20)
	if err != nil {
		return nil, err
	}

	items := make([]*model.FileListItem, 0, len(tracks))
	for _, track := range tracks {
		item := compat.BuildFileListItemFromExternalTrack(track)
		if item != nil {
			items = append(items, item)
		}
	}
	return items, nil
}

// calculateRelevanceScore 计算相关性评分
// 评分规则：
// - 标题完全匹配: +100
// - 标题包含关键词: +50
// - 歌手完全匹配: +80
// - 歌手包含关键词: +40
// - 专辑完全匹配: +60
// - 专辑包含关键词: +30
// - 路径包含关键词: +10
func calculateRelevanceScore(mf *model.MusicFile, lowerKeyword string) int {
	score := 0

	lowerTitle := strings.ToLower(mf.Title)
	lowerArtist := strings.ToLower(mf.Artist)
	lowerAlbum := strings.ToLower(mf.Album)
	lowerPath := strings.ToLower(mf.Path)

	// 标题匹配（权重最高）
	if lowerTitle == lowerKeyword {
		score += 100
	} else if strings.Contains(lowerTitle, lowerKeyword) {
		score += 50
	}

	// 歌手匹配
	if lowerArtist == lowerKeyword {
		score += 80
	} else if strings.Contains(lowerArtist, lowerKeyword) {
		score += 40
	}

	// 专辑匹配
	if lowerAlbum == lowerKeyword {
		score += 60
	} else if strings.Contains(lowerAlbum, lowerKeyword) {
		score += 30
	}

	// 路径匹配（权重最低）
	if strings.Contains(lowerPath, lowerKeyword) {
		score += 10
	}

	return score
}
