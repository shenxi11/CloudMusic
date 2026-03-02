package service

import (
	"context"
	"strings"

	"music-platform/internal/artist/repository"
)

// ArtistService 歌手服务接口
type ArtistService interface {
	SearchArtist(ctx context.Context, artistName string) (bool, error)
}

type artistService struct {
	repo repository.ArtistRepository
}

// NewArtistService 创建歌手服务
func NewArtistService(repo repository.ArtistRepository) ArtistService {
	return &artistService{repo: repo}
}

// SearchArtist 搜索歌手是否存在
func (s *artistService) SearchArtist(ctx context.Context, artistName string) (bool, error) {
	// 去除首尾空格
	artistName = strings.TrimSpace(artistName)
	if artistName == "" {
		return false, nil
	}

	// 查询数据库
	exists, err := s.repo.ExistsByName(ctx, artistName)
	if err != nil {
		return false, err
	}

	return exists, nil
}
