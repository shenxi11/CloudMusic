package service

import (
	"context"
	"fmt"
	"strings"

	"music-platform/internal/common/eventbus"
	"music-platform/internal/common/logger"
	"music-platform/internal/common/outbox"
	"music-platform/internal/music/compat"
	"music-platform/internal/music/external"
	"music-platform/internal/playlist/model"
)

type PlaylistService struct {
	repo           playlistRepository
	baseURL        string
	publisher      eventbus.Publisher
	outbox         *outbox.Store
	jamendoService external.JamendoService
}

type playlistRepository interface {
	CreatePlaylist(userAccount string, req model.CreatePlaylistRequest) (int64, error)
	ListPlaylists(userAccount string, page, pageSize int) ([]model.Playlist, int, error)
	GetPlaylistDetail(userAccount string, playlistID int64) (*model.Playlist, []model.PlaylistItemRecord, error)
	UpdatePlaylist(userAccount string, playlistID int64, req model.UpdatePlaylistRequest) error
	DeletePlaylist(userAccount string, playlistID int64) error
	AddPlaylistItems(userAccount string, playlistID int64, items []model.PlaylistTrackInput) (int64, int64, error)
	RemovePlaylistItems(userAccount string, playlistID int64, musicPaths []string) (int64, error)
	ReorderPlaylistItems(userAccount string, playlistID int64, items []model.PlaylistReorderItem) error
}

func NewPlaylistService(repo playlistRepository, baseURL string, publisher eventbus.Publisher, outboxStore *outbox.Store, jamendoService external.JamendoService) *PlaylistService {
	return &PlaylistService{
		repo:           repo,
		baseURL:        strings.TrimSuffix(baseURL, "/"),
		publisher:      publisher,
		outbox:         outboxStore,
		jamendoService: jamendoService,
	}
}

func (s *PlaylistService) CreatePlaylist(userAccount string, req model.CreatePlaylistRequest) (int64, error) {
	if strings.TrimSpace(userAccount) == "" {
		return 0, fmt.Errorf("用户账号不能为空")
	}
	if strings.TrimSpace(req.Name) == "" {
		return 0, fmt.Errorf("歌单名称不能为空")
	}

	id, err := s.repo.CreatePlaylist(userAccount, req)
	if err != nil {
		return 0, err
	}

	s.publishEvent("user.playlist.created", map[string]interface{}{
		"user_account": userAccount,
		"playlist_id":  id,
		"name":         req.Name,
	})
	return id, nil
}

func (s *PlaylistService) ListPlaylists(userAccount string, page, pageSize int) (*model.PlaylistListResponse, error) {
	if strings.TrimSpace(userAccount) == "" {
		return nil, fmt.Errorf("用户账号不能为空")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	playlists, total, err := s.repo.ListPlaylists(userAccount, page, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]model.PlaylistSummary, 0, len(playlists))
	for _, playlist := range playlists {
		items = append(items, model.PlaylistSummary{
			ID:               playlist.ID,
			Name:             playlist.Name,
			Description:      playlist.Description,
			CoverURL:         s.buildAssetURL(playlist.CoverPath),
			TrackCount:       playlist.TrackCount,
			TotalDurationSec: playlist.TotalDurationSec,
			CreatedAt:        formatTime(playlist.CreatedAt),
			UpdatedAt:        formatTime(playlist.UpdatedAt),
		})
	}

	return &model.PlaylistListResponse{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *PlaylistService) GetPlaylistDetail(userAccount string, playlistID int64) (*model.PlaylistDetail, error) {
	if strings.TrimSpace(userAccount) == "" {
		return nil, fmt.Errorf("用户账号不能为空")
	}
	if playlistID <= 0 {
		return nil, fmt.Errorf("歌单ID不能为空")
	}

	playlist, items, err := s.repo.GetPlaylistDetail(userAccount, playlistID)
	if err != nil {
		return nil, err
	}

	result := &model.PlaylistDetail{
		ID:               playlist.ID,
		Name:             playlist.Name,
		Description:      playlist.Description,
		CoverURL:         s.buildAssetURL(playlist.CoverPath),
		TrackCount:       playlist.TrackCount,
		TotalDurationSec: playlist.TotalDurationSec,
		CreatedAt:        formatTime(playlist.CreatedAt),
		UpdatedAt:        formatTime(playlist.UpdatedAt),
		Items:            make([]model.PlaylistItem, 0, len(items)),
	}
	for _, item := range items {
		coverURL := s.buildAssetURL(item.CoverArtPath)
		if coverURL == "" && !item.IsLocal {
			coverURL = s.resolveJamendoCoverURL(context.Background(), item.MusicPath)
		}
		result.Items = append(result.Items, model.PlaylistItem{
			ID:          item.ID,
			Position:    item.Position,
			MusicPath:   item.MusicPath,
			MusicTitle:  item.MusicTitle,
			Artist:      item.Artist,
			Album:       item.Album,
			DurationSec: item.DurationSec,
			IsLocal:     item.IsLocal,
			CoverArtURL: coverURL,
			AddedAt:     formatTime(item.CreatedAt),
		})
	}
	return result, nil
}

func (s *PlaylistService) UpdatePlaylist(userAccount string, playlistID int64, req model.UpdatePlaylistRequest) error {
	if strings.TrimSpace(userAccount) == "" {
		return fmt.Errorf("用户账号不能为空")
	}
	if playlistID <= 0 {
		return fmt.Errorf("歌单ID不能为空")
	}
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("歌单名称不能为空")
	}

	if err := s.repo.UpdatePlaylist(userAccount, playlistID, req); err != nil {
		return err
	}
	s.publishEvent("user.playlist.updated", map[string]interface{}{
		"user_account": userAccount,
		"playlist_id":  playlistID,
	})
	return nil
}

