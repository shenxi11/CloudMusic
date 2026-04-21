package handler

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"music-platform/internal/common/logger"
	"music-platform/pkg/response"
)

var seekIndexWindowsDrivePattern = regexp.MustCompile(`^[a-zA-Z]:`)

const seekIndexVersionSalt = "csv-v2"

type seekIndexPoint struct {
	TimeMs     int64 `json:"time_ms"`
	ByteOffset int64 `json:"byte_offset"`
}

type seekIndexResponse struct {
	MusicPath  string           `json:"music_path"`
	Supported  bool             `json:"supported"`
	Format     string           `json:"format"`
	DurationMs int64            `json:"duration_ms"`
	FileSize   int64            `json:"file_size"`
	Version    string           `json:"version"`
	Points     []seekIndexPoint `json:"points"`
}

type seekIndexProbePacket struct {
	Pos     string `json:"pos"`
	PtsTime string `json:"pts_time"`
}

type seekIndexCacheStore struct {
	mu    sync.RWMutex
	items map[string]seekIndexResponse
}

func newSeekIndexCacheStore() *seekIndexCacheStore {
	return &seekIndexCacheStore{items: make(map[string]seekIndexResponse)}
}

func (c *seekIndexCacheStore) Get(key string) (seekIndexResponse, bool) {
	if c == nil {
		return seekIndexResponse{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.items[key]
	if !ok {
		return seekIndexResponse{}, false
	}
	return cloneSeekIndexResponse(value), true
}

func (c *seekIndexCacheStore) Set(key string, value seekIndexResponse) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = cloneSeekIndexResponse(value)
}

func (h *MediaHandler) LocalSeekIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "仅支持GET请求")
		return
	}

	musicPath, absPath, err := h.resolveSeekIndexMusicPath(r.URL.Query().Get("music_path"))
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	result, cacheHit, generateMs := h.buildLocalSeekIndex(r.Context(), musicPath, absPath)
	logger.Info("local seek-index music_path=%s seek_index_cache_hit=%t seek_index_generate_ms=%d seek_index_point_count=%d supported=%t",
		result.MusicPath,
		cacheHit,
		generateMs,
		len(result.Points),
		result.Supported,
	)
	response.JSON(w, http.StatusOK, result)
}

func (h *MediaHandler) buildLocalSeekIndex(ctx context.Context, musicPath, absPath string) (seekIndexResponse, bool, int64) {
	startedAt := time.Now()
	format := normalizeSeekIndexFormat(filepath.Ext(musicPath))

	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		return newUnsupportedSeekIndexResponse(musicPath, format, nil, ""), false, time.Since(startedAt).Milliseconds()
	}

	version := buildSeekIndexVersion(musicPath, info)
	cacheKey := buildSeekIndexCacheKey(musicPath, info)
	base := newUnsupportedSeekIndexResponse(musicPath, format, info, version)

	if cached, ok := h.seekIndexCache.Get(cacheKey); ok {
		return cached, true, 0
	}
	if cached, ok := h.loadSeekIndexFromDisk(musicPath, version); ok {
		h.seekIndexCache.Set(cacheKey, cached)
		return cached, true, 0
	}

	result := base
	if isSeekIndexFormatSupported(format) {
		durationMs, points, probeErr := h.probeSeekIndex(ctx, absPath)
		if probeErr == nil {
			result.Supported = true
			result.DurationMs = durationMs
			result.Points = points
		} else {
			logger.Warn("seek-index ffprobe failed music_path=%s err=%v", musicPath, probeErr)
		}
	}

	h.seekIndexCache.Set(cacheKey, result)
	if err := h.writeSeekIndexToDisk(musicPath, result); err != nil {
		logger.Warn("seek-index disk cache write failed music_path=%s err=%v", musicPath, err)
	}

	return result, false, time.Since(startedAt).Milliseconds()
}

