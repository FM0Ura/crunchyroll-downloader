package download

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
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

	// Verify the series subfolder was created inside outputDir
	expectedDir := filepath.Join(outputDir, sanitizeFilename("Test Series"))
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Fatalf("Episode() did not create series subfolder at %s", expectedDir)
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
