package service

import (
	"context"
	"fmt"
	"strings"

	"music-platform/internal/common/eventbus"
	"music-platform/internal/common/logger"
	"music-platform/internal/common/outbox"
	"music-platform/internal/usermusic/model"
	"music-platform/internal/usermusic/repository"
)

type UserMusicService struct {
	repo      *repository.UserMusicRepository
	baseURL   string
	publisher eventbus.Publisher
	outbox    *outbox.Store
}

func NewUserMusicService(repo *repository.UserMusicRepository, baseURL string, publisher eventbus.Publisher, outboxStore *outbox.Store) *UserMusicService {
	return &UserMusicService{
		repo:      repo,
		baseURL:   baseURL,
		publisher: publisher,
		outbox:    outboxStore,
	}
}

// AddFavorite 添加喜欢的音乐
func (s *UserMusicService) AddFavorite(userAccount string, req model.AddFavoriteRequest) error {
	// 参数验证
	if strings.TrimSpace(userAccount) == "" {
		return fmt.Errorf("用户账号不能为空")
	}
	if strings.TrimSpace(req.MusicPath) == "" {
		return fmt.Errorf("音乐路径不能为空")
	}

	if err := s.repo.AddFavorite(userAccount, req); err != nil {
		return err
	}

	s.publishEvent("user.favorite.added", map[string]interface{}{
		"user_account": userAccount,
		"music_path":   req.MusicPath,
		"is_local":     req.IsLocal,
	})
	return nil
}

// RemoveFavorite 移除喜欢的音乐
func (s *UserMusicService) RemoveFavorite(userAccount, musicPath string) error {
	// 参数验证
	if strings.TrimSpace(userAccount) == "" {
		return fmt.Errorf("用户账号不能为空")
	}
	if strings.TrimSpace(musicPath) == "" {
		return fmt.Errorf("音乐路径不能为空")
	}

	if err := s.repo.RemoveFavorite(userAccount, musicPath); err != nil {
		return err
	}

	s.publishEvent("user.favorite.removed", map[string]interface{}{
		"user_account": userAccount,
		"music_path":   musicPath,
	})
	return nil
}

// ListFavorites 获取喜欢的音乐列表
func (s *UserMusicService) ListFavorites(userAccount string) ([]model.MusicItem, error) {
	// 参数验证
	if strings.TrimSpace(userAccount) == "" {
		return nil, fmt.Errorf("用户账号不能为空")
	}

	favorites, err := s.repo.ListFavorites(userAccount)
	if err != nil {
		return nil, err
	}

	// 转换为MusicItem格式
	items := make([]model.MusicItem, 0, len(favorites))
	for _, fav := range favorites {
		addedAt := fav.CreatedAt.Format("2006-01-02 15:04:05")
		item := model.MusicItem{
			Path:     fav.MusicPath,
			Title:    fav.MusicTitle,
			Artist:   fav.Artist,
			Duration: formatDuration(fav.DurationSec),
			IsLocal:  fav.IsLocal,
			AddedAt:  &addedAt,
		}

		// 为在线音乐添加封面URL
		if !fav.IsLocal && fav.CoverArtPath != "" {
			coverURL := fmt.Sprintf("%s/uploads/%s", s.baseURL, fav.CoverArtPath)
			item.CoverArtURL = &coverURL
		}

		items = append(items, item)
	}
	return items, nil
}

// AddPlayHistory 添加播放历史
func (s *UserMusicService) AddPlayHistory(userAccount string, req model.AddPlayHistoryRequest) error {
	// 参数验证
	if strings.TrimSpace(userAccount) == "" {
		return fmt.Errorf("用户账号不能为空")
	}
	if strings.TrimSpace(req.MusicPath) == "" {
		return fmt.Errorf("音乐路径不能为空")
	}

	if err := s.repo.AddPlayHistory(userAccount, req); err != nil {
		return err
	}

	s.publishEvent("user.play_history.added", map[string]interface{}{
		"user_account": userAccount,
		"music_path":   req.MusicPath,
		"is_local":     req.IsLocal,
	})
	return nil
}

