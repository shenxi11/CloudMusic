package handler

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"music-platform/internal/common/logger"
	"music-platform/pkg/response"
)

const (
	localPlaybackVersionSalt  = "hls-v1"
	localPlaybackSegmentTime  = 2
	localPlaybackAudioBitrate = "192k"
)

func hasSupportedLocalPlaybackExtension(pathLower string) bool {
	return strings.HasSuffix(pathLower, ".mp3") ||
		strings.HasSuffix(pathLower, ".flac") ||
		strings.HasSuffix(pathLower, ".wav") ||
		strings.HasSuffix(pathLower, ".ogg") ||
		strings.HasSuffix(pathLower, ".m4a") ||
		strings.HasSuffix(pathLower, ".aac")
}

type localPlaybackInfoResponse struct {
	MusicPath      string `json:"music_path"`
	DurationMs     int64  `json:"duration_ms"`
	DirectURL      string `json:"direct_url"`
	HLSSupported   bool   `json:"hls_supported"`
	HLSManifestURL string `json:"hls_manifest_url"`
	Version        string `json:"version"`
}

func (h *MediaHandler) LocalPlaybackInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持GET请求")
		return
	}

	musicPath, absPath, err := h.resolveSeekIndexMusicPath(r.URL.Query().Get("music_path"))
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	result, err := h.buildLocalPlaybackInfo(r.Context(), r, musicPath, absPath)
	if err != nil {
		logger.Warn("local playback-info failed music_path=%s err=%v", musicPath, err)
		response.InternalServerError(w, "生成本地音乐播放信息失败")
		return
	}

	logger.Info("local playback-info music_path=%s hls_supported=%t version=%s manifest=%s",
		result.MusicPath,
		result.HLSSupported,
		result.Version,
		result.HLSManifestURL,
	)
	response.JSON(w, http.StatusOK, result)
}

func (h *MediaHandler) buildLocalPlaybackInfo(ctx context.Context, r *http.Request, musicPath, absPath string) (localPlaybackInfoResponse, error) {
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		return localPlaybackInfoResponse{}, fmt.Errorf("source file missing: %w", err)
	}

	directURL := h.absoluteMediaURL(r, "/uploads/"+filepath.ToSlash(musicPath))
	version := buildLocalPlaybackVersion(musicPath, info)
	result := localPlaybackInfoResponse{
		MusicPath:  musicPath,
		DirectURL:  directURL,
		Version:    version,
		DurationMs: 0,
	}

	seekIndex, _, _ := h.buildLocalSeekIndex(ctx, musicPath, absPath)
	if seekIndex.DurationMs > 0 {
		result.DurationMs = seekIndex.DurationMs
	}

	if !h.canGenerateLocalHLS(musicPath) {
		return result, nil
	}

	manifestURL, err := h.ensureLocalHLS(ctx, r, musicPath, absPath, version)
	if err != nil {
		logger.Warn("local hls generation failed music_path=%s err=%v", musicPath, err)
		return result, nil
	}

	result.HLSSupported = true
	result.HLSManifestURL = manifestURL
	return result, nil
}

func (h *MediaHandler) canGenerateLocalHLS(musicPath string) bool {
	lower := strings.ToLower(musicPath)
	return hasSupportedLocalPlaybackExtension(lower)
}

func (h *MediaHandler) ensureLocalHLS(ctx context.Context, r *http.Request, musicPath, absPath, version string) (string, error) {
	cacheKey := h.localHLSCacheKey(musicPath, version)
	relativeDir := filepath.ToSlash(filepath.Join("local", cacheKey))
	absoluteDir := filepath.Join(h.hlsRoot(), filepath.FromSlash(relativeDir))
	playlistPath := filepath.Join(absoluteDir, "index.m3u8")
	manifestURL := h.absoluteMediaURL(r, "/hls/"+relativeDir+"/index.m3u8")

	if _, err := os.Stat(playlistPath); err == nil {
		return manifestURL, nil
	}

	if err := os.MkdirAll(absoluteDir, 0o755); err != nil {
		return "", fmt.Errorf("create hls directory: %w", err)
	}

	_ = os.Remove(filepath.Join(absoluteDir, "index.m3u8"))
	_ = os.Remove(filepath.Join(absoluteDir, "init.mp4"))
	if matches, _ := filepath.Glob(filepath.Join(absoluteDir, "seg_*.m4s")); len(matches) > 0 {
		for _, match := range matches {
			_ = os.Remove(match)
		}
	}

	ffmpegCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	segmentPattern := filepath.Join(absoluteDir, "seg_%05d.m4s")
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", absPath,
		"-vn",
		"-c:a", "aac",
		"-b:a", localPlaybackAudioBitrate,
		"-ar", "44100",
		"-ac", "2",
		"-f", "hls",
		"-hls_time", strconv.Itoa(localPlaybackSegmentTime),
		"-hls_playlist_type", "vod",
		"-hls_list_size", "0",
		"-hls_flags", "independent_segments+temp_file",
		"-hls_segment_type", "fmp4",
		"-hls_fmp4_init_filename", "init.mp4",
		"-hls_segment_filename", segmentPattern,
		playlistPath,
	}
	cmd := exec.CommandContext(ffmpegCtx, h.ffmpegPath(), args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg hls convert failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if _, err := os.Stat(playlistPath); err != nil {
		return "", fmt.Errorf("playlist missing after ffmpeg: %w", err)
	}

	logger.Info("local hls generated music_path=%s manifest=%s", musicPath, manifestURL)
	return manifestURL, nil
}

func (h *MediaHandler) localHLSCacheKey(musicPath, version string) string {
	sum := sha1.Sum([]byte(musicPath + "@" + version))
	return hex.EncodeToString(sum[:10])
}

func (h *MediaHandler) hlsRoot() string {
	root := strings.TrimSpace(h.hlsDir)
	if root == "" {
		root = deriveLocalHLSRoot(h.uploadDir)
	}
	return filepath.Clean(root)
}

func (h *MediaHandler) ffmpegPath() string {
	binary := strings.TrimSpace(h.ffmpegBinary)
	if binary == "" {
		return "ffmpeg"
	}
	return binary
}

func (h *MediaHandler) absoluteMediaURL(r *http.Request, path string) string {
	if r == nil {
		return path
	}
	scheme := "http"
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		scheme = forwarded
	} else if r.TLS != nil {
		scheme = "https"
	}
	if host := strings.TrimSpace(r.Host); host != "" {
		return scheme + "://" + host + path
	}
	return path
}

func buildLocalPlaybackVersion(musicPath string, info os.FileInfo) string {
	payload := fmt.Sprintf("%s|%d|%d|%s", musicPath, info.Size(), info.ModTime().UnixMilli(), localPlaybackVersionSalt)
	sum := sha1.Sum([]byte(payload))
	return fmt.Sprintf("%d-%d-%s", info.ModTime().Unix(), info.Size(), hex.EncodeToString(sum[:6]))
}

func deriveLocalHLSRoot(uploadDir string) string {
	clean := filepath.Clean(strings.TrimSpace(uploadDir))
	if clean == "" || clean == "." {
		return filepath.Clean("./uploads_hls")
	}
	base := filepath.Base(clean)
	parent := filepath.Dir(clean)
	if base == "uploads" {
		return filepath.Join(parent, "uploads_hls")
	}
	return clean + "_hls"
}
