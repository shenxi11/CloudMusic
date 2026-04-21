package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestLocalSeekIndexRejectsInvalidPaths(t *testing.T) {
	h := NewMediaHandler(t.TempDir(), nil, "music_media", "music_users", nil)

	cases := []string{
		"",
		"/abs/song.mp3",
		"../song.mp3",
		"..\\song.mp3",
		"C:\\music\\song.mp3",
		"\\server\\share\\song.mp3",
	}

	for _, musicPath := range cases {
		t.Run(musicPath, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/music/local/seek-index?music_path="+url.QueryEscape(musicPath), nil)
			rec := httptest.NewRecorder()

			h.LocalSeekIndex(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestLocalSeekIndexReturnsUnsupportedForMissingAndUnsupportedFormat(t *testing.T) {
	uploadDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(uploadDir, "artist"), 0o755); err != nil {
		t.Fatalf("mkdir upload dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(uploadDir, "artist", "song.flac"), []byte("not-supported"), 0o644); err != nil {
		t.Fatalf("write flac file: %v", err)
	}

	h := NewMediaHandler(uploadDir, nil, "music_media", "music_users", nil)

	missingReq := httptest.NewRequest(http.MethodGet, "/music/local/seek-index?music_path="+url.QueryEscape("artist/missing.mp3"), nil)
	missingRec := httptest.NewRecorder()
	h.LocalSeekIndex(missingRec, missingReq)
	missing := decodeSeekIndexResponse(t, missingRec)
	if missingRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for missing file, got %d body=%s", missingRec.Code, missingRec.Body.String())
	}
	if missing.Supported {
		t.Fatalf("expected missing file to be unsupported")
	}

	unsupportedReq := httptest.NewRequest(http.MethodGet, "/music/local/seek-index?music_path="+url.QueryEscape("artist/song.flac"), nil)
	unsupportedRec := httptest.NewRecorder()
	h.LocalSeekIndex(unsupportedRec, unsupportedReq)
	unsupported := decodeSeekIndexResponse(t, unsupportedRec)
	if unsupportedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for unsupported format, got %d body=%s", unsupportedRec.Code, unsupportedRec.Body.String())
	}
	if unsupported.Supported {
		t.Fatalf("expected flac file to be unsupported")
	}
	if unsupported.FileSize == 0 {
		t.Fatalf("expected unsupported response to include file size")
	}
	if unsupported.Version == "" {
		t.Fatalf("expected unsupported response to include version")
	}
}

func TestLocalSeekIndexCachesAcrossMemoryAndDiskAndInvalidatesOnFileChange(t *testing.T) {
	t.Setenv("FAKE_FFPROBE_COUNT_FILE", filepath.Join(t.TempDir(), "ffprobe-count.txt"))

	uploadDir := filepath.Join(t.TempDir(), "uploads")
	songDir := filepath.Join(uploadDir, "artist")
	if err := os.MkdirAll(songDir, 0o755); err != nil {
		t.Fatalf("mkdir song dir: %v", err)
	}
	songPath := filepath.Join(songDir, "song.mp3")
	if err := os.WriteFile(songPath, []byte("dummy-mp3-data"), 0o644); err != nil {
		t.Fatalf("write song file: %v", err)
	}

	ffprobePath := filepath.Join(t.TempDir(), "fake_ffprobe.sh")
	if err := os.WriteFile(ffprobePath, []byte(`#!/bin/sh
count_file="$FAKE_FFPROBE_COUNT_FILE"
count=0
if [ -f "$count_file" ]; then
  count=$(cat "$count_file")
fi
count=$((count + 1))
printf '%s' "$count" > "$count_file"
cat <<'JSON'
{"format":{"duration":"3.400"},"packets":[{"pts_time":"0.023","pos":"1024"},{"pts_time":"0.500","pos":"2048"},{"pts_time":"1.700","pos":"4096"},{"pts_time":"3.400","pos":"8192"}]}
JSON
`), 0o755); err != nil {
		t.Fatalf("write fake ffprobe: %v", err)
	}

	h := NewMediaHandler(uploadDir, nil, "music_media", "music_users", nil)
	h.ffprobeBinary = ffprobePath

	firstRec := httptest.NewRecorder()
	h.LocalSeekIndex(firstRec, httptest.NewRequest(http.MethodGet, "/music/local/seek-index?music_path=artist/song.mp3", nil))
	first := decodeSeekIndexResponse(t, firstRec)
	if !first.Supported {
		t.Fatalf("expected first request to be supported: %+v", first)
	}
	if got := readSeekIndexProbeCount(t); got != 1 {
		t.Fatalf("expected probe count 1 after first request, got %d", got)
	}
	if len(first.Points) < 3 {
		t.Fatalf("expected reduced seek points, got %+v", first.Points)
	}
	if first.Points[0] != (seekIndexPoint{TimeMs: 0, ByteOffset: 0}) {
		t.Fatalf("expected 0/0 prefix, got %+v", first.Points[0])
	}

	secondRec := httptest.NewRecorder()
	h.LocalSeekIndex(secondRec, httptest.NewRequest(http.MethodGet, "/music/local/seek-index?music_path=artist/song.mp3", nil))
	second := decodeSeekIndexResponse(t, secondRec)
	if got := readSeekIndexProbeCount(t); got != 1 {
		t.Fatalf("expected memory cache hit without re-probe, got count %d", got)
	}
	if second.Version != first.Version {
		t.Fatalf("expected memory cache response version %q, got %q", first.Version, second.Version)
	}

	hDisk := NewMediaHandler(uploadDir, nil, "music_media", "music_users", nil)
	hDisk.ffprobeBinary = ffprobePath
	diskRec := httptest.NewRecorder()
	hDisk.LocalSeekIndex(diskRec, httptest.NewRequest(http.MethodGet, "/music/local/seek-index?music_path=artist/song.mp3", nil))
	disk := decodeSeekIndexResponse(t, diskRec)
	if got := readSeekIndexProbeCount(t); got != 1 {
		t.Fatalf("expected disk cache hit without re-probe, got count %d", got)
	}
	if disk.Version != first.Version {
		t.Fatalf("expected disk cache response version %q, got %q", first.Version, disk.Version)
	}
	if _, err := os.Stat(filepath.Join(uploadDir, ".seek_index_cache", "artist", "song.mp3.json")); err != nil {
		t.Fatalf("expected disk cache file to exist: %v", err)
	}

	if err := os.WriteFile(songPath, []byte("dummy-mp3-data-updated"), 0o644); err != nil {
		t.Fatalf("update song file: %v", err)
	}
	modTime := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(songPath, modTime, modTime); err != nil {
		t.Fatalf("chtimes song file: %v", err)
	}

	hInvalidated := NewMediaHandler(uploadDir, nil, "music_media", "music_users", nil)
	hInvalidated.ffprobeBinary = ffprobePath
	updatedRec := httptest.NewRecorder()
	hInvalidated.LocalSeekIndex(updatedRec, httptest.NewRequest(http.MethodGet, "/music/local/seek-index?music_path=artist/song.mp3", nil))
	updated := decodeSeekIndexResponse(t, updatedRec)
	if got := readSeekIndexProbeCount(t); got != 2 {
		t.Fatalf("expected cache invalidation to trigger second probe, got count %d", got)
	}
	if updated.Version == first.Version {
		t.Fatalf("expected version to change after file update, old=%q new=%q", first.Version, updated.Version)
	}
}

func decodeSeekIndexResponse(t *testing.T, rec *httptest.ResponseRecorder) seekIndexResponse {
	t.Helper()
	var resp seekIndexResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	return resp
}

func readSeekIndexProbeCount(t *testing.T) int {
	t.Helper()
	countFile := os.Getenv("FAKE_FFPROBE_COUNT_FILE")
	data, err := os.ReadFile(countFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("read probe count: %v", err)
	}
	count, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("parse probe count: %v", err)
	}
	return count
}