// ListPlayHistory 获取播放历史
func (s *UserMusicService) ListPlayHistory(userAccount string, limit int) ([]model.MusicItem, error) {
	// 参数验证
	if strings.TrimSpace(userAccount) == "" {
		return nil, fmt.Errorf("用户账号不能为空")
	}

	history, err := s.repo.ListPlayHistory(userAccount, limit)
	if err != nil {
		return nil, err
	}

	// 转换为MusicItem格式
	items := make([]model.MusicItem, 0, len(history))
	for _, h := range history {
		playTime := h.PlayTime.Format("2006-01-02 15:04:05")
		item := model.MusicItem{
			Path:     h.MusicPath,
			Title:    h.MusicTitle,
			Artist:   h.Artist,
			Album:    h.Album,
			Duration: formatDuration(h.DurationSec),
			IsLocal:  h.IsLocal,
			PlayTime: &playTime,
		}

		// 为在线音乐添加封面URL
		if !h.IsLocal && h.CoverArtPath != "" {
			coverURL := fmt.Sprintf("%s/uploads/%s", s.baseURL, h.CoverArtPath)
			item.CoverArtURL = &coverURL
		}

		items = append(items, item)
	}
	return items, nil
}

// ListPlayHistoryDistinct 获取去重的播放历史（每首歌只显示最近一次）
func (s *UserMusicService) ListPlayHistoryDistinct(userAccount string, limit int) ([]model.MusicItem, error) {
	// 参数验证
	if strings.TrimSpace(userAccount) == "" {
		return nil, fmt.Errorf("用户账号不能为空")
	}

	history, err := s.repo.ListPlayHistoryDistinct(userAccount, limit)
	if err != nil {
		return nil, err
	}

	// 转换为MusicItem格式
	items := make([]model.MusicItem, 0, len(history))
	for _, h := range history {
		playTime := h.PlayTime.Format("2006-01-02 15:04:05")
		item := model.MusicItem{
			Path:     h.MusicPath,
			Title:    h.MusicTitle,
			Artist:   h.Artist,
			Album:    h.Album,
			Duration: formatDuration(h.DurationSec),
			IsLocal:  h.IsLocal,
			PlayTime: &playTime,
		}

		// 为在线音乐添加封面URL
		if !h.IsLocal && h.CoverArtPath != "" {
			coverURL := fmt.Sprintf("%s/uploads/%s", s.baseURL, h.CoverArtPath)
			item.CoverArtURL = &coverURL
		}

		items = append(items, item)
	}
	return items, nil
}

// DeletePlayHistory 删除指定的播放历史记录（支持批量删除）
func (s *UserMusicService) DeletePlayHistory(userAccount string, musicPaths []string) (int64, error) {
	// 参数验证
	if strings.TrimSpace(userAccount) == "" {
		return 0, fmt.Errorf("用户账号不能为空")
	}
	if len(musicPaths) == 0 {
		return 0, fmt.Errorf("音乐路径列表不能为空")
	}

	deleted, err := s.repo.DeletePlayHistory(userAccount, musicPaths)
	if err != nil {
		return 0, err
	}

	s.publishEvent("user.play_history.deleted", map[string]interface{}{
		"user_account": userAccount,
		"count":        deleted,
	})
	return deleted, nil
}

// ClearPlayHistory 清空用户的全部播放历史
func (s *UserMusicService) ClearPlayHistory(userAccount string) (int64, error) {
	// 参数验证
	if strings.TrimSpace(userAccount) == "" {
		return 0, fmt.Errorf("用户账号不能为空")
	}

	deleted, err := s.repo.ClearPlayHistory(userAccount)
	if err != nil {
		return 0, err
	}

	s.publishEvent("user.play_history.cleared", map[string]interface{}{
		"user_account": userAccount,
		"count":        deleted,
	})
	return deleted, nil
}

// formatDuration 格式化时长（秒 -> mm:ss）
func formatDuration(seconds float64) string {
	if seconds <= 0 {
		return ""
	}
	totalSec := int(seconds)
	minutes := totalSec / 60
	secs := totalSec % 60
	return fmt.Sprintf("%02d:%02d", minutes, secs)
}

func (s *UserMusicService) publishEvent(eventType string, payload interface{}) {
	if s.publisher == nil && s.outbox == nil {
		return
	}

	evt, err := eventbus.NewEvent(eventType, "profile-service", payload)
	if err != nil {
		logger.Warn("创建领域事件失败: %v", err)
		return
	}

	if s.publisher != nil {
		if err := s.publisher.Publish(context.Background(), evt); err == nil {
			return
		} else {
			logger.Warn("发布领域事件失败，将写入 outbox: %v", err)
			s.enqueueOutbox(evt, err.Error())
			return
		}
	}

	s.enqueueOutbox(evt, "publisher_unavailable")
}

func (s *UserMusicService) enqueueOutbox(evt *eventbus.Event, reason string) {
	if s.outbox == nil || evt == nil {
		return
	}
	if err := s.outbox.SavePending(evt, reason); err != nil {
		logger.Warn("写入 outbox 失败: %v", err)
	}
}
