package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"music-platform/internal/video/model"
)

// VideoService 视频服务接口
type VideoService interface {
	GetVideoList(ctx context.Context) ([]*model.VideoFile, error)
	GetVideoStreamURL(ctx context.Context, videoPath string, baseURL string) (string, error)
}

type videoService struct {
	videoDir string // 视频目录路径
}

// NewVideoService 创建视频服务
func NewVideoService(videoDir string) VideoService {
	return &videoService{
		videoDir: videoDir,
	}
}

// GetVideoList 获取视频列表
func (s *videoService) GetVideoList(ctx context.Context) ([]*model.VideoFile, error) {
	var videos []*model.VideoFile

	// 遍历视频目录
	err := filepath.Walk(s.videoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 只处理 mp4 文件
		if strings.ToLower(filepath.Ext(path)) != ".mp4" {
			return nil
		}

		// 获取相对路径
		relPath, err := filepath.Rel(s.videoDir, path)
		if err != nil {
			return err
		}

		// 获取文件名（不含扩展名）
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

		videos = append(videos, &model.VideoFile{
			Name: name,
			Path: relPath,
			Size: info.Size(),
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("扫描视频目录失败: %w", err)
	}

	return videos, nil
}

// GetVideoStreamURL 获取视频流URL
func (s *videoService) GetVideoStreamURL(ctx context.Context, videoPath string, baseURL string) (string, error) {
	// 验证路径安全性（防止路径遍历攻击）
	cleanPath := filepath.Clean(videoPath)
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("非法的视频路径")
	}

	// 检查文件是否存在
	fullPath := filepath.Join(s.videoDir, cleanPath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", fmt.Errorf("视频文件不存在: %s", videoPath)
	}

	// 生成流媒体URL（baseURL 已包含协议）
	streamURL := fmt.Sprintf("%s/video/%s", baseURL, videoPath)
	return streamURL, nil
}