func (h *MediaHandler) resolveSeekIndexMusicPath(raw string) (string, string, error) {
	musicPath := strings.TrimSpace(raw)
	if musicPath == "" {
		return "", "", fmt.Errorf("music_path参数不能为空")
	}
	if strings.ContainsRune(musicPath, '\x00') {
		return "", "", fmt.Errorf("music_path必须是上传目录内的相对路径")
	}
	if seekIndexWindowsDrivePattern.MatchString(musicPath) || filepath.IsAbs(musicPath) || strings.HasPrefix(musicPath, "/") || strings.HasPrefix(musicPath, "\\") {
		return "", "", fmt.Errorf("music_path必须是上传目录内的相对路径")
	}

	normalized := strings.ReplaceAll(musicPath, "\\", "/")
	cleanSlash := path.Clean(normalized)
	if cleanSlash == "." || cleanSlash == "" || cleanSlash == ".." || strings.HasPrefix(cleanSlash, "../") {
		return "", "", fmt.Errorf("music_path必须是上传目录内的相对路径")
	}

	cleanNative := filepath.Clean(filepath.FromSlash(cleanSlash))
	uploadRoot, err := filepath.Abs(filepath.Clean(h.uploadDir))
	if err != nil {
		return "", "", fmt.Errorf("uploadDir解析失败: %w", err)
	}
	absPath, err := filepath.Abs(filepath.Join(uploadRoot, cleanNative))
	if err != nil {
		return "", "", fmt.Errorf("music_path解析失败: %w", err)
	}
	rel, err := filepath.Rel(uploadRoot, absPath)
	if err != nil {
		return "", "", fmt.Errorf("music_path解析失败: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("music_path必须是上传目录内的相对路径")
	}

	return filepath.ToSlash(cleanNative), absPath, nil
}

func (h *MediaHandler) probeSeekIndex(ctx context.Context, absPath string) (int64, []seekIndexPoint, error) {
	durationMs, err := h.probeSeekIndexDuration(ctx, absPath)
	if err != nil {
		return 0, nil, err
	}

	packets, err := h.probeSeekIndexPackets(ctx, absPath)
	if err != nil {
		return 0, nil, err
	}

	points, err := buildSeekIndexPoints(packets)
	if err != nil {
		return 0, nil, err
	}
	if durationMs <= 0 {
		durationMs = points[len(points)-1].TimeMs
	}
	return durationMs, points, nil
}

func (h *MediaHandler) probeSeekIndexDuration(ctx context.Context, absPath string) (int64, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		probeCtx,
		h.ffprobePath(),
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		absPath,
	)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return parseSeekIndexDurationMs(string(output)), nil
}

func (h *MediaHandler) probeSeekIndexPackets(ctx context.Context, absPath string) ([]seekIndexProbePacket, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		probeCtx,
		h.ffprobePath(),
		"-v", "quiet",
		"-select_streams", "a:0",
		"-show_entries", "packet=pts_time,pos",
		"-of", "csv=p=0",
		absPath,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	packets := make([]seekIndexProbePacket, 0, len(lines))
	for _, line := range lines {
		packet, ok := parseSeekIndexPacketLine(line)
		if !ok {
			continue
		}
		packets = append(packets, packet)
	}
	if len(packets) == 0 {
		return nil, fmt.Errorf("ffprobe未返回可用packet")
	}
	return packets, nil
}

func parseSeekIndexPacketLine(raw string) (seekIndexProbePacket, bool) {
	line := strings.TrimSpace(raw)
	if line == "" {
		return seekIndexProbePacket{}, false
	}
	parts := strings.Split(line, ",")
	if len(parts) < 2 {
		return seekIndexProbePacket{}, false
	}
	ptsTime := strings.TrimSpace(parts[0])
	pos := strings.TrimSpace(parts[1])
	if ptsTime == "" || pos == "" {
		return seekIndexProbePacket{}, false
	}
	return seekIndexProbePacket{PtsTime: ptsTime, Pos: pos}, true
}

func (h *MediaHandler) ffprobePath() string {
	binary := strings.TrimSpace(h.ffprobeBinary)
	if binary == "" {
		return "ffprobe"
	}
	return binary
}

func buildSeekIndexPoints(packets []seekIndexProbePacket) ([]seekIndexPoint, error) {
	actual := make([]seekIndexPoint, 0, len(packets))
	var firstTimeMs int64
	var hasBase bool
	var lastTimeMs int64
	var lastOffset int64

	for _, packet := range packets {
		offset, err := strconv.ParseInt(strings.TrimSpace(packet.Pos), 10, 64)
		if err != nil || offset < 0 {
			continue
		}
		timeMs, ok := parseSeekIndexTimeMs(packet.PtsTime)
		if !ok {
			continue
		}
		if !hasBase {
			firstTimeMs = timeMs
			hasBase = true
		}
		timeMs -= firstTimeMs
		if timeMs < 0 {
			timeMs = 0
		}
		if len(actual) > 0 {
			if timeMs < lastTimeMs || offset < lastOffset {
				continue
			}
			if timeMs == lastTimeMs && offset == lastOffset {
				continue
			}
		}
		actual = append(actual, seekIndexPoint{TimeMs: timeMs, ByteOffset: offset})
		lastTimeMs = timeMs
		lastOffset = offset
	}

	if len(actual) == 0 {
		return nil, fmt.Errorf("ffprobe未返回可用packet")
	}

	reduced := make([]seekIndexPoint, 0, len(actual)+1)
	reduced = append(reduced, actual[0])
	lastKeptTime := actual[0].TimeMs
	for i := 1; i < len(actual)-1; i++ {
		if actual[i].TimeMs-lastKeptTime >= 1500 {
			reduced = append(reduced, actual[i])
			lastKeptTime = actual[i].TimeMs
		}
	}
	lastPoint := actual[len(actual)-1]
	if !sameSeekIndexPoint(reduced[len(reduced)-1], lastPoint) {
		reduced = append(reduced, lastPoint)
	}
	if !sameSeekIndexPoint(reduced[0], seekIndexPoint{}) {
		reduced = append([]seekIndexPoint{{TimeMs: 0, ByteOffset: 0}}, reduced...)
	}
	return dedupeSeekIndexPoints(reduced), nil
}

