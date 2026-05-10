package service

/*
模块名称: video_playback_service
功能概述: 为在线视频提供直链、HLS、多清晰度播放信息，并按需异步生成 HLS VOD 资源。
对外接口: videoService.GetVideoPlaybackInfo
依赖关系: ffmpeg、ffprobe、视频源目录、视频 HLS 输出目录。
输入输出: 输入视频相对路径，输出 direct_url、master_url、variants、duration/status/version。
异常与错误: 路径非法、源文件缺失、探测失败时返回明确错误；转码失败写日志并保留直链兜底。
维护说明: 转码任务通过内存 in-flight 表去重，当前按单机虚拟机环境设计，不依赖 CDN。
*/

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"music-platform/internal/video/model"
)

const (
	videoPlaybackVersionSalt = "video-hls-v1"
	videoHLSBuildLimit       = 30 * time.Minute
	videoHLSSegmentSeconds   = 2
)

type videoProbeInfo struct {
	Width      int
	Height     int
	DurationMs int64
}

type videoQualityProfile struct {
	ID           string
	Label        string
	Height       int
	VideoBitrate string
	AudioBitrate string
	Bandwidth    int
}

var defaultVideoProfiles = []videoQualityProfile{
	{ID: "480p", Label: "流畅 480P", Height: 480, VideoBitrate: "900k", AudioBitrate: "96k", Bandwidth: 1100000},
	{ID: "720p", Label: "高清 720P", Height: 720, VideoBitrate: "2500k", AudioBitrate: "128k", Bandwidth: 2800000},
	{ID: "1080p", Label: "超清 1080P", Height: 1080, VideoBitrate: "5000k", AudioBitrate: "160k", Bandwidth: 5500000},
}

// GetVideoPlaybackInfo 返回视频播放策略，HLS 未就绪时会异步触发构建并保留直链兜底。
func (s *videoService) GetVideoPlaybackInfo(ctx context.Context, videoPath string, baseURL string) (*model.VideoPlaybackInfoResponse, error) {
	cleanPath, absPath, info, err := s.resolveVideoFile(videoPath)
	if err != nil {
		return nil, err
	}

	probe, _ := s.probeVideo(ctx, absPath)
	version := s.buildVideoVersion(cleanPath, info)
	directURL := s.absoluteVideoURL(baseURL, "/video/"+filepath.ToSlash(cleanPath))
	profiles := s.selectProfiles(probe)
	cacheKey := s.videoHLSCacheKey(cleanPath, version)
	hlsDir := filepath.Join(s.videoHLSDir, cacheKey)
	masterPath := filepath.Join(hlsDir, "master.m3u8")
	masterURL := s.absoluteVideoURL(baseURL, "/video-hls/"+cacheKey+"/master.m3u8")

	ready := s.isVideoHLSReady(hlsDir, masterPath, profiles)
	variants := make([]model.VideoPlaybackVariant, 0, len(profiles))
	for _, profile := range profiles {
		width := scaledWidth(probe.Width, probe.Height, profile.Height)
		if width <= 0 {
			width = 16 * int(math.Ceil(float64(profile.Height*16/9)/16.0))
		}
		variants = append(variants, model.VideoPlaybackVariant{
			ID:        profile.ID,
			Label:     profile.Label,
			Width:     width,
			Height:    profile.Height,
			Bandwidth: profile.Bandwidth,
			URL:       s.absoluteVideoURL(baseURL, "/video-hls/"+cacheKey+"/"+profile.ID+"/index.m3u8"),
			Ready:     ready,
		})
	}

	status := "direct"
	if ready {
		status = "ready"
	} else if s.beginVideoHLSBuild(cacheKey) {
		status = "building"
		go s.runVideoHLSBuild(cacheKey, absPath, hlsDir, masterPath, profiles, probe)
	} else {
		status = "building"
	}

	return &model.VideoPlaybackInfoResponse{
		Path:       filepath.ToSlash(cleanPath),
		DirectURL:  directURL,
		HLSReady:   ready,
		MasterURL:  masterURL,
		Variants:   variants,
		DurationMs: probe.DurationMs,
		Status:     status,
		Version:    version,
	}, nil
}