func (s *PlaylistService) DeletePlaylist(userAccount string, playlistID int64) error {
	if strings.TrimSpace(userAccount) == "" {
		return fmt.Errorf("用户账号不能为空")
	}
	if playlistID <= 0 {
		return fmt.Errorf("歌单ID不能为空")
	}

	if err := s.repo.DeletePlaylist(userAccount, playlistID); err != nil {
		return err
	}
	s.publishEvent("user.playlist.deleted", map[string]interface{}{
		"user_account": userAccount,
		"playlist_id":  playlistID,
	})
	return nil
}

func (s *PlaylistService) AddPlaylistItems(userAccount string, playlistID int64, req model.AddPlaylistItemsRequest) (int64, int64, error) {
	if strings.TrimSpace(userAccount) == "" {
		return 0, 0, fmt.Errorf("用户账号不能为空")
	}
	if playlistID <= 0 {
		return 0, 0, fmt.Errorf("歌单ID不能为空")
	}
	if len(req.Items) == 0 {
		return 0, 0, fmt.Errorf("歌曲列表不能为空")
	}

	added, skipped, err := s.repo.AddPlaylistItems(userAccount, playlistID, req.Items)
	if err != nil {
		return 0, 0, err
	}

	s.publishEvent("user.playlist.item_added", map[string]interface{}{
		"user_account": userAccount,
		"playlist_id":  playlistID,
		"added_count":  added,
		"skipped":      skipped,
	})
	return added, skipped, nil
}

func (s *PlaylistService) RemovePlaylistItems(userAccount string, playlistID int64, req model.RemovePlaylistItemsRequest) (int64, error) {
	if strings.TrimSpace(userAccount) == "" {
		return 0, fmt.Errorf("用户账号不能为空")
	}
	if playlistID <= 0 {
		return 0, fmt.Errorf("歌单ID不能为空")
	}
	if len(req.MusicPaths) == 0 {
		return 0, fmt.Errorf("音乐路径列表不能为空")
	}

	deleted, err := s.repo.RemovePlaylistItems(userAccount, playlistID, req.MusicPaths)
	if err != nil {
		return 0, err
	}

	s.publishEvent("user.playlist.item_removed", map[string]interface{}{
		"user_account": userAccount,
		"playlist_id":  playlistID,
		"deleted":      deleted,
	})
	return deleted, nil
}

func (s *PlaylistService) ReorderPlaylistItems(userAccount string, playlistID int64, req model.ReorderPlaylistItemsRequest) error {
	if strings.TrimSpace(userAccount) == "" {
		return fmt.Errorf("用户账号不能为空")
	}
	if playlistID <= 0 {
		return fmt.Errorf("歌单ID不能为空")
	}
	if len(req.Items) == 0 {
		return fmt.Errorf("排序项不能为空")
	}

	if err := s.repo.ReorderPlaylistItems(userAccount, playlistID, req.Items); err != nil {
		return err
	}
	s.publishEvent("user.playlist.reordered", map[string]interface{}{
		"user_account": userAccount,
		"playlist_id":  playlistID,
		"item_count":   len(req.Items),
	})
	return nil
}

func (s *PlaylistService) buildAssetURL(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "/uploads/") {
		return s.baseURL + trimmed
	}
	return s.baseURL + "/uploads/" + strings.TrimPrefix(trimmed, "/")
}

func (s *PlaylistService) resolveJamendoCoverURL(ctx context.Context, musicPath string) string {
	if s.jamendoService == nil || !s.jamendoService.IsConfigured() {
		return ""
	}

	sourceID, ok := compat.ParseJamendoSourceID(musicPath)
	if !ok {
		return ""
	}

	track, err := s.jamendoService.GetTrack(ctx, sourceID)
	if err != nil {
		logger.Warn("Jamendo playlist cover lookup failed for path %q: %v", musicPath, err)
		return ""
	}
	return strings.TrimSpace(track.CoverArtURL)
}

func formatTime(t interface{ Format(string) string }) string {
	return t.Format("2006-01-02 15:04:05")
}

func (s *PlaylistService) publishEvent(eventType string, payload interface{}) {
	if s.publisher == nil && s.outbox == nil {
		return
	}

	evt, err := eventbus.NewEvent(eventType, "profile-service", payload)
	if err != nil {
		logger.Warn("创建歌单领域事件失败: %v", err)
		return
	}

	if s.publisher != nil {
		if err := s.publisher.Publish(context.Background(), evt); err == nil {
			return
		} else {
			logger.Warn("发布歌单领域事件失败，将写入 outbox: %v", err)
			s.enqueueOutbox(evt, err.Error())
			return
		}
	}

	s.enqueueOutbox(evt, "publisher_unavailable")
}

func (s *PlaylistService) enqueueOutbox(evt *eventbus.Event, reason string) {
	if s.outbox == nil || evt == nil {
		return
	}
	if err := s.outbox.SavePending(evt, reason); err != nil {
		logger.Warn("写入歌单 outbox 失败: %v", err)
	}
}
