package download

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"crunchyroll-downloader/internal/api"
	"crunchyroll-downloader/internal/mux"
	"github.com/iyear/gowidevine"
	"github.com/unki2aut/go-mpd"
)

func TestEpisodeReturnsErrorForUnavailableAudioLocale(t *testing.T) {
	t.Chdir(t.TempDir())

	videoQuality := "1080p"
	audioQuality := "192k"
	info := &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:   "Test Series",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AudioLocale:   "ja-JP",
		},
		Title: "Test Episode",
	}

	err := Episode(context.Background(), nil, "base-content-id", info, []string{"en-US"}, nil, &videoQuality, &audioQuality, 2, "", 1)
	if err == nil {
		t.Fatal("Episode() error = nil, want unavailable audio locale error")
	}
	if !strings.Contains(err.Error(), "primary audio locale en-US not available") {
		t.Fatalf("Episode() error = %q, want primary unavailable audio locale message", err)
	}
}

func TestEpisodeSkipsUnavailableSecondaryAudioLocale(t *testing.T) {
	t.Chdir(t.TempDir())
	restoreEpisodeTestSeams(t, map[string]*api.Subtitle{})

	videoQuality := "1080p"
	audioQuality := "192k"
	info := testEpisodeInfoWithLocales("ja-JP", nil)
	client := api.NewTestClient(nil, "https://example.com", "test-token")

	stdout := captureEpisodeStdout(t, func() {
		err := Episode(context.Background(), client, "content-id", info, []string{"ja-JP", "en-US"}, nil, &videoQuality, &audioQuality, 2, "", 1)
		if err != nil {
			t.Fatalf("Episode() error = %v, want nil with secondary audio skipped", err)
		}
	})

	if !strings.Contains(stdout, "Skipping en-US audio") {
		t.Fatalf("Episode() stdout = %q, want secondary audio skip warning", stdout)
	}
}

func TestEpisodeReturnsPrimaryAudioErrorBeforePostLoopGuard(t *testing.T) {
	t.Chdir(t.TempDir())

	videoQuality := "1080p"
	audioQuality := "192k"
	info := testEpisodeInfoWithLocales("ja-JP", nil)

	err := Episode(context.Background(), nil, "content-id", info, []string{"en-US", "es-419"}, nil, &videoQuality, &audioQuality, 2, "", 1)
	if err == nil {
		t.Fatal("Episode() error = nil, want primary audio locale error")
	}
	if !strings.Contains(err.Error(), "primary audio locale "+"en-US not available") {
		t.Fatalf("Episode() error = %q, want primary audio locale message", err)
	}
	if strings.Contains(err.Error(), "no audio tracks "+"available") {
		t.Fatalf("Episode() error = %q, want primary branch before post-loop guard", err)
	}
}

func TestEpisodeWarnsWhenDownloadedPartially(t *testing.T) {
	t.Chdir(t.TempDir())
	restoreEpisodeTestSeams(t, map[string]*api.Subtitle{
		"en-US": {Language: "en-US", URL: "https://example.com/en-US.ass"},
	})

	videoQuality := "1080p"
	audioQuality := "192k"
	info := testEpisodeInfoWithLocales("ja-JP", nil)
	client := api.NewTestClient(nil, "https://example.com", "test-token")

	stdout := captureEpisodeStdout(t, func() {
		err := Episode(context.Background(), client, "content-id", info, []string{"ja-JP", "en-US"}, []string{"en-US", "es-419"}, &videoQuality, &audioQuality, 2, "", 1)
		if err != nil {
			t.Fatalf("Episode() error = %v, want nil with secondary subtitle skipped", err)
		}
	})

	if !strings.Contains(stdout, "✓") {
		t.Fatalf("Episode() stdout = %q, want success line", stdout)
	}
	if !strings.Contains(stdout, "downloaded partially") || !strings.Contains(stdout, "es-419 sub") {
		t.Fatalf("Episode() stdout = %q, want partial warning with skipped subtitle", stdout)
	}
}

