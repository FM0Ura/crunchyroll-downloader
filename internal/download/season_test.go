package download

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"crunchyroll-downloader/internal/api"
)

func TestRunSeasonEmptyEpisodes(t *testing.T) {
	err := runSeason(context.Background(), nil, nil, nil, nil, nil, nil, 0, "", nil)
	if err != nil {
		t.Fatalf("runSeason(empty) error = %v, want nil", err)
	}
}

func TestSeasonErrorFormatting(t *testing.T) {
	err := &SeasonError{Failed: 2, Total: 5}
	want := "2 of 5 episode(s) failed"
	if got := err.Error(); got != want {
		t.Fatalf("SeasonError.Error() = %q, want %q", got, want)
	}
}

func TestFormatFailedList(t *testing.T) {
	failures := []episodeError{
		{Number: 1, Title: "First", Err: errors.New("network error")},
		{Number: 3, Title: "Third", Err: errors.New("timeout")},
	}
	result := formatFailedList(failures)
	if !strings.Contains(result, "episode 1") {
		t.Fatalf("formatFailedList() = %q, want episode 1", result)
	}
	if !strings.Contains(result, "network error") {
		t.Fatalf("formatFailedList() = %q, want network error", result)
	}
	if !strings.Contains(result, "episode 3") {
		t.Fatalf("formatFailedList() = %q, want episode 3", result)
	}
	if !strings.Contains(result, "timeout") {
		t.Fatalf("formatFailedList() = %q, want timeout", result)
	}
	if !strings.Contains(result, ";") {
		t.Fatalf("formatFailedList() = %q, want semicolon separator", result)
	}
}

func TestRunSeasonContinuesAfterEpisodeFailure(t *testing.T) {
	// Phase 7: runSeason now fetches series info (via seriesGetSeriesInfo) before
	// the episode loop. This test passes a nil client + empty SeriesID, so stub
	// the series-info + tvshow.nfo seams so the nil client is not dereferenced.
	origGetSeries := seriesGetSeriesInfo
	origWriteTvshow := seriesWriteTvshowNfo
	seriesGetSeriesInfo = func(context.Context, *api.Client, string, string, string) (*api.SeriesInfo, error) {
		return nil, fmt.Errorf("stub: nil client")
	}
	seriesWriteTvshowNfo = func(context.Context, string, *api.SeriesInfo) error { return nil }
	t.Cleanup(func() {
		seriesGetSeriesInfo = origGetSeries
		seriesWriteTvshowNfo = origWriteTvshow
	})

	videoQuality := "1080p"
	audioQuality := "192k"
	episodes := []api.SeasonEpisode{
		{
			ID:            "episode-1",
			SeriesTitle:   "Test Series",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AudioLocale:   "ja-JP",
			Title:         "First",
		},
		{
			ID:            "episode-2",
			SeriesTitle:   "Test Series",
			SeasonNumber:  1,
			EpisodeNumber: 2,
			AudioLocale:   "ja-JP",
			Title:         "Second",
		},
	}

	firstErr := errors.New("first episode failed")
	var calls []int
	err := runSeason(context.Background(), nil, &videoQuality, &audioQuality, []string{"ja-JP"}, nil, episodes, 2, "",
		func(_ context.Context, _ *api.Client, _ string, info *api.EpisodeInfo, _ []string, _ []string, _ *string, _ *string, workers int, outputDir string, totalEpisodes int) error {
			if workers != 2 {
				t.Fatalf("workers = %d, want 2", workers)
			}
			if outputDir != "" {
				t.Fatalf("outputDir = %q, want \"\"", outputDir)
			}
			calls = append(calls, info.EpisodeMetadata.EpisodeNumber)
			if info.EpisodeMetadata.EpisodeNumber == 1 {
				return firstErr
			}
			return nil
		})

	if len(calls) != 2 {
		t.Fatalf("runSeason() called downloader %d times, want 2", len(calls))
	}
	if calls[0] != 1 || calls[1] != 2 {
		t.Fatalf("runSeason() calls = %v, want [1 2]", calls)
	}
	if err == nil {
		t.Fatal("runSeason() error = nil, want aggregate season error")
	}
	var seasonErr *SeasonError
	if !errors.As(err, &seasonErr) {
		t.Fatalf("runSeason() error type = %T, want *SeasonError", err)
	}
	if seasonErr.Failed != 1 || seasonErr.Total != 2 {
		t.Fatalf("SeasonError = failed %d total %d, want failed 1 total 2", seasonErr.Failed, seasonErr.Total)
	}
	if !errors.Is(err, firstErr) {
		t.Fatalf("runSeason() error does not wrap first episode failure: %v", err)
	}
}

