package nfo

import (
	"context"
	"fmt"
	"os"

	"crunchyroll-downloader/internal/api"
)

// WriteEpisodeFn / WriteTVShowFn are package-level seam vars mirroring
// internal/download/episode.go:46 (episodeMerge). Default to the real
// functions so production callers get real behavior; tests override via
// assignment + t.Cleanup restore (Task 3 injection point).
var (
	WriteEpisodeFn  = WriteEpisode
	WriteTVShowFn   = WriteTVShow
)

// marshalEpisode builds the episodedetails XML document from an EpisodeInfo
// + contentID, returning the marshaled bytes (header-prepended). Split from
// WriteEpisode so the escape cases (Pitfall 6) are testable without disk IO.
// Uses info.EpisodeMetadata.SeasonNumber (NOT EpisodeNumber) for <season>.
//
// STUB for TDD RED phase — returns nil, nil so escape tests fail.
func marshalEpisode(info *api.EpisodeInfo, contentID string) ([]byte, error) {
	return nil, nil
}

// marshalTVShow builds the tvshow XML document from a SeriesInfo, returning
// the marshaled bytes (header-prepended). The uniqueid chardata is info.ID
// (the Crunchyroll series content id — D-08).
//
// STUB for TDD RED phase — returns nil, nil so escape tests fail.
func marshalTVShow(info *api.SeriesInfo) ([]byte, error) {
	return nil, nil
}

// WriteEpisode writes a per-episode .nfo at path from info + contentID.
// The caller (download/episode.go) converts the returned error to a
// non-fatal warn per D-09 — nfo.go is a pure emitter, it never logs/warns.
func WriteEpisode(ctx context.Context, path string, info *api.EpisodeInfo, contentID string) error {
	out, err := marshalEpisode(info, contentID)
	if err != nil {
		return fmt.Errorf("nfo episode %s: %w", path, err)
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("nfo episode %s: %w", path, err)
	}
	return nil
}

// WriteTVShow writes a tvshow.nfo at path from info. Same pure-emitter
// convention as WriteEpisode: returns errors, never logs.
func WriteTVShow(ctx context.Context, path string, info *api.SeriesInfo) error {
	out, err := marshalTVShow(info)
	if err != nil {
		return fmt.Errorf("nfo tvshow %s: %w", path, err)
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("nfo tvshow %s: %w", path, err)
	}
	return nil
}