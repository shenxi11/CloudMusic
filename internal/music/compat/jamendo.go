package compat

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"music-platform/internal/music/model"
)

type SearchPreference int

const (
	SearchPreferenceLocalFirst SearchPreference = iota
	SearchPreferenceJamendoFirst
)

var (
	jamendoVirtualPathPattern = regexp.MustCompile(`\[jamendo-([A-Za-z0-9_-]+)\]`)
)

func DetectSearchPreference(keyword string) SearchPreference {
	hasCJK := false
	hasLatin := false

	for _, r := range strings.TrimSpace(keyword) {
		if unicode.Is(unicode.Han, r) {
			hasCJK = true
		}
		if unicode.IsLetter(r) && unicode.In(r, unicode.Latin) {
			hasLatin = true
		}
	}

	if hasCJK {
		return SearchPreferenceLocalFirst
	}
	if hasLatin {
		return SearchPreferenceJamendoFirst
	}
	return SearchPreferenceLocalFirst
}

func BuildJamendoVirtualPath(title, sourceID string) string {
	base := sanitizeVirtualTitle(title)
	if base == "" {
		base = "Jamendo Track"
	}
	return fmt.Sprintf("%s [jamendo-%s].mp3", base, strings.TrimSpace(sourceID))
}

func ParseJamendoSourceID(path string) (string, bool) {
	match := jamendoVirtualPathPattern.FindStringSubmatch(strings.TrimSpace(path))
	if len(match) != 2 {
		return "", false
	}
	sourceID := strings.TrimSpace(match[1])
	if sourceID == "" {
		return "", false
	}
	return sourceID, true
}

func BuildFileListItemFromExternalTrack(track *model.ExternalMusicTrack) *model.FileListItem {
	if track == nil {
		return nil
	}

	item := &model.FileListItem{
		Path:     BuildJamendoVirtualPath(track.Title, track.SourceID),
		Duration: fmt.Sprintf("%.2f seconds", track.DurationSec),
		Artist:   strings.TrimSpace(track.Artist),
	}
	if cover := strings.TrimSpace(track.CoverArtURL); cover != "" {
		item.CoverArtURL = &cover
	}
	return item
}

func BuildMusicResponseFromExternalTrack(track *model.ExternalMusicTrack) *model.MusicResponse {
	if track == nil {
		return nil
	}

	resp := &model.MusicResponse{
		StreamURL: strings.TrimSpace(track.StreamURL),
		Title:     strings.TrimSpace(track.Title),
		Artist:    strings.TrimSpace(track.Artist),
		Album:     strings.TrimSpace(track.Album),
	}
	if track.DurationSec > 0 {
		duration := track.DurationSec
		resp.Duration = &duration
	}
	if cover := strings.TrimSpace(track.CoverArtURL); cover != "" {
		resp.AlbumCoverURL = &cover
	}
	return resp
}

func sanitizeVirtualTitle(title string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, r := range trimmed {
		switch {
		case unicode.IsControl(r):
			builder.WriteRune(' ')
		case strings.ContainsRune(`\/:*?"<>|`, r):
			builder.WriteRune(' ')
		default:
			builder.WriteRune(r)
		}
	}

	sanitized := strings.Join(strings.Fields(builder.String()), " ")
	sanitized = strings.Trim(sanitized, ". ")
	if len([]rune(sanitized)) > 96 {
		sanitized = string([]rune(sanitized)[:96])
		sanitized = strings.TrimRight(sanitized, ". ")
	}
	return sanitized
}