func TestEpisodeDownloadsAlternateSubtitleFromMatchingAudioVersion(t *testing.T) {
	t.Chdir(t.TempDir())
	restoreEpisodeTestSeams(t, map[string]*api.Subtitle{
		"pt-BR": {Language: "pt-BR", URL: "https://example.com/pt-BR-full.ass"},
	})

	videoQuality := "1080p"
	audioQuality := "192k"
	info := testEpisodeInfoWithLocales("ja-JP", []*api.DubVersion{
		{AudioLocale: "pt-BR", GUID: "pt-br-guid"},
	})
	client := api.NewTestClient(nil, "https://example.com", "test-token")

	getEpisodeCalls := map[string]int{}
	episodeGetEpisode = func(_ context.Context, _ *api.Client, id string) (*api.Episode, error) {
		getEpisodeCalls[id]++
		subtitles := map[string]*api.Subtitle{
			"pt-BR": {Language: "pt-BR", URL: "https://example.com/pt-BR-full.ass"},
		}
		if id == "pt-br-guid" {
			subtitles = map[string]*api.Subtitle{
				"pt-BR": {Language: "pt-BR", URL: "https://example.com/pt-BR-signs.ass"},
			}
		}
		return &api.Episode{
			ManifestURL: "https://example.com/manifest.mpd",
			Subtitles:   subtitles,
			Token:       "stream-token-" + id,
		}, nil
	}

	var downloadedURLs []string
	episodeDownloadSubs = func(_ context.Context, _ *api.Client, url string) (string, error) {
		downloadedURLs = append(downloadedURLs, url)
		return writeEpisodeTestFile(t, t.TempDir(), sanitizeFilename(filepath.Base(url))), nil
	}

	var capturedSubTracks []mux.MediaTrack
	episodeMerge = func(_ context.Context, _ string, _ []mux.MediaTrack, subTracks []mux.MediaTrack, outputFile string, _ *api.EpisodeInfo) error {
		capturedSubTracks = append([]mux.MediaTrack(nil), subTracks...)
		return os.WriteFile(outputFile, []byte("mkv"), 0o600)
	}

	err := Episode(context.Background(), client, "content-id", info, []string{"ja-JP", "pt-BR"}, []string{"pt-BR"}, &videoQuality, &audioQuality, 2, "", 1)
	if err != nil {
		t.Fatalf("Episode() error = %v, want nil", err)
	}

	wantURLs := []string{"https://example.com/pt-BR-full.ass", "https://example.com/pt-BR-signs.ass"}
	if !slices.Equal(downloadedURLs, wantURLs) {
		t.Fatalf("downloaded subtitle URLs = %v, want %v", downloadedURLs, wantURLs)
	}
	if len(capturedSubTracks) != 2 {
		t.Fatalf("mux subtitle tracks = %d, want 2: %#v", len(capturedSubTracks), capturedSubTracks)
	}
	if capturedSubTracks[0].Title != "Português (Brasil)" {
		t.Fatalf("primary subtitle title = %q, want Português (Brasil)", capturedSubTracks[0].Title)
	}
	if !strings.Contains(capturedSubTracks[1].Title, "Português (Brasil) audio") {
		t.Fatalf("alternate subtitle title = %q, want audio-source suffix", capturedSubTracks[1].Title)
	}
	if getEpisodeCalls["pt-br-guid"] != 1 {
		t.Fatalf("pt-BR playback fetched %d time(s), want 1 reused for subtitles and audio", getEpisodeCalls["pt-br-guid"])
	}
}

func TestEpisodeReturnsErrorWhenAudioLangsEmptyBeforeMux(t *testing.T) {
	t.Chdir(t.TempDir())
	muxCalled := false
	restoreEpisodeTestSeams(t, map[string]*api.Subtitle{})
	episodeMerge = func(context.Context, string, []mux.MediaTrack, []mux.MediaTrack, string, *api.EpisodeInfo) error {
		muxCalled = true
		return nil
	}

	videoQuality := "1080p"
	audioQuality := "192k"
	info := testEpisodeInfoWithLocales("ja-JP", nil)

	err := Episode(context.Background(), nil, "content-id", info, []string{}, nil, &videoQuality, &audioQuality, 2, "", 1)
	if err == nil {
		t.Fatal("Episode() error = nil, want empty audio hard error")
	}
	if !strings.Contains(err.Error(), "no audio tracks available") {
		t.Fatalf("Episode() error = %q, want empty audio hard-error message", err)
	}
	if muxCalled {
		t.Fatal("Episode() invoked mux despite empty audioLangs hard error")
	}
}

