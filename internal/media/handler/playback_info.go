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
	localPlaybackVersionSalt   = "hls-v1"
	localPlaybackSegmentTime   = 2
	localPlaybackAudioBitrate  = "192k"
	localPlaybackHLSBuildLimit = 2 * time.Minute
)

func hasSupportedLocalPlaybackExtension(pathLower string) bool {
	return strings.HasSuffix(pathLower, ".mp3") ||
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

	manifestURL, ready := h.readyLocalHLSManifestURL(r, musicPath, version)
	if ready {
		result.HLSSupported = true
		result.HLSManifestURL = manifestURL
		return result, nil
	}

	h.scheduleLocalHLSBuild(musicPath, absPath, version)
	return result, nil
}

func (h *MediaHandler) canGenerateLocalHLS(musicPath string) bool {
	lower := strings.ToLower(musicPath)
	return hasSupportedLocalPlaybackExtension(lower)
}

func (h *MediaHandler) readyLocalHLSManifestURL(r *http.Request, musicPath, version string) (string, bool) {
	_, absoluteDir, playlistPath, manifestURL := h.localHLSPaths(r, musicPath, version)
	if err := ensureLocalHLSArtifactsReady(absoluteDir, playlistPath); err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("local hls manifest not ready music_path=%s playlist=%s err=%v", musicPath, playlistPath, err)
		}
		return "", false
	}
	return manifestURL, true
}

func (h *MediaHandler) localHLSPaths(r *http.Request, musicPath, version string) (cacheKey, absoluteDir, playlistPath, manifestURL string) {
	cacheKey = h.localHLSCacheKey(musicPath, version)
	relativeDir := filepath.ToSlash(filepath.Join("local", cacheKey))
	absoluteDir = filepath.Join(h.hlsRoot(), filepath.FromSlash(relativeDir))
	playlistPath = filepath.Join(absoluteDir, "index.m3u8")
	manifestURL = h.absoluteMediaURL(r, "/hls/"+relativeDir+"/index.m3u8")
	return cacheKey, absoluteDir, playlistPath, manifestURL
}

func (h *MediaHandler) scheduleLocalHLSBuild(musicPath, absPath, version string) {
	cacheKey, absoluteDir, playlistPath, _ := h.localHLSPaths(nil, musicPath, version)
	if err := ensureLocalHLSArtifactsReady(absoluteDir, playlistPath); err == nil {
		return
	}
	if !h.beginLocalHLSBuild(cacheKey) {
		return
	}

	go func() {
		defer h.finishLocalHLSBuild(cacheKey)
		ctx, cancel := context.WithTimeout(context.Background(), localPlaybackHLSBuildLimit)
		defer cancel()
		if _, err := h.ensureLocalHLS(ctx, nil, musicPath, absPath, version); err != nil {
			logger.Warn("local hls async build failed music_path=%s err=%v", musicPath, err)
			return
		}
		logger.Info("local hls async build ready music_path=%s cache_key=%s", musicPath, cacheKey)
	}()
}

func (h *MediaHandler) beginLocalHLSBuild(cacheKey string) bool {
	h.localHLSBuildMu.Lock()
	defer h.localHLSBuildMu.Unlock()
	if _, exists := h.localHLSBuildInFlight[cacheKey]; exists {
		return false
	}
	h.localHLSBuildInFlight[cacheKey] = struct{}{}
	return true
}

func (h *MediaHandler) finishLocalHLSBuild(cacheKey string) {
	h.localHLSBuildMu.Lock()
	delete(h.localHLSBuildInFlight, cacheKey)
	h.localHLSBuildMu.Unlock()
}

func (h *MediaHandler) ensureLocalHLS(ctx context.Context, r *http.Request, musicPath, absPath, version string) (string, error) {
	_, absoluteDir, playlistPath, manifestURL := h.localHLSPaths(r, musicPath, version)

	if err := ensureLocalHLSArtifactsReady(absoluteDir, playlistPath); err == nil {
		return manifestURL, nil
	} else if !os.IsNotExist(err) {
		logger.Warn("local hls manifest not ready music_path=%s playlist=%s err=%v", musicPath, playlistPath, err)
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

	ffmpegCtx, cancel := context.WithTimeout(ctx, localPlaybackHLSBuildLimit)
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
	if err := ensureLocalHLSArtifactsReady(absoluteDir, playlistPath); err != nil {
		return "", fmt.Errorf("playlist not ready after ffmpeg: %w", err)
	}

	logger.Info("local hls generated music_path=%s manifest=%s", musicPath, manifestURL)
	return manifestURL, nil
}

func ensureLocalHLSArtifactsReady(absoluteDir, playlistPath string) error {
	playlistInfo, err := os.Stat(playlistPath)
	if err != nil {
		return err
	}
	if playlistInfo.IsDir() {
		return fmt.Errorf("playlist path is directory")
	}
	if playlistInfo.Size() == 0 {
		return fmt.Errorf("playlist is empty")
	}

	playlistBytes, err := os.ReadFile(playlistPath)
	if err != nil {
		return fmt.Errorf("read playlist: %w", err)
	}
	if !strings.Contains(string(playlistBytes), "#EXTM3U") {
		return fmt.Errorf("playlist missing EXTM3U header")
	}

	initPath := filepath.Join(absoluteDir, "init.mp4")
	initInfo, err := os.Stat(initPath)
	if err != nil {
		return fmt.Errorf("init segment missing: %w", err)
	}
	if initInfo.IsDir() || initInfo.Size() == 0 {
		return fmt.Errorf("init segment not ready")
	}

	segments, err := filepath.Glob(filepath.Join(absoluteDir, "seg_*.m4s"))
	if err != nil {
		return fmt.Errorf("glob segments: %w", err)
	}
	if len(segments) == 0 {
		return fmt.Errorf("media segments missing")
	}
	return nil
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