func (s *videoService) resolveVideoFile(videoPath string) (string, string, os.FileInfo, error) {
	trimmed := strings.TrimSpace(videoPath)
	if trimmed == "" {
		return "", "", nil, fmt.Errorf("视频路径不能为空")
	}
	cleanPath := filepath.Clean(filepath.FromSlash(trimmed))
	if cleanPath == "." || strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		return "", "", nil, fmt.Errorf("非法的视频路径")
	}
	if strings.ToLower(filepath.Ext(cleanPath)) != ".mp4" {
		return "", "", nil, fmt.Errorf("仅支持 MP4 视频")
	}
	fullPath := filepath.Join(s.videoDir, cleanPath)
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		return "", "", nil, fmt.Errorf("视频文件不存在: %s", videoPath)
	}
	return cleanPath, fullPath, info, nil
}

func (s *videoService) probeVideo(ctx context.Context, absPath string) (videoProbeInfo, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	args := []string{"-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height:format=duration", "-of", "json", absPath}
	output, err := exec.CommandContext(probeCtx, s.ffprobePath(), args...).Output()
	if err != nil {
		return videoProbeInfo{}, err
	}
	var raw struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(output, &raw); err != nil {
		return videoProbeInfo{}, err
	}
	probe := videoProbeInfo{}
	if len(raw.Streams) > 0 {
		probe.Width = raw.Streams[0].Width
		probe.Height = raw.Streams[0].Height
	}
	if seconds, err := strconv.ParseFloat(raw.Format.Duration, 64); err == nil && seconds > 0 {
		probe.DurationMs = int64(seconds * 1000)
	}
	return probe, nil
}

func (s *videoService) selectProfiles(probe videoProbeInfo) []videoQualityProfile {
	maxHeight := probe.Height
	if maxHeight <= 0 {
		maxHeight = 1080
	}
	profiles := make([]videoQualityProfile, 0, len(defaultVideoProfiles))
	for _, profile := range defaultVideoProfiles {
		if profile.Height <= maxHeight+32 {
			profiles = append(profiles, profile)
		}
	}
	if len(profiles) == 0 {
		profiles = append(profiles, defaultVideoProfiles[0])
	}
	return profiles
}

func (s *videoService) runVideoHLSBuild(cacheKey, absPath, hlsDir, masterPath string, profiles []videoQualityProfile, probe videoProbeInfo) {
	defer s.finishVideoHLSBuild(cacheKey)
	ctx, cancel := context.WithTimeout(context.Background(), videoHLSBuildLimit)
	defer cancel()
	if err := s.ensureVideoHLS(ctx, absPath, hlsDir, masterPath, profiles, probe); err != nil {
		log.Printf("[WARN] 视频 HLS 生成失败 path=%s err=%v", absPath, err)
		return
	}
	log.Printf("[INFO] 视频 HLS 生成完成 path=%s cache=%s", absPath, cacheKey)
}

func (s *videoService) ensureVideoHLS(ctx context.Context, absPath, hlsDir, masterPath string, profiles []videoQualityProfile, probe videoProbeInfo) error {
	if err := os.MkdirAll(hlsDir, 0o755); err != nil {
		return fmt.Errorf("创建 HLS 目录失败: %w", err)
	}
	_ = os.Remove(masterPath)
	for _, profile := range profiles {
		_ = os.RemoveAll(filepath.Join(hlsDir, profile.ID))
		if err := os.MkdirAll(filepath.Join(hlsDir, profile.ID), 0o755); err != nil {
			return fmt.Errorf("创建清晰度目录失败: %w", err)
		}
	}
	for _, profile := range profiles {
		if err := s.generateVariant(ctx, absPath, hlsDir, profile, probe); err != nil {
			return err
		}
	}
	return s.writeMasterPlaylist(masterPath, profiles, probe)
}