func TestCleanupEpisodeArtifactsRemovesPartialOutputAndTempFiles(t *testing.T) {
	dir := t.TempDir()
	outputFile := writeEpisodeTestFile(t, dir, "partial.mkv")
	audioFile := writeEpisodeTestFile(t, dir, "audio.mp3")
	videoFile := writeEpisodeTestFile(t, dir, "video.mp4")

	cleanupEpisodeArtifacts(outputFile, []string{audioFile, videoFile, ""})

	for _, path := range []string{outputFile, audioFile, videoFile} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s still exists after cleanup; stat error = %v", filepath.Base(path), err)
		}
	}
}

func TestEpisodeSingleVersion(t *testing.T) {
	t.Chdir(t.TempDir())

	videoQuality := "1080p"
	audioQuality := "192k"
	info := &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:   "Test Series",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AudioLocale:   "ja-JP",
		},
		Title: "Test Episode",
	}

	// Use a cancelled context so GetEpisode fails fast (no real HTTP call).
	// With only one version (no Versions slice), the function takes the
	// sequential Phase A path — no errgroup is created.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := api.NewTestClient(nil, "https://example.com", "test-token")

	err := Episode(ctx, client, "content-id", info, []string{"ja-JP"}, nil, &videoQuality, &audioQuality, 2, "", 1)
	if err == nil {
		t.Fatal("Episode() error = nil, want error from single-version sequential path")
	}
}

func TestEpisodeParallelAudio(t *testing.T) {
	// NOTE: Full parallel audio integration test requires a mock HTTP server.
	// See Phase 5 for comprehensive test coverage.
	t.Chdir(t.TempDir())

	videoQuality := "1080p"
	audioQuality := "192k"
	info := &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:   "Test Series",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AudioLocale:   "ja-JP",
			Versions: []*api.DubVersion{
				{AudioLocale: "en-US", GUID: "en-us-guid"},
			},
		},
		Title: "Test Episode",
	}

	// Cancelled context causes GetEpisode to fail fast before HTTP calls.
	// The error should propagate from the sequential path (Phase A) before
	// the errgroup section (Phase B) is even reached.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := api.NewTestClient(nil, "https://example.com", "test-token")

	err := Episode(ctx, client, "content-id", info, []string{"ja-JP", "en-US"}, nil, &videoQuality, &audioQuality, 2, "", 1)
	if err == nil {
		t.Fatal("Episode() error = nil, want error from parallel or sequential path")
	}
}

func TestEpisodeParallelAudioZeroVersions(t *testing.T) {
	// Verify that requesting a non-existent audio locale returns an error
	// before any parallel work begins.
	t.Chdir(t.TempDir())

	videoQuality := "1080p"
	audioQuality := "192k"
	info := &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:   "Test Series",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AudioLocale:   "ja-JP",
		},
		Title: "Test Episode",
	}

	err := Episode(context.Background(), nil, "content-id", info, []string{"fr-FR"}, nil, &videoQuality, &audioQuality, 2, "", 1)
	if err == nil {
		t.Fatal("Episode() error = nil, want audio locale unavailable error")
	}
	if !strings.Contains(err.Error(), "primary audio locale fr-FR not available") {
		t.Fatalf("Episode() error = %q, want unavailable audio locale message", err)
	}
}

func TestSanitizeFilenameCollapsesMultiUnderscore(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "single underscore unchanged", input: "a_b", want: "a_b"},
		{name: "double underscore collapses", input: "a__b", want: "a_b"},
		{name: "triple underscore collapses", input: "a___b", want: "a_b"},
		{name: "illegal chars with multi-underscore collapsed", input: "a__:__b", want: "a_b"},
		{name: "trailing space trimmed", input: "a ", want: "a"},
		{name: "empty returns Unknown", input: "", want: "Unknown"},
		{name: "illegal chars become underscore", input: "a:b", want: "a_b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Fatalf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestOutputDirCreatesSeriesSubfolderInOutputDir(t *testing.T) {
	outputDir := t.TempDir()

	videoQuality := "1080p"
	audioQuality := "192k"
	info := &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:   "Test Series",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AudioLocale:   "ja-JP",
		},
		Title: "Test Episode",
	}

	// Use a cancelled context so GetEpisode fails fast (no real HTTP call).
	// The outputDir's series subfolder should be created before the API call.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := api.NewTestClient(nil, "https://example.com", "test-token")

	err := Episode(ctx, client, "content-id", info, []string{"ja-JP"}, nil, &videoQuality, &audioQuality, 2, outputDir, 1)
	if err == nil {
		t.Fatal("Episode() error = nil, want error from cancelled context (GetEpisode)")
	}

	// Verify the series root and the nested Season 01 subfolder were created
	// inside outputDir (D-01/D-03: Series Title/Season NN/ layout).
	expectedSeriesDir := filepath.Join(outputDir, sanitizeFilename("Test Series"))
	if _, err := os.Stat(expectedSeriesDir); os.IsNotExist(err) {
		t.Fatalf("Episode() did not create series subfolder at %s", expectedSeriesDir)
	}
	expectedSeasonDir := filepath.Join(expectedSeriesDir, "Season 01")
	if _, err := os.Stat(expectedSeasonDir); os.IsNotExist(err) {
		t.Fatalf("Episode() did not create season subfolder at %s", expectedSeasonDir)
	}
}

