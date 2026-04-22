package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCanGenerateLocalHLSOnlyForMp3AacM4a(t *testing.T) {
	h := NewMediaHandler(t.TempDir(), nil, "music_media", "music_users", nil)

	cases := []struct {
		musicPath string
		want      bool
	}{
		{musicPath: "artist/song.mp3", want: true},
		{musicPath: "artist/song.aac", want: true},
		{musicPath: "artist/song.m4a", want: true},
		{musicPath: "artist/song.flac", want: false},
		{musicPath: "artist/song.wav", want: false},
		{musicPath: "artist/song.ogg", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.musicPath, func(t *testing.T) {
			if got := h.canGenerateLocalHLS(tc.musicPath); got != tc.want {
				t.Fatalf("expected %s support=%t, got %t", tc.musicPath, tc.want, got)
			}
		})
	}
}

func TestBuildLocalPlaybackInfoReturnsExistingReadyManifestForMp3(t *testing.T) {
	uploadDir := filepath.Join(t.TempDir(), "uploads")
	songPath := writePlaybackInfoTestSong(t, uploadDir, "artist/song.mp3")

	h := NewMediaHandler(uploadDir, nil, "music_media", "music_users", nil)
	manifestURL := createReadyLocalHLSManifest(t, h, "artist/song.mp3", songPath)

	req := httptest.NewRequest(http.MethodGet, "http://music.example/music/local/playback-info", nil)
	resp, err := h.buildLocalPlaybackInfo(context.Background(), req, "artist/song.mp3", songPath)
	if err != nil {
		t.Fatalf("buildLocalPlaybackInfo returned error: %v", err)
	}
	if !resp.HLSSupported {
		t.Fatalf("expected hls_supported=true, got false: %+v", resp)
	}
	if resp.HLSManifestURL != manifestURL {
		t.Fatalf("expected manifest url %q, got %q", manifestURL, resp.HLSManifestURL)
	}
}

func TestBuildLocalPlaybackInfoKeepsHLSDisabledWhenManifestNotReady(t *testing.T) {
	uploadDir := filepath.Join(t.TempDir(), "uploads")
	songPath := writePlaybackInfoTestSong(t, uploadDir, "artist/song.mp3")

	h := NewMediaHandler(uploadDir, nil, "music_media", "music_users", nil)
	h.ffmpegBinary = filepath.Join(t.TempDir(), "missing-ffmpeg")
	createBrokenLocalHLSManifest(t, h, "artist/song.mp3", songPath)

	req := httptest.NewRequest(http.MethodGet, "http://music.example/music/local/playback-info", nil)
	resp, err := h.buildLocalPlaybackInfo(context.Background(), req, "artist/song.mp3", songPath)
	if err != nil {
		t.Fatalf("buildLocalPlaybackInfo returned error: %v", err)
	}
	if resp.HLSSupported {
		t.Fatalf("expected hls_supported=false when manifest is not ready: %+v", resp)
	}
	if resp.HLSManifestURL != "" {
		t.Fatalf("expected empty manifest url when manifest is not ready, got %q", resp.HLSManifestURL)
	}
}

func writePlaybackInfoTestSong(t *testing.T, uploadDir, musicPath string) string {
	t.Helper()
	absPath := filepath.Join(uploadDir, filepath.FromSlash(musicPath))
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		t.Fatalf("mkdir song dir: %v", err)
	}
	if err := os.WriteFile(absPath, []byte("dummy-mp3-data"), 0o644); err != nil {
		t.Fatalf("write song file: %v", err)
	}
	return absPath
}

func createReadyLocalHLSManifest(t *testing.T, h *MediaHandler, musicPath, songPath string) string {
	t.Helper()
	info, err := os.Stat(songPath)
	if err != nil {
		t.Fatalf("stat song file: %v", err)
	}
	version := buildLocalPlaybackVersion(musicPath, info)
	cacheKey := h.localHLSCacheKey(musicPath, version)
	relativeDir := filepath.Join("local", cacheKey)
	absoluteDir := filepath.Join(h.hlsRoot(), relativeDir)
	if err := os.MkdirAll(absoluteDir, 0o755); err != nil {
		t.Fatalf("mkdir hls dir: %v", err)
	}
	playlist := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:2
#EXT-X-PLAYLIST-TYPE:VOD
#EXT-X-MAP:URI="init.mp4"
#EXTINF:2.0,
seg_00000.m4s
#EXT-X-ENDLIST
`
	if err := os.WriteFile(filepath.Join(absoluteDir, "index.m3u8"), []byte(playlist), 0o644); err != nil {
		t.Fatalf("write playlist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(absoluteDir, "init.mp4"), []byte("init"), 0o644); err != nil {
		t.Fatalf("write init segment: %v", err)
	}
	if err := os.WriteFile(filepath.Join(absoluteDir, "seg_00000.m4s"), []byte("segment"), 0o644); err != nil {
		t.Fatalf("write media segment: %v", err)
	}
	return "http://music.example/hls/" + filepath.ToSlash(relativeDir) + "/index.m3u8"
}

func createBrokenLocalHLSManifest(t *testing.T, h *MediaHandler, musicPath, songPath string) {
	t.Helper()
	info, err := os.Stat(songPath)
	if err != nil {
		t.Fatalf("stat song file: %v", err)
	}
	version := buildLocalPlaybackVersion(musicPath, info)
	cacheKey := h.localHLSCacheKey(musicPath, version)
	absoluteDir := filepath.Join(h.hlsRoot(), "local", cacheKey)
	if err := os.MkdirAll(absoluteDir, 0o755); err != nil {
		t.Fatalf("mkdir broken hls dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(absoluteDir, "index.m3u8"), []byte("broken-playlist"), 0o644); err != nil {
		t.Fatalf("write broken playlist: %v", err)
	}
}
