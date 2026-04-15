package external

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"music-platform/internal/common/config"
	"music-platform/internal/music/model"
)

var (
	ErrNotConfigured = errors.New("jamendo external music source is not configured")
	ErrNotFound      = errors.New("jamendo track not found")
	ErrUpstream      = errors.New("jamendo upstream request failed")
)

type JamendoService interface {
	IsConfigured() bool
	Search(ctx context.Context, keyword string, limit int) ([]*model.ExternalMusicTrack, error)
	GetTrack(ctx context.Context, id string) (*model.ExternalMusicTrack, error)
}

type JamendoClient struct {
	enabled      bool
	clientID     string
	baseURL      string
	defaultLimit int
	httpClient   *http.Client
}

func NewJamendoClient(cfg config.JamendoExternalConfig) *JamendoClient {
	timeout := cfg.TimeoutSec
	if timeout <= 0 {
		timeout = 8
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.jamendo.com/v3.0"
	}
	defaultLimit := cfg.DefaultLimit
	if defaultLimit <= 0 {
		defaultLimit = 20
	}

	return &JamendoClient{
		enabled:      cfg.Enabled,
		clientID:     strings.TrimSpace(cfg.ClientID),
		baseURL:      baseURL,
		defaultLimit: defaultLimit,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

func (c *JamendoClient) IsConfigured() bool {
	return c != nil && c.enabled && c.clientID != ""
}

func (c *JamendoClient) Search(ctx context.Context, keyword string, limit int) ([]*model.ExternalMusicTrack, error) {
	if !c.IsConfigured() {
		return nil, ErrNotConfigured
	}
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []*model.ExternalMusicTrack{}, nil
	}

	values := c.baseParams()
	values.Set("search", keyword)
	values.Set("limit", strconv.Itoa(c.normalizeLimit(limit)))

	resp, err := c.fetchTracks(ctx, values)
	if err != nil {
		return nil, err
	}

	tracks := make([]*model.ExternalMusicTrack, 0, len(resp.Results))
	for _, item := range resp.Results {
		track := item.toExternalTrack()
		if track.StreamURL == "" {
			continue
		}
		tracks = append(tracks, track)
	}
	return tracks, nil
}

func (c *JamendoClient) GetTrack(ctx context.Context, id string) (*model.ExternalMusicTrack, error) {
	if !c.IsConfigured() {
		return nil, ErrNotConfigured
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrNotFound
	}

	values := c.baseParams()
	values.Set("id", id)
	values.Set("limit", "1")

	resp, err := c.fetchTracks(ctx, values)
	if err != nil {
		return nil, err
	}
	for _, item := range resp.Results {
		track := item.toExternalTrack()
		if track.StreamURL == "" {
			continue
		}
		return track, nil
	}
	return nil, ErrNotFound
}

func (c *JamendoClient) baseParams() url.Values {
	values := url.Values{}
	values.Set("client_id", c.clientID)
	values.Set("format", "json")
	values.Set("include", "lyrics+musicinfo+licenses")
	return values
}

func (c *JamendoClient) normalizeLimit(limit int) int {
	if limit <= 0 {
		limit = c.defaultLimit
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func (c *JamendoClient) fetchTracks(ctx context.Context, values url.Values) (*jamendoTracksResponse, error) {
	endpoint := c.baseURL + "/tracks/?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build request", ErrUpstream)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: http %d", ErrUpstream, res.StatusCode)
	}

	var payload jamendoTracksResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("%w: decode response", ErrUpstream)
	}
	if payload.Headers.Status == "failed" || payload.Headers.Code != 0 {
		msg := strings.TrimSpace(payload.Headers.ErrorMessage)
		if msg == "" {
			msg = "api returned failure"
		}
		return nil, fmt.Errorf("%w: %s", ErrUpstream, msg)
	}
	return &payload, nil
}

type jamendoTracksResponse struct {
	Headers jamendoHeaders `json:"headers"`
	Results []jamendoTrack `json:"results"`
}

type jamendoHeaders struct {
	Status       string `json:"status"`
	Code         int    `json:"code"`
	ErrorMessage string `json:"error_message"`
	ResultsCount int    `json:"results_count"`
}

type jamendoTrack struct {
	ID                   string      `json:"id"`
	Name                 string      `json:"name"`
	Duration             json.Number `json:"duration"`
	ArtistName           string      `json:"artist_name"`
	AlbumName            string      `json:"album_name"`
	Audio                string      `json:"audio"`
	Image                string      `json:"image"`
	AlbumImage           string      `json:"album_image"`
	Lyrics               string      `json:"lyrics"`
	LicenseCCURL         string      `json:"license_ccurl"`
	ShareURL             string      `json:"shareurl"`
	AudioDownloadAllowed bool        `json:"audiodownload_allowed"`
	Explicit             bool        `json:"explicit"`
	Lang                 string      `json:"lang"`
}

func (t jamendoTrack) toExternalTrack() *model.ExternalMusicTrack {
	duration, _ := t.Duration.Float64()
	cover := strings.TrimSpace(t.Image)
	if cover == "" {
		cover = strings.TrimSpace(t.AlbumImage)
	}

	return &model.ExternalMusicTrack{
		Source:          "jamendo",
		SourceID:        strings.TrimSpace(t.ID),
		Title:           strings.TrimSpace(t.Name),
		Artist:          strings.TrimSpace(t.ArtistName),
		Album:           strings.TrimSpace(t.AlbumName),
		DurationSec:     duration,
		StreamURL:       strings.TrimSpace(t.Audio),
		CoverArtURL:     cover,
		Lyrics:          strings.TrimSpace(t.Lyrics),
		LicenseURL:      strings.TrimSpace(t.LicenseCCURL),
		ShareURL:        strings.TrimSpace(t.ShareURL),
		DownloadAllowed: t.AudioDownloadAllowed,
		Explicit:        t.Explicit,
		Lang:            strings.TrimSpace(t.Lang),
	}
}