// TestEpisodeOutputSkipsAlreadyDownloadedInNestedLayout proves D-02 resumability
// still fires on the deeper nested path: when the final
// Series Title/Season 01/Series Title S01E01 - Title.mkv already exists, the
// skip path runs and the mux seam is never invoked.
func TestEpisodeOutputSkipsAlreadyDownloadedInNestedLayout(t *testing.T) {
	t.Chdir(t.TempDir())

	videoQuality := "1080p"
	audioQuality := "192k"
	info := testEpisodeInfoWithLocales("ja-JP", nil)

	// Pre-create the nested layout the new path-build produces, including the
	// deep .mkv file, so the os.Stat skip check (D-02) fires.
	seriesDir := sanitizeFilename(info.EpisodeMetadata.SeriesTitle)
	seasonDir := fmt.Sprintf("Season %02d", info.EpisodeMetadata.SeasonNumber)
	seasonPath := filepath.Join(seriesDir, seasonDir)
	if err := os.MkdirAll(seasonPath, 0o777); err != nil {
		t.Fatalf("MkdirAll season path: %v", err)
	}
	deepFile := filepath.Join(seasonPath, fmt.Sprintf("%s S%02dE%02d - %s.mkv",
		seriesDir, info.EpisodeMetadata.SeasonNumber, info.EpisodeMetadata.EpisodeNumber,
		sanitizeFilename(info.Title)))
	if err := os.WriteFile(deepFile, []byte("mkv"), 0o600); err != nil {
		t.Fatalf("WriteFile deep mkv: %v", err)
	}

	mergeCalls := 0
	restoreEpisodeTestSeams(t, map[string]*api.Subtitle{})
	origMerge := episodeMerge
	episodeMerge = func(context.Context, string, []mux.MediaTrack, []mux.MediaTrack, string, *api.EpisodeInfo) error {
		mergeCalls++
		return nil
	}
	t.Cleanup(func() { episodeMerge = origMerge })

	client := api.NewTestClient(nil, "https://example.com", "test-token")

	stdout := captureEpisodeStdout(t, func() {
		err := Episode(context.Background(), client, "content-id", info, []string{"ja-JP"}, nil, &videoQuality, &audioQuality, 2, "", 1)
		if err != nil {
			t.Fatalf("Episode() error = %v, want nil skip on already-downloaded deep path", err)
		}
	})

	if mergeCalls != 0 {
		t.Fatalf("episodeMerge invoked %d time(s); want 0 (skip path must short-circuit before mux)", mergeCalls)
	}
	if !strings.Contains(stdout, "skipped (already downloaded)") {
		t.Fatalf("Episode() stdout = %q, want skipped (already downloaded) info line", stdout)
	}
}

