package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"crunchyroll-downloader/internal/api"
	"crunchyroll-downloader/internal/diag"
	"crunchyroll-downloader/internal/nfo"
	"crunchyroll-downloader/internal/output"
)

type episodeDownloader func(ctx context.Context, client *api.Client, baseContentID string, info *api.EpisodeInfo, audioLangs, subsLangs []string, videoQuality, audioQuality *string, workers int, outputDir string, totalEpisodes int) error

// seriesGetSeriesInfo / seriesWriteTvshowNfo are package-level seam vars
// mirroring the episodeDownloader seam (season.go) and episodeMerge
// (episode.go). Default to the real functions; tests override via
// assignment + t.Cleanup restore.
var (
	seriesGetSeriesInfo = func(ctx context.Context, client *api.Client, seriesID, audioLocale, subLocale string) (*api.SeriesInfo, error) {
		return client.GetSeriesInfo(ctx, seriesID, audioLocale, subLocale)
	}
	seriesWriteTvshowNfo = nfo.WriteTVShow
	seriesFetchArtwork   = func(ctx context.Context, client *api.Client, artworkURL, destPath string) error {
		return client.FetchArtwork(ctx, artworkURL, destPath)
	}
)

type artworkFetcher func(context.Context, *api.Client, string, string) error

type seriesArtwork struct {
	posterURL   string
	backdropURL string
}

func (a seriesArtwork) empty() bool {
	return a.posterURL == "" && a.backdropURL == ""
}

func seriesArtworkFromInfo(info *api.SeriesInfo) seriesArtwork {
	if info == nil {
		return seriesArtwork{}
	}
	return seriesArtwork{
		posterURL:   info.PosterURL,
		backdropURL: info.BackdropURL,
	}
}

func needsSeriesArtwork(seriesRoot string) bool {
	for _, name := range []string{"poster.jpg", "backdrop.jpg"} {
		if _, err := os.Stat(filepath.Join(seriesRoot, name)); os.IsNotExist(err) {
			return true
		}
	}
	return false
}

func writeSeriesArtwork(ctx context.Context, client *api.Client, seriesTitle, seriesRoot string, artwork seriesArtwork, fetch artworkFetcher) {
	targets := []struct {
		name string
		url  string
	}{
		{name: "poster.jpg", url: artwork.posterURL},
		{name: "backdrop.jpg", url: artwork.backdropURL},
	}

	loggedNoURL := false
	for _, target := range targets {
		destPath := filepath.Join(seriesRoot, target.name)
		if _, err := os.Stat(destPath); err == nil {
			continue
		}
		if target.url == "" {
			if !loggedNoURL && diag.ApiLogger != nil {
				diag.ApiLogger.Info("artwork_no_url", "series", seriesTitle)
			}
			loggedNoURL = true
			continue
		}
		if err := fetch(ctx, client, target.url, destPath); err != nil {
			output.Global.Warn("No artwork available for %s: %v", seriesTitle, err)
			if diag.ApiLogger != nil {
				diag.ApiLogger.Warn("artwork_fetch_failed", "series", seriesTitle, "file", target.name, "err", err)
			}
		}
	}
}

type episodeError struct {
	Number int
	Title  string
	Err    error
}

type SeasonError struct {
	Failed int
	Total  int
	err    error
}

func (e *SeasonError) Error() string {
	return fmt.Sprintf("%d of %d episode(s) failed", e.Failed, e.Total)
}

func (e *SeasonError) Unwrap() error {
	return e.err
}

func formatFailedList(failures []episodeError) string {
	var parts []string
	for _, f := range failures {
		parts = append(parts, fmt.Sprintf("episode %d: %v", f.Number, f.Err))
	}
	return strings.Join(parts, "; ")
}

func Season(ctx context.Context, client *api.Client, videoQuality, audioQuality *string, audioLangs, subsLangs []string, episodes []api.SeasonEpisode, workers int, outputDir string) error {
	return runSeason(ctx, client, videoQuality, audioQuality, audioLangs, subsLangs, episodes, workers, outputDir, Episode)
}