// TestRunSeasonWritesTvshowNfoOnceBeforeEpisodeLoop (Task 3, D-07 LOCKED)
// asserts: (a) on the DEFAULT path, seriesGetSeriesInfo IS invoked with
// episodes[0].SeriesID and seriesWriteTvshowNfo receives the rich SeriesInfo
// (NOT a title-only one); (b) when a tvshow.nfo already exists, the os.Stat
// guard SKIPS both the fetch and the write.
func TestRunSeasonWritesTvshowNfoOnceBeforeEpisodeLoop(t *testing.T) {
	videoQuality := "1080p"
	audioQuality := "192k"
	episodes := []api.SeasonEpisode{
		{
			ID:            "episode-1",
			SeriesTitle:   "Test Series",
			SeriesID:      "GSERIES-SZ",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AudioLocale:   "ja-JP",
			Title:         "First",
		},
	}

	t.Run("default_path_rich_series_info", func(t *testing.T) {
		var seriesCalledWith string
		var tvshowInfo *api.SeriesInfo
		origGetSeries := seriesGetSeriesInfo
		origWriteTvshow := seriesWriteTvshowNfo
		seriesGetSeriesInfo = func(_ context.Context, _ *api.Client, seriesID, _, _ string) (*api.SeriesInfo, error) {
			seriesCalledWith = seriesID
			return &api.SeriesInfo{
				ID:          "GSERIES-SZ",
				Title:       "Test Series",
				Description: "Rich series plot",
				Genres:      []string{"Drama"},
				Studio:      "Studio Ghibli",
			}, nil
		}
		seriesWriteTvshowNfo = func(_ context.Context, _ string, info *api.SeriesInfo) error {
			tvshowInfo = info
			return nil
		}
		t.Cleanup(func() {
			seriesGetSeriesInfo = origGetSeries
			seriesWriteTvshowNfo = origWriteTvshow
		})

		err := runSeason(context.Background(), nil, &videoQuality, &audioQuality, []string{"ja-JP"}, nil, episodes, 2, "",
			func(context.Context, *api.Client, string, *api.EpisodeInfo, []string, []string, *string, *string, int, string, int) error { return nil })
		if err != nil {
			t.Fatalf("runSeason() error = %v, want nil default path", err)
		}
		if seriesCalledWith != "GSERIES-SZ" {
			t.Fatalf("seriesGetSeriesInfo called with seriesID=%q, want GSERIES-SZ", seriesCalledWith)
		}
		if tvshowInfo == nil {
			t.Fatal("seriesWriteTvshowNfo not invoked")
		}
		if tvshowInfo.Description != "Rich series plot" {
			t.Fatalf("tvshow info = title-only (Description=%q); want rich SeriesInfo (D-07 default path)", tvshowInfo.Description)
		}
		if tvshowInfo.Studio != "Studio Ghibli" {
			t.Fatalf("tvshow Studio = %q, want Studio Ghibli (rich, not title-only)", tvshowInfo.Studio)
		}
	})

	t.Run("skip_when_tvshow_nfo_exists", func(t *testing.T) {
		dir := t.TempDir()
		// Pre-create the tvshow.nfo at the series root so the os.Stat guard fires.
		seriesDir := filepath.Join(dir, sanitizeFilename("Test Series"))
		if err := os.MkdirAll(seriesDir, 0o777); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		tvshowPath := filepath.Join(seriesDir, "tvshow.nfo")
		if err := os.WriteFile(tvshowPath, []byte("exists"), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		seriesCalled := false
		writeCalled := false
		origGetSeries := seriesGetSeriesInfo
		origWriteTvshow := seriesWriteTvshowNfo
		seriesGetSeriesInfo = func(context.Context, *api.Client, string, string, string) (*api.SeriesInfo, error) {
			seriesCalled = true
			return &api.SeriesInfo{ID: "GSERIES-SZ", Title: "Test Series"}, nil
		}
		seriesWriteTvshowNfo = func(context.Context, string, *api.SeriesInfo) error {
			writeCalled = true
			return nil
		}
		t.Cleanup(func() {
			seriesGetSeriesInfo = origGetSeries
			seriesWriteTvshowNfo = origWriteTvshow
		})

		err := runSeason(context.Background(), nil, &videoQuality, &audioQuality, []string{"ja-JP"}, nil, episodes, 2, dir,
			func(context.Context, *api.Client, string, *api.EpisodeInfo, []string, []string, *string, *string, int, string, int) error { return nil })
		if err != nil {
			t.Fatalf("runSeason() error = %v, want nil with existing tvshow.nfo", err)
		}
		if seriesCalled {
			t.Fatal("seriesGetSeriesInfo invoked despite existing tvshow.nfo (D-02 os.Stat guard should skip)")
		}
		if writeCalled {
			t.Fatal("seriesWriteTvshowNfo invoked despite existing tvshow.nfo (D-02 os.Stat guard should skip)")
		}
	})
}

// TestRunSeasonTvshowNfoNonFatal (Task 3, D-09) asserts: (a) when
// seriesGetSeriesInfo returns an error, the episode loop STILL completes ALL
// episodes (no early return) and the warn line fires; (b) a title-only
// tvshow.nfo IS written as the error-fallback.
func TestRunSeasonTvshowNfoNonFatal(t *testing.T) {
	videoQuality := "1080p"
	audioQuality := "192k"
	episodes := []api.SeasonEpisode{
		{ID: "e1", SeriesTitle: "Test Series", SeriesID: "GSERIES-NF", SeasonNumber: 1, EpisodeNumber: 1, AudioLocale: "ja-JP", Title: "First"},
		{ID: "e2", SeriesTitle: "Test Series", SeriesID: "GSERIES-NF", SeasonNumber: 1, EpisodeNumber: 2, AudioLocale: "ja-JP", Title: "Second"},
	}

	var tvshowInfo *api.SeriesInfo
	origGetSeries := seriesGetSeriesInfo
	origWriteTvshow := seriesWriteTvshowNfo
	seriesGetSeriesInfo = func(context.Context, *api.Client, string, string, string) (*api.SeriesInfo, error) {
		return nil, fmt.Errorf("API down")
	}
	seriesWriteTvshowNfo = func(_ context.Context, _ string, info *api.SeriesInfo) error {
		tvshowInfo = info
		return nil
	}
	t.Cleanup(func() {
		seriesGetSeriesInfo = origGetSeries
		seriesWriteTvshowNfo = origWriteTvshow
	})

	var epCalls int
	err := runSeason(context.Background(), nil, &videoQuality, &audioQuality, []string{"ja-JP"}, nil, episodes, 2, "",
		func(context.Context, *api.Client, string, *api.EpisodeInfo, []string, []string, *string, *string, int, string, int) error {
			epCalls++
			return nil
		})
	if err != nil {
		t.Fatalf("runSeason() error = %v, want nil despite GetSeriesInfo failure (non-fatal D-09)", err)
	}
	if epCalls != 2 {
		t.Fatalf("episode loop ran %d time(s); want 2 (loop must NOT abort on series-info failure)", epCalls)
	}
	if tvshowInfo == nil {
		t.Fatal("seriesWriteTvshowNfo not invoked on error path")
	}
	if tvshowInfo.Description != "" || tvshowInfo.Studio != "" {
		t.Fatalf("tvshow info = %#v; want title-only fallback (ID+Title only) on error path", tvshowInfo)
	}
	if tvshowInfo.ID != "GSERIES-NF" || tvshowInfo.Title != "Test Series" {
		t.Fatalf("tvshow fallback = %#v, want ID+Title from episodes[0]", tvshowInfo)
	}
}