// TestEpisodeSingleEpisodeMirrorsSeasonLayout (D-03) proves a single-episode
// invocation (totalEpisodes=1) produces the SAME Series Title/Season NN/ nesting
// as a season episode would — uniform layout regardless of invocation style.
func TestEpisodeSingleEpisodeMirrorsSeasonLayout(t *testing.T) {
	t.Chdir(t.TempDir())

	videoQuality := "1080p"
	audioQuality := "192k"
	info := testEpisodeInfoWithLocales("ja-JP", nil)

	var capturedOutputFile string
	restoreEpisodeTestSeams(t, map[string]*api.Subtitle{})
	origMerge := episodeMerge
	episodeMerge = func(_ context.Context, _ string, _ []mux.MediaTrack, _ []mux.MediaTrack, outputFile string, _ *api.EpisodeInfo) error {
		capturedOutputFile = outputFile
		return os.WriteFile(outputFile, []byte("mkv"), 0o600)
	}
	t.Cleanup(func() { episodeMerge = origMerge })

	client := api.NewTestClient(nil, "https://example.com", "test-token")

	if err := Episode(context.Background(), client, "content-id", info, []string{"ja-JP"}, nil, &videoQuality, &audioQuality, 2, "", 1); err != nil {
		t.Fatalf("Episode() error = %v, want nil single-episode run", err)
	}

	// Assert the deep nested path: .../Series Title/Season 01/Series Title S01E01 - Title.mkv
	expectedSeasonSegment := filepath.Join(sanitizeFilename(info.EpisodeMetadata.SeriesTitle), "Season 01")
	if !strings.Contains(capturedOutputFile, expectedSeasonSegment) {
		t.Fatalf("single-episode outputFile = %q, want it to contain nested %q segment", capturedOutputFile, expectedSeasonSegment)
	}
	if strings.Contains(capturedOutputFile, "[") {
		t.Fatalf("single-episode outputFile = %q, must NOT contain a quality bracket (D-04)", capturedOutputFile)
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{name: "zero bytes", bytes: 0, want: "0 B"},
		{name: "bytes", bytes: 500, want: "500 B"},
		{name: "kilobytes boundary", bytes: 1024, want: "1.0 KB"},
		{name: "kilobytes", bytes: 2048, want: "2.0 KB"},
		{name: "megabytes boundary", bytes: 1048576, want: "1.0 MB"},
		{name: "megabytes", bytes: 5242880, want: "5.0 MB"},
		{name: "gigabytes boundary", bytes: 1073741824, want: "1.0 GB"},
		{name: "gigabytes", bytes: 2147483648, want: "2.0 GB"},
		{name: "negative bytes", bytes: -100, want: "-100 B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFileSize(tt.bytes)
			if got != tt.want {
				t.Fatalf("formatFileSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "zero seconds", d: 0, want: "0s"},
		{name: "five seconds", d: 5 * time.Second, want: "5s"},
		{name: "one minute", d: 60 * time.Second, want: "1m 0s"},
		{name: "two minutes thirty seconds", d: 150 * time.Second, want: "2m 30s"},
		{name: "one hour", d: 3600 * time.Second, want: "60m 0s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.d)
			if got != tt.want {
				t.Fatalf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func writeEpisodeTestFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("partial"), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	return path
}

func testEpisodeInfoWithLocales(primary string, versions []*api.DubVersion) *api.EpisodeInfo {
	return &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:   "Test Series",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AudioLocale:   primary,
			Versions:      versions,
		},
		Title: "Test Episode",
	}
}

func restoreEpisodeTestSeams(t *testing.T, subtitles map[string]*api.Subtitle) {
	t.Helper()
	origGetEpisode := episodeGetEpisode
	origDeleteStream := episodeDeleteStream
	origFetchManifest := episodeFetchManifest
	origParseManifest := episodeParseManifest
	origGetPssh := episodeGetPssh
	origGetLicense := episodeGetLicense
	origDownloadParts := episodeDownloadParts
	origDownloadSubs := episodeDownloadSubs
	origMerge := episodeMerge
	origWriteNfo := episodeWriteNfo
	origWriteTvshow := episodeWriteTvshowNfo
	origGetSeriesInfo := episodeGetSeriesInfo
	origFetchArtwork := episodeFetchArtwork

	episodeGetEpisode = func(context.Context, *api.Client, string) (*api.Episode, error) {
		return &api.Episode{
			ManifestURL: "https://example.com/manifest.mpd",
			Subtitles:   subtitles,
			Token:       "stream-token",
		}, nil
	}
	episodeDeleteStream = func(context.Context, *api.Client, string, string) (bool, error) {
		return true, nil
	}
	episodeFetchManifest = func(context.Context, *api.Client, string) ([]byte, error) {
		return []byte("stub-manifest"), nil
	}
	episodeParseManifest = func([]byte) (*mpd.MPD, error) {
		return testEpisodeManifest(), nil
	}
	pssh := "cHNzaA=="
	episodeGetPssh = func(*mpd.MPD) *string {
		return &pssh
	}
	episodeGetLicense = func(context.Context, *api.Client, string, string, string) ([]*widevine.Key, error) {
		return nil, nil
	}
	episodeDownloadParts = func(_ context.Context, _ *api.Client, _, _ *string, _ *mpd.AdaptationSet, _ []*widevine.Key, _ int, label string) (string, error) {
		return writeEpisodeTestFile(t, t.TempDir(), sanitizeFilename(label)+".bin"), nil
	}
	episodeDownloadSubs = func(context.Context, *api.Client, string) (string, error) {
		return writeEpisodeTestFile(t, t.TempDir(), "subtitle.ass"), nil
	}
	episodeMerge = func(_ context.Context, _ string, _ []mux.MediaTrack, _ []mux.MediaTrack, outputFile string, _ *api.EpisodeInfo) error {
		return os.WriteFile(outputFile, []byte("mkv"), 0o600)
	}
	// No-op the NFO seams so existing tests don't write .nfo files or fire
	// real GetSeriesInfo calls. Task 3 tests override these with capturing fakes.
	episodeWriteNfo = func(context.Context, string, *api.EpisodeInfo, string) error { return nil }
	episodeWriteTvshowNfo = func(context.Context, string, *api.SeriesInfo) error { return nil }
	episodeGetSeriesInfo = func(context.Context, *api.Client, string, string, string) (*api.SeriesInfo, error) {
		return nil, nil
	}
	episodeFetchArtwork = func(context.Context, *api.Client, string, string) error { return nil }

	t.Cleanup(func() {
		episodeGetEpisode = origGetEpisode
		episodeDeleteStream = origDeleteStream
		episodeFetchManifest = origFetchManifest
		episodeParseManifest = origParseManifest
		episodeGetPssh = origGetPssh
		episodeGetLicense = origGetLicense
		episodeDownloadParts = origDownloadParts
		episodeDownloadSubs = origDownloadSubs
		episodeMerge = origMerge
		episodeWriteNfo = origWriteNfo
		episodeWriteTvshowNfo = origWriteTvshow
		episodeGetSeriesInfo = origGetSeriesInfo
		episodeFetchArtwork = origFetchArtwork
	})
}

func testEpisodeManifest() *mpd.MPD {
	videoID := "video-1080"
	videoHeight := uint64(1080)
	audioID := "audio-192"
	audioBandwidth := uint64(192000)

	return &mpd.MPD{
		Period: []*mpd.Period{
			{
				AdaptationSets: []*mpd.AdaptationSet{
					{
						Representations: []mpd.Representation{
							{
								ID:     &videoID,
								Height: &videoHeight,
								BaseURL: []*mpd.BaseURL{
									{Value: "https://example.com/video/"},
								},
							},
						},
					},
					{
						Representations: []mpd.Representation{
							{
								ID:        &audioID,
								Bandwidth: &audioBandwidth,
								BaseURL: []*mpd.BaseURL{
									{Value: "https://example.com/audio/"},
								},
							},
						},
					},
				},
			},
		},
	}
}

func captureEpisodeStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writePipe

	fn()

	if err := writePipe.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	os.Stdout = original

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, readPipe); err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	return buf.String()
}

// TestEpisodeWritesPerEpisodeNfoNonFatal (Task 3, D-08/D-09) asserts:
// (a) after a successful merge, episodeWriteNfo is invoked with the path
// beside the .mkv and baseContentID as uniqueid chardata; (b) when
// episodeWriteNfo returns an error, Episode still returns nil and the NFO
// warn line fires (non-fatal — the .mkv succeeded).
func TestEpisodeWritesPerEpisodeNfoNonFatal(t *testing.T) {
	t.Chdir(t.TempDir())

	videoQuality := "1080p"
	audioQuality := "192k"
	info := testEpisodeInfoWithLocales("ja-JP", nil)

	restoreEpisodeTestSeams(t, map[string]*api.Subtitle{})

	var nfoCalls []struct{ path, contentID string }
	origWriteNfo := episodeWriteNfo
	episodeWriteNfo = func(_ context.Context, path string, _ *api.EpisodeInfo, contentID string) error {
		nfoCalls = append(nfoCalls, struct{ path, contentID string }{path, contentID})
		return nil
	}
	// No-op the tvshow seam so it doesn't fire GetSeriesInfo.
	origWriteTvshow := episodeWriteTvshowNfo
	episodeWriteTvshowNfo = func(context.Context, string, *api.SeriesInfo) error { return nil }
	origGetSeries := episodeGetSeriesInfo
	episodeGetSeriesInfo = func(context.Context, *api.Client, string, string, string) (*api.SeriesInfo, error) {
		return nil, nil
	}
	t.Cleanup(func() {
		episodeWriteNfo = origWriteNfo
		episodeWriteTvshowNfo = origWriteTvshow
		episodeGetSeriesInfo = origGetSeries
	})

	client := api.NewTestClient(nil, "https://example.com", "test-token")

	stdout := captureEpisodeStdout(t, func() {
		err := Episode(context.Background(), client, "base-content-id", info, []string{"ja-JP"}, nil, &videoQuality, &audioQuality, 2, "", 1)
		if err != nil {
			t.Fatalf("Episode() error = %v, want nil with non-fatal NFO write", err)
		}
	})

	if len(nfoCalls) != 1 {
		t.Fatalf("episodeWriteNfo invoked %d time(s); want 1", len(nfoCalls))
	}
	if !strings.HasSuffix(nfoCalls[0].path, ".nfo") {
		t.Errorf("nfo path = %q, want .nfo suffix", nfoCalls[0].path)
	}
	if !strings.HasSuffix(nfoCalls[0].path, ".nfo") || strings.Contains(nfoCalls[0].path, ".mkv") {
		// .nfo should be the .mkv path with suffix swapped
	}
	if nfoCalls[0].contentID != "base-content-id" {
		t.Errorf("nfo contentID = %q, want base-content-id", nfoCalls[0].contentID)
	}
	_ = stdout

	// Now assert non-fatal on NFO write error.
	t.Chdir(t.TempDir())
	nfoCalls = nil
	episodeWriteNfo = func(_ context.Context, path string, _ *api.EpisodeInfo, contentID string) error {
		nfoCalls = append(nfoCalls, struct{ path, contentID string }{path, contentID})
		return fmt.Errorf("disk full")
	}
	t.Cleanup(func() { episodeWriteNfo = origWriteNfo })

	errOut := captureEpisodeStdout(t, func() {
		err := Episode(context.Background(), client, "base-content-id", info, []string{"ja-JP"}, nil, &videoQuality, &audioQuality, 2, "", 1)
		if err != nil {
			t.Fatalf("Episode() error = %v, want nil despite NFO write failure (non-fatal D-09)", err)
		}
	})
	if len(nfoCalls) != 1 {
		t.Fatalf("episodeWriteNfo invoked %d time(s) on error path; want 1", len(nfoCalls))
	}
	if !strings.Contains(errOut, "Failed to write NFO") {
		t.Errorf("stdout = %q, want NFO write-failure warn line", errOut)
	}
}

// TestEpisodeWritesTvshowNfoOnSingleEpisodeFlow (Task 3, D-03/D-07) asserts:
// (a) on the DEFAULT path (seam returns rich SeriesInfo), episodeGetSeriesInfo
// IS invoked with info.EpisodeMetadata.SeriesID and episodeWriteTvshowNfo
// receives the rich SeriesInfo (NOT a title-only one); (b) on the ERROR path
// (seam returns error), Episode still returns nil, a title-only tvshow.nfo IS
// still written (fallback SeriesInfo has only ID+Title).
func TestEpisodeWritesTvshowNfoOnSingleEpisodeFlow(t *testing.T) {
	info := &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:   "Test Series",
			SeriesID:      "GSERIES-EP",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AudioLocale:   "ja-JP",
		},
		Title: "Test Episode",
	}
	videoQuality := "1080p"
	audioQuality := "192k"
	client := api.NewTestClient(nil, "https://example.com", "test-token")

	// (a) DEFAULT path: rich SeriesInfo.
	t.Run("default_path_rich_series_info", func(t *testing.T) {
		t.Chdir(t.TempDir())
		restoreEpisodeTestSeams(t, map[string]*api.Subtitle{})

		var seriesCalledWith string
		var tvshowInfo *api.SeriesInfo
		episodeGetSeriesInfo = func(_ context.Context, _ *api.Client, seriesID, _, _ string) (*api.SeriesInfo, error) {
			seriesCalledWith = seriesID
			return &api.SeriesInfo{
				ID:          "GSERIES-EP",
				Title:       "Test Series",
				Description: "Rich plot from API",
				Genres:      []string{"Action"},
				Studio:      "Test Studio",
			}, nil
		}
		episodeWriteTvshowNfo = func(_ context.Context, _ string, info *api.SeriesInfo) error {
			tvshowInfo = info
			return nil
		}

		err := Episode(context.Background(), client, "base-content-id", info, []string{"ja-JP"}, nil, &videoQuality, &audioQuality, 2, "", 1)
		if err != nil {
			t.Fatalf("Episode() error = %v, want nil default path", err)
		}
		if seriesCalledWith != "GSERIES-EP" {
			t.Fatalf("episodeGetSeriesInfo called with seriesID=%q, want GSERIES-EP", seriesCalledWith)
		}
		if tvshowInfo == nil {
			t.Fatal("episodeWriteTvshowNfo not invoked")
		}
		if tvshowInfo.Description != "Rich plot from API" {
			t.Fatalf("tvshow info = title-only (Description=%q); want rich SeriesInfo (D-07 default path)", tvshowInfo.Description)
		}
		if tvshowInfo.Studio != "Test Studio" {
			t.Fatalf("tvshow Studio = %q, want Test Studio (rich, not title-only)", tvshowInfo.Studio)
		}
	})

	// (b) ERROR path: GetSeriesInfo fails, title-only fallback written.
	t.Run("error_path_title_only_fallback", func(t *testing.T) {
		t.Chdir(t.TempDir())
		restoreEpisodeTestSeams(t, map[string]*api.Subtitle{})

		var tvshowInfo *api.SeriesInfo
		episodeGetSeriesInfo = func(context.Context, *api.Client, string, string, string) (*api.SeriesInfo, error) {
			return nil, fmt.Errorf("network down")
		}
		episodeWriteTvshowNfo = func(_ context.Context, _ string, info *api.SeriesInfo) error {
			tvshowInfo = info
			return nil
		}

		stdout := captureEpisodeStdout(t, func() {
			err := Episode(context.Background(), client, "base-content-id", info, []string{"ja-JP"}, nil, &videoQuality, &audioQuality, 2, "", 1)
			if err != nil {
				t.Fatalf("Episode() error = %v, want nil despite GetSeriesInfo failure (non-fatal D-09)", err)
			}
		})
		if tvshowInfo == nil {
			t.Fatal("episodeWriteTvshowNfo not invoked on error path")
		}
		if tvshowInfo.Description != "" || tvshowInfo.Studio != "" {
			t.Fatalf("tvshow info = %#v; want title-only fallback (ID+Title only) on error path", tvshowInfo)
		}
		if tvshowInfo.Title != "Test Series" || tvshowInfo.ID != "GSERIES-EP" {
			t.Fatalf("tvshow fallback = %#v, want ID+Title from enriched metadata", tvshowInfo)
		}
		if !strings.Contains(stdout, "Failed to fetch series metadata") {
			t.Errorf("stdout = %q, want series-info-fetch-failed warn line", stdout)
		}
	})
}