func runSeason(ctx context.Context, client *api.Client, videoQuality, audioQuality *string, audioLangs, subsLangs []string, episodes []api.SeasonEpisode, workers int, outputDir string, downloadEpisode episodeDownloader) error {
	if len(episodes) == 0 {
		return nil
	}

	output.Global.Info("Downloading season %d of %s (%d episodes)", episodes[0].SeasonNumber, episodes[0].SeriesTitle, len(episodes))

	// D-07/D-09/D-02: once-per-series tvshow.nfo write BEFORE the episode loop.
	// The series root is the SAME inline 3-line pattern as episode.go outputBase
	// so tvshow.nfo and the Season NN folder are siblings at the series root.
	// os.Stat guard (D-02 resumability): multi-season invocations skip re-fetch.
	// DEFAULT path fires seriesGetSeriesInfo(episodes[0].SeriesID); title-only
	// fallback is written ONLY when GetSeriesInfo errors (D-09 non-fatal).
	cleanSeriesTitle := sanitizeFilename(episodes[0].SeriesTitle)
	seriesRoot := cleanSeriesTitle
	if outputDir != "" {
		seriesRoot = filepath.Join(outputDir, cleanSeriesTitle)
	}
	if err := os.MkdirAll(seriesRoot, 0777); err != nil {
		return fmt.Errorf("creating series directory: %w", err)
	}
	seriesInfo := (*api.SeriesInfo)(nil)
	seriesInfoFetched := false
	fetchSeriesInfo := func() (*api.SeriesInfo, error) {
		if seriesInfoFetched {
			return seriesInfo, nil
		}
		seriesInfoFetched = true
		var err error
		seriesInfo, err = seriesGetSeriesInfo(ctx, client, episodes[0].SeriesID, "", "")
		return seriesInfo, err
	}
	tvshowNfoPath := filepath.Join(seriesRoot, "tvshow.nfo")
	if _, err := os.Stat(tvshowNfoPath); err != nil {
		seriesID := episodes[0].SeriesID
		seriesInfo, serr := fetchSeriesInfo()
		if serr != nil || seriesInfo == nil {
			if serr != nil {
				output.Global.Warn("Failed to fetch series metadata for %s: %v", episodes[0].SeriesTitle, serr)
				if diag.ApiLogger != nil {
					diag.ApiLogger.Warn("series_info_fetch_failed", "series", episodes[0].SeriesTitle, "err", serr)
				}
			} else {
				output.Global.Warn("No series metadata returned for %s", episodes[0].SeriesTitle)
			}
			// Title-only error-fallback (D-09): write a tvshow.nfo regardless.
			fallbackInfo := &api.SeriesInfo{
				ID:    seriesID,
				Title: episodes[0].SeriesTitle,
			}
			if werr := seriesWriteTvshowNfo(ctx, tvshowNfoPath, fallbackInfo); werr != nil {
				output.Global.Warn("Failed to write tvshow.nfo for %s: %v", episodes[0].SeriesTitle, werr)
				if diag.MuxLogger != nil {
					diag.MuxLogger.Warn("tvshow_nfo_write_failed", "path", tvshowNfoPath, "err", werr)
				}
			}
		} else {
			if werr := seriesWriteTvshowNfo(ctx, tvshowNfoPath, seriesInfo); werr != nil {
				output.Global.Warn("Failed to write tvshow.nfo for %s: %v", episodes[0].SeriesTitle, werr)
				if diag.MuxLogger != nil {
					diag.MuxLogger.Warn("tvshow_nfo_write_failed", "path", tvshowNfoPath, "err", werr)
				}
			}
		}
	}
	if needsSeriesArtwork(seriesRoot) {
		if _, serr := fetchSeriesInfo(); serr != nil {
			output.Global.Warn("No artwork available for %s: %v", episodes[0].SeriesTitle, serr)
			if diag.ApiLogger != nil {
				diag.ApiLogger.Warn("artwork_fetch_failed", "series", episodes[0].SeriesTitle, "err", serr)
			}
		}
	}
	writeSeriesArtwork(ctx, client, episodes[0].SeriesTitle, seriesRoot, seriesArtworkFromInfo(seriesInfo), seriesFetchArtwork)

	var failures []episodeError
	for _, ep := range episodes {
		info := &api.EpisodeInfo{
			EpisodeMetadata: api.EpisodeMetadata{
				SeriesTitle:        ep.SeriesTitle,
				SeasonNumber:       ep.SeasonNumber,
				EpisodeNumber:      ep.EpisodeNumber,
				AudioLocale:        ep.AudioLocale,
				Versions:           ep.Versions,
				AvailabilityStarts: ep.AvailabilityStarts,
			},
			Title: ep.Title,
		}

		if err := downloadEpisode(ctx, client, ep.ID, info, audioLangs, subsLangs, videoQuality, audioQuality, workers, outputDir, len(episodes)); err != nil {
			output.Global.Error("[Episode %d/%d] %s ... ✗ %s", ep.EpisodeNumber, len(episodes), ep.Title, err.Error())
			failures = append(failures, episodeError{
				Number: ep.EpisodeNumber,
				Title:  ep.Title,
				Err:    fmt.Errorf("episode %v: %w", ep.EpisodeNumber, err),
			})
		}
	}

	if len(failures) > 0 {
		output.Global.Warn("Season %d download complete. %d of %d episode(s) failed:",
			episodes[0].SeasonNumber, len(failures), len(episodes))
		for _, f := range failures {
			output.Global.Error("  Episode %d: %v", f.Number, f.Err)
		}
		err := &SeasonError{
			Failed: len(failures),
			Total:  len(episodes),
			err:    fmt.Errorf("season %d: %d of %d episodes failed: %s: %w", episodes[0].SeasonNumber, len(failures), len(episodes), formatFailedList(failures), failures[0].Err),
		}
		if diag.DownloadLogger != nil {
			diag.DownloadLogger.Info("season_failure", "error", err.Error())
		}
		return err
	}

	output.Global.Info("%sSeason %d download complete. All %d episodes successful.%s",
		output.ANSIGreen, episodes[0].SeasonNumber, len(episodes), output.ANSIReset)
	return nil
}