func (s *videoService) generateVariant(ctx context.Context, absPath, hlsDir string, profile videoQualityProfile, probe videoProbeInfo) error {
	variantDir := filepath.Join(hlsDir, profile.ID)
	playlistPath := filepath.Join(variantDir, "index.m3u8")
	segmentPattern := filepath.Join(variantDir, "seg_%05d.m4s")
	width := scaledWidth(probe.Width, probe.Height, profile.Height)
	if width <= 0 {
		width = -2
	}
	scale := fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease,scale=trunc(iw/2)*2:trunc(ih/2)*2", width, profile.Height)
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", absPath, "-map", "0:v:0", "-map", "0:a:0?", "-vf", scale, "-c:v", "libx264", "-preset", "veryfast", "-profile:v", "main", "-crf", "23", "-b:v", profile.VideoBitrate, "-maxrate", profile.VideoBitrate, "-bufsize", bitrateBuffer(profile.VideoBitrate), "-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", videoHLSSegmentSeconds), "-c:a", "aac", "-b:a", profile.AudioBitrate, "-ac", "2", "-ar", "44100", "-f", "hls", "-hls_time", strconv.Itoa(videoHLSSegmentSeconds), "-hls_playlist_type", "vod", "-hls_list_size", "0", "-hls_flags", "independent_segments+temp_file", "-hls_segment_type", "fmp4", "-hls_fmp4_init_filename", "init.mp4", "-hls_segment_filename", segmentPattern, playlistPath}
	output, err := exec.CommandContext(ctx, s.ffmpegPath(), args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("生成 %s HLS 失败: %w: %s", profile.ID, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (s *videoService) writeMasterPlaylist(masterPath string, profiles []videoQualityProfile, probe videoProbeInfo) error {
	var builder strings.Builder
	builder.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n")
	for _, profile := range profiles {
		width := scaledWidth(probe.Width, probe.Height, profile.Height)
		if width <= 0 {
			width = 16 * int(math.Ceil(float64(profile.Height*16/9)/16.0))
		}
		builder.WriteString(fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"avc1.4d401f,mp4a.40.2\"\n%s/index.m3u8\n", profile.Bandwidth, width, profile.Height, profile.ID))
	}
	return os.WriteFile(masterPath, []byte(builder.String()), 0o644)
}

func (s *videoService) isVideoHLSReady(hlsDir, masterPath string, profiles []videoQualityProfile) bool {
	if info, err := os.Stat(masterPath); err != nil || info.IsDir() || info.Size() == 0 {
		return false
	}
	for _, profile := range profiles {
		playlist := filepath.Join(hlsDir, profile.ID, "index.m3u8")
		initFile := filepath.Join(hlsDir, profile.ID, "init.mp4")
		if info, err := os.Stat(playlist); err != nil || info.IsDir() || info.Size() == 0 {
			return false
		}
		if info, err := os.Stat(initFile); err != nil || info.IsDir() || info.Size() == 0 {
			return false
		}
		segments, err := filepath.Glob(filepath.Join(hlsDir, profile.ID, "seg_*.m4s"))
		if err != nil || len(segments) == 0 {
			return false
		}
	}
	return true
}

func (s *videoService) beginVideoHLSBuild(cacheKey string) bool {
	s.videoHLSBuildMu.Lock()
	defer s.videoHLSBuildMu.Unlock()
	if _, exists := s.videoHLSBuildInFlight[cacheKey]; exists {
		return false
	}
	s.videoHLSBuildInFlight[cacheKey] = struct{}{}
	return true
}

func (s *videoService) finishVideoHLSBuild(cacheKey string) {
	s.videoHLSBuildMu.Lock()
	delete(s.videoHLSBuildInFlight, cacheKey)
	s.videoHLSBuildMu.Unlock()
}

func (s *videoService) buildVideoVersion(videoPath string, info os.FileInfo) string {
	raw := fmt.Sprintf("%s@%d@%d@%s", videoPath, info.Size(), info.ModTime().UnixNano(), videoPlaybackVersionSalt)
	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:8])
}
func (s *videoService) videoHLSCacheKey(videoPath, version string) string {
	sum := sha1.Sum([]byte(videoPath + "@" + version))
	return hex.EncodeToString(sum[:10])
}
func (s *videoService) absoluteVideoURL(baseURL, path string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if i != 0 && part != "" {
			parts[i] = url.PathEscape(part)
		}
	}
	return base + strings.Join(parts, "/")
}
func (s *videoService) ffmpegPath() string {
	if strings.TrimSpace(s.ffmpegBinary) != "" {
		return strings.TrimSpace(s.ffmpegBinary)
	}
	return "ffmpeg"
}
func (s *videoService) ffprobePath() string {
	if strings.TrimSpace(s.ffprobeBinary) != "" {
		return strings.TrimSpace(s.ffprobeBinary)
	}
	return "ffprobe"
}
func scaledWidth(sourceW, sourceH, targetH int) int {
	if sourceW <= 0 || sourceH <= 0 || targetH <= 0 {
		return 0
	}
	width := int(math.Round(float64(sourceW) * float64(targetH) / float64(sourceH)))
	if width%2 != 0 {
		width++
	}
	return width
}
func bitrateBuffer(videoBitrate string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(videoBitrate), "k")
	value, err := strconv.Atoi(trimmed)
	if err != nil || value <= 0 {
		return videoBitrate
	}
	return strconv.Itoa(value*2) + "k"
}
