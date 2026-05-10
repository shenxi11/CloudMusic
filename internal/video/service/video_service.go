package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"music-platform/internal/video/model"
)

// VideoService 视频服务接口
type VideoService interface {
	GetVideoList(ctx context.Context) ([]*model.VideoFile, error)
	GetVideoStreamURL(ctx context.Context, videoPath string, baseURL string) (string, error)
	GetVideoPlaybackInfo(ctx context.Context, videoPath string, baseURL string) (*model.VideoPlaybackInfoResponse, error)
}

type videoService struct {
	videoDir              string // 视频目录路径
	videoHLSDir           string
	ffmpegBinary          string
	ffprobeBinary         string
	videoHLSBuildMu       sync.Mutex
	videoHLSBuildInFlight map[string]struct{}
}

// NewVideoService 创建视频服务
func NewVideoService(videoDir string) VideoService {
	return NewVideoServiceWithPlayback(videoDir, "", "", "")
}

// NewVideoServiceWithPlayback 创建支持 HLS 播放信息的视频服务。
func NewVideoServiceWithPlayback(videoDir, videoHLSDir, ffmpegBinary, ffprobeBinary string) VideoService {
	if strings.TrimSpace(videoHLSDir) == "" {
		videoHLSDir = deriveVideoHLSDir(videoDir)
	}
	return &videoService{
		videoDir:              videoDir,
		videoHLSDir:           videoHLSDir,
		ffmpegBinary:          ffmpegBinary,
		ffprobeBinary:         ffprobeBinary,
		videoHLSBuildInFlight: make(map[string]struct{}),
	}
}

func deriveVideoHLSDir(videoDir string) string {
	clean := filepath.Clean(strings.TrimSpace(videoDir))
	if clean == "." || clean == "" {
		return filepath.Clean("./video_hls")
	}
	return clean + "_hls"
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
	cleanPath, _, _, err := s.resolveVideoFile(videoPath)
	if err != nil {
		return "", err
	}

	// 生成流媒体URL（baseURL 已包含协议）
	streamURL := fmt.Sprintf("%s/video/%s", strings.TrimRight(baseURL, "/"), filepath.ToSlash(cleanPath))
	return streamURL, nil
}
