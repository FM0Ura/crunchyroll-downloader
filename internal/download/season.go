package download

import (
	"context"
	"fmt"
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
)

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

	// TDD RED placeholder: tvshow.nfo pre-loop call site added in GREEN commit.

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