func parseSeekIndexTimeMs(raw string) (int64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value < 0 {
		return 0, false
	}
	return int64(math.Round(value * 1000)), true
}

func parseSeekIndexDurationMs(raw string) int64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value < 0 {
		return 0
	}
	return int64(math.Round(value * 1000))
}

func dedupeSeekIndexPoints(points []seekIndexPoint) []seekIndexPoint {
	if len(points) == 0 {
		return []seekIndexPoint{}
	}
	result := make([]seekIndexPoint, 0, len(points))
	for _, point := range points {
		if len(result) == 0 || !sameSeekIndexPoint(result[len(result)-1], point) {
			result = append(result, point)
		}
	}
	return result
}

func sameSeekIndexPoint(a, b seekIndexPoint) bool {
	return a.TimeMs == b.TimeMs && a.ByteOffset == b.ByteOffset
}

func normalizeSeekIndexFormat(ext string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(ext)), ".")
}

func isSeekIndexFormatSupported(format string) bool {
	switch format {
	case "mp3", "aac", "m4a":
		return true
	default:
		return false
	}
}

func buildSeekIndexVersion(musicPath string, info os.FileInfo) string {
	sum := sha1.Sum([]byte(musicPath))
	return fmt.Sprintf("%d-%d-%x-%s", info.ModTime().UnixMilli(), info.Size(), sum[:6], seekIndexVersionSalt)
}

func buildSeekIndexCacheKey(musicPath string, info os.FileInfo) string {
	return fmt.Sprintf("%s|%d|%d", musicPath, info.ModTime().UnixMilli(), info.Size())
}

func newUnsupportedSeekIndexResponse(musicPath, format string, info os.FileInfo, version string) seekIndexResponse {
	resp := seekIndexResponse{
		MusicPath: musicPath,
		Supported: false,
		Format:    format,
		Version:   version,
		Points:    []seekIndexPoint{},
	}
	if info != nil {
		resp.FileSize = info.Size()
	}
	return resp
}

func cloneSeekIndexResponse(src seekIndexResponse) seekIndexResponse {
	dst := src
	if len(src.Points) == 0 {
		dst.Points = []seekIndexPoint{}
		return dst
	}
	dst.Points = append([]seekIndexPoint(nil), src.Points...)
	return dst
}

func (h *MediaHandler) seekIndexCacheFilePath(musicPath string) (string, error) {
	cacheRoot := filepath.Join(h.uploadDir, ".seek_index_cache")
	cachePath := filepath.Join(cacheRoot, filepath.FromSlash(musicPath)+".json")
	rootAbs, err := filepath.Abs(filepath.Clean(cacheRoot))
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(filepath.Clean(cachePath))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("seek-index缓存路径越界")
	}
	return pathAbs, nil
}

func (h *MediaHandler) loadSeekIndexFromDisk(musicPath, version string) (seekIndexResponse, bool) {
	cachePath, err := h.seekIndexCacheFilePath(musicPath)
	if err != nil {
		logger.Warn("seek-index cache path resolve failed music_path=%s err=%v", musicPath, err)
		return seekIndexResponse{}, false
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return seekIndexResponse{}, false
	}
	var cached seekIndexResponse
	if err := json.Unmarshal(data, &cached); err != nil {
		logger.Warn("seek-index disk cache decode failed music_path=%s err=%v", musicPath, err)
		return seekIndexResponse{}, false
	}
	if cached.Version != version {
		return seekIndexResponse{}, false
	}
	return cloneSeekIndexResponse(cached), true
}

func (h *MediaHandler) writeSeekIndexToDisk(musicPath string, result seekIndexResponse) error {
	if strings.TrimSpace(result.Version) == "" {
		return nil
	}
	cachePath, err := h.seekIndexCacheFilePath(musicPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	tmpPath := cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, cachePath)
}