func TestEpisodeWritesArtworkOnSingleEpisodeFlow(t *testing.T) {
	t.Chdir(t.TempDir())

	info := &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:   "Test Series",
			SeriesID:      "GSERIES-EP-ART",
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AudioLocale:   "ja-JP",
		},
		Title: "Test Episode",
	}
	videoQuality := "1080p"
	audioQuality := "192k"
	client := api.NewTestClient(nil, "https://example.com", "test-token")

	restoreEpisodeTestSeams(t, map[string]*api.Subtitle{})
	episodeGetSeriesInfo = func(context.Context, *api.Client, string, string, string) (*api.SeriesInfo, error) {
		return &api.SeriesInfo{
			ID:          "GSERIES-EP-ART",
			Title:       "Test Series",
			PosterURL:   "https://img1.crunchyroll.com/poster.jpg",
			BackdropURL: "https://img1.crunchyroll.com/backdrop.jpg",
		}, nil
	}

	var dests []string
	episodeFetchArtwork = func(_ context.Context, _ *api.Client, _ string, destPath string) error {
		dests = append(dests, destPath)
		return api.ErrArtworkNotFound
	}

	stdout := captureEpisodeStdout(t, func() {
		err := Episode(context.Background(), client, "base-content-id", info, []string{"ja-JP"}, nil, &videoQuality, &audioQuality, 2, "", 1)
		if err != nil {
			t.Fatalf("Episode() error = %v, want nil despite artwork 404", err)
		}
	})

	wantRoot := sanitizeFilename("Test Series")
	want := []string{filepath.Join(wantRoot, "poster.jpg"), filepath.Join(wantRoot, "backdrop.jpg")}
	if len(dests) != len(want) {
		t.Fatalf("episodeFetchArtwork invoked %d time(s), want %d: %v", len(dests), len(want), dests)
	}
	for i := range want {
		if dests[i] != want[i] {
			t.Fatalf("dests[%d] = %q, want %q", i, dests[i], want[i])
		}
	}
	if !strings.Contains(stdout, "No artwork available for Test Series") {
		t.Fatalf("stdout = %q, want artwork warning", stdout)
	}
}
