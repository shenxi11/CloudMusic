package external

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"music-platform/internal/common/config"
)

func TestJamendoClientSearchRequiresConfig(t *testing.T) {
	client := NewJamendoClient(config.JamendoExternalConfig{
		Enabled: true,
	})

	_, err := client.Search(context.Background(), "rock", 10)
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestJamendoClientSearchEmptyKeywordDoesNotCallUpstream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected upstream request: %s", r.URL.String())
	}))
	defer server.Close()

	client := NewJamendoClient(config.JamendoExternalConfig{
		Enabled:  true,
		ClientID: "test-client",
		BaseURL:  server.URL,
	})

	tracks, err := client.Search(context.Background(), "   ", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tracks) != 0 {
		t.Fatalf("expected empty result, got %d", len(tracks))
	}
}

func TestJamendoClientSearchMapsResultsAndDefaultLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if r.URL.Path != "/tracks/" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if query.Get("client_id") != "test-client" {
			t.Fatalf("unexpected client_id: %q", query.Get("client_id"))
		}
		if query.Get("search") != "mayday" {
			t.Fatalf("unexpected search: %q", query.Get("search"))
		}
		if query.Get("limit") != "7" {
			t.Fatalf("unexpected limit: %q", query.Get("limit"))
		}
		if query.Get("include") != "lyrics+musicinfo+licenses" {
			t.Fatalf("unexpected include: %q", query.Get("include"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"headers": {"status": "success", "code": 0, "results_count": 2},
			"results": [
				{
					"id": "1218138",
					"name": "28 Mayday Circus",
					"duration": "252",
					"artist_name": "Hasenchat",
					"album_name": "Mayday Album",
					"audio": "https://audio.example/track.mp3",
					"image": "",
					"album_image": "https://img.example/album.jpg",
					"lyrics": "line one",
					"license_ccurl": "http://creativecommons.org/licenses/by/3.0/",
					"shareurl": "https://www.jamendo.com/track/1218138",
					"audiodownload_allowed": true,
					"explicit": false,
					"lang": "en"
				},
				{
					"id": "no-audio",
					"name": "No Audio",
					"duration": "10",
					"artist_name": "Muted",
					"audio": ""
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewJamendoClient(config.JamendoExternalConfig{
		Enabled:      true,
		ClientID:     " test-client ",
		BaseURL:      server.URL + "/",
		DefaultLimit: 7,
	})

	tracks, err := client.Search(context.Background(), " mayday ", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected one playable track, got %d", len(tracks))
	}
	track := tracks[0]
	if track.Source != "jamendo" || track.SourceID != "1218138" || track.Title != "28 Mayday Circus" {
		t.Fatalf("unexpected mapped track: %+v", track)
	}
	if track.DurationSec != 252 || track.CoverArtURL != "https://img.example/album.jpg" {
		t.Fatalf("unexpected duration/cover mapping: %+v", track)
	}
	if !track.DownloadAllowed || track.LicenseURL == "" || track.ShareURL == "" || track.Lyrics == "" {
		t.Fatalf("missing expected metadata: %+v", track)
	}
}

func TestJamendoClientSearchCapsLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "200" {
			t.Fatalf("expected capped limit 200, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"headers":{"status":"success","code":0},"results":[]}`))
	}))
	defer server.Close()

	client := NewJamendoClient(config.JamendoExternalConfig{
		Enabled:  true,
		ClientID: "test-client",
		BaseURL:  server.URL,
	})

	if _, err := client.Search(context.Background(), "rock", 999); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJamendoClientUpstreamFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"headers": {"status": "failed", "code": 1, "error_message": "invalid client"},
			"results": []
		}`))
	}))
	defer server.Close()

	client := NewJamendoClient(config.JamendoExternalConfig{
		Enabled:  true,
		ClientID: "bad-client",
		BaseURL:  server.URL,
	})

	_, err := client.Search(context.Background(), "rock", 10)
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

func TestJamendoClientGetTrackNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("id"); got != "404" {
			t.Fatalf("unexpected id: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"headers":{"status":"success","code":0},"results":[]}`))
	}))
	defer server.Close()

	client := NewJamendoClient(config.JamendoExternalConfig{
		Enabled:  true,
		ClientID: "test-client",
		BaseURL:  server.URL,
	})

	_, err := client.GetTrack(context.Background(), "404")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
