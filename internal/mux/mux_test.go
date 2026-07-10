package mux

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"crunchyroll-downloader/internal/api"
	"crunchyroll-downloader/internal/locale"
)

func TestMergeEverythingReturnsErrorAndRemovesPartialOutputOnFFmpegFailure(t *testing.T) {
	restoreFFmpegCommand(t, "1", "bad mux")

	dir := t.TempDir()
	videoFile := writeTempFile(t, dir, "video.mp4")
	audioFile := writeTempFile(t, dir, "audio.mp3")
	outputFile := writeTempFile(t, dir, "partial.mkv")

	err := MergeEverything(context.Background(), videoFile, []MediaTrack{{File: audioFile, Locale: "ja-JP"}}, nil, outputFile, testEpisodeInfo())
	if err == nil {
		t.Fatal("MergeEverything() error = nil, want ffmpeg error")
	}
	if !strings.Contains(err.Error(), "ffmpeg failed") {
		t.Fatalf("MergeEverything() error = %q, want ffmpeg failure", err)
	}
	if _, statErr := os.Stat(outputFile); !os.IsNotExist(statErr) {
		t.Fatalf("partial output still exists after ffmpeg failure; stat error = %v", statErr)
	}
}

func TestMergeEverythingWarnsButSucceedsWhenCleanupFails(t *testing.T) {
	restoreFFmpegCommand(t, "0", "")

	dir := t.TempDir()
	absentOptionalVideoFile := ""
	audioFile := writeTempFile(t, dir, "audio.mp3")
	outputFile := filepath.Join(dir, "output.mkv")

	stdout := captureStdout(t, func() {
		err := MergeEverything(context.Background(), absentOptionalVideoFile, []MediaTrack{{File: audioFile, Locale: "ja-JP"}}, nil, outputFile, testEpisodeInfo())
		if err != nil {
			t.Fatalf("MergeEverything() error = %v, want nil despite cleanup warning", err)
		}
	})

	if !strings.Contains(stdout, "Failed to remove temporary file") {
		t.Fatalf("MergeEverything() stdout = %q, want cleanup warning", stdout)
	}
}

func TestMergeEverythingKillsFFmpegAndRemovesPartialOutputOnCancellation(t *testing.T) {
	restoreFFmpegCommand(t, "0", "")

	dir := t.TempDir()
	videoFile := writeTempFile(t, dir, "video.mp4")
	audioFile := writeTempFile(t, dir, "audio.mp3")
	outputFile := writeTempFile(t, dir, "partial.mkv")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := MergeEverything(ctx, videoFile, []MediaTrack{{File: audioFile, Locale: "ja-JP"}}, nil, outputFile, testEpisodeInfo())
	if err == nil {
		t.Fatal("MergeEverything() error = nil, want cancellation error")
	}
	if _, statErr := os.Stat(outputFile); !os.IsNotExist(statErr) {
		t.Fatalf("partial output still exists after cancellation; stat error = %v", statErr)
	}
}

func TestMergeEverythingRejectsEmptyVideo(t *testing.T) {
	var invoked bool
	restoreFFmpegCommandSentinel(t, &invoked)

	dir := t.TempDir()
	videoFile := writeEmptyFile(t, dir, "video.mp4")
	audioFile := writeTempFile(t, dir, "audio.mp3")
	outputFile := filepath.Join(dir, "output.mkv")

	err := MergeEverything(context.Background(), videoFile, []MediaTrack{{File: audioFile, Locale: "ja-JP"}}, nil, outputFile, testEpisodeInfo())
	if err == nil {
		t.Fatal("MergeEverything() error = nil, want empty input error")
	}
	if !strings.Contains(err.Error(), "is empty (0 bytes)") {
		t.Fatalf("MergeEverything() error = %q, want empty input error", err)
	}
	if invoked {
		t.Fatal("ffmpeg was invoked on empty video input")
	}
}

func TestMergeEverythingRejectsEmptyAudioTrack(t *testing.T) {
	var invoked bool
	restoreFFmpegCommandSentinel(t, &invoked)

	dir := t.TempDir()
	videoFile := writeTempFile(t, dir, "video.mp4")
	audioFile := writeEmptyFile(t, dir, "audio.mp3")
	outputFile := filepath.Join(dir, "output.mkv")

	err := MergeEverything(context.Background(), videoFile, []MediaTrack{{File: audioFile, Locale: "ja-JP"}}, nil, outputFile, testEpisodeInfo())
	if err == nil {
		t.Fatal("MergeEverything() error = nil, want empty input error")
	}
	if !strings.Contains(err.Error(), "is empty (0 bytes)") {
		t.Fatalf("MergeEverything() error = %q, want empty input error", err)
	}
	if invoked {
		t.Fatal("ffmpeg was invoked on empty audio input")
	}
}

func TestMergeEverythingRejectsMissingInput(t *testing.T) {
	var invoked bool
	restoreFFmpegCommandSentinel(t, &invoked)

	dir := t.TempDir()
	videoFile := filepath.Join(dir, "missing-video.mp4")
	outputFile := filepath.Join(dir, "output.mkv")

	err := MergeEverything(context.Background(), videoFile, nil, nil, outputFile, testEpisodeInfo())
	if err == nil {
		t.Fatal("MergeEverything() error = nil, want missing input error")
	}
	if !strings.Contains(err.Error(), "mux input") {
		t.Fatalf("MergeEverything() error = %q, want mux input error", err)
	}
	if invoked {
		t.Fatal("ffmpeg was invoked on missing input")
	}
}

// restoreFFmpegCommandWithArgCapture swaps ffmpegCommand for the
// TestHelperProcess subprocess seam (same exit/stderr contract as
// restoreFFmpegCommand) and additionally captures the assembled FFmpeg args to
// the file at argsFile. The parent test reads argsFile after MergeEverything
// returns to assert on metadata arg values without invoking a real ffmpeg.
func restoreFFmpegCommandWithArgCapture(t *testing.T, exitCode, stderr, argsFile string) {
	t.Helper()
	original := ffmpegCommand
	ffmpegCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			"GO_HELPER_EXIT_CODE="+exitCode,
			"GO_HELPER_STDERR="+stderr,
			"GO_HELPER_ARGS_FILE="+argsFile,
		)
		return cmd
	}
	t.Cleanup(func() {
		ffmpegCommand = original
	})
}

// TestMergeEverythingSetsCorrectSeasonNumber is the D-05 regression test
// (OUT-03): the FFmpeg `-metadata:g season_number=` arg must equal
// info.EpisodeMetadata.SeasonNumber, NOT EpisodeNumber. SeasonNumber and
// EpisodeNumber are set to DISTINCT values (2 and 7) so a swap of the two
// fields is detectable — the pre-fix line stamped season_number=7 (episode
// number), the post-fix line stamps season_number=2 (season number).
func TestMergeEverythingSetsCorrectSeasonNumber(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	restoreFFmpegCommandWithArgCapture(t, "0", "", argsFile)

	videoFile := writeTempFile(t, dir, "video.mp4")
	outputFile := filepath.Join(dir, "output.mkv")

	// Distinct SeasonNumber (2) and EpisodeNumber (7) make the bug
	// detectable: pre-fix season_number arg = "7", post-fix = "2".
	info := &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:   "Test Series",
			SeasonNumber:  2,
			EpisodeNumber: 7,
		},
		Title: "Test Episode",
	}

	if err := MergeEverything(context.Background(), videoFile, nil, nil, outputFile, info); err != nil {
		t.Fatalf("MergeEverything() error = %v, want nil", err)
	}

	argsBytes, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read captured args file: %v", err)
	}
	args := string(argsBytes)
	lines := strings.Split(args, "\n")

	// The season_number metadata value must equal SeasonNumber (2), derived
	// from info.EpisodeMetadata.SeasonNumber. The buggy pre-fix value (7,
	// from EpisodeNumber) MUST be absent.
	var seasonNumberVal string
	for i, ln := range lines {
		if ln == "-metadata:g" && i+1 < len(lines) && strings.HasPrefix(lines[i+1], "season_number=") {
			seasonNumberVal = strings.TrimPrefix(lines[i+1], "season_number=")
			break
		}
	}
	if seasonNumberVal != "2" {
		t.Fatalf("season_number metadata arg = %q, want %q (SeasonNumber); full args:\n%s", seasonNumberVal, "2", args)
	}

	// Guard against the bug regressing: the episode number (7) must never
	// appear as a season_number value.
	if strings.Contains(args, "season_number=7") {
		t.Fatalf("season_number arg carries EpisodeNumber (7) — the v1.0 bug is present; full args:\n%s", args)
	}

	// Sanity: track still legitimately uses EpisodeNumber, and the global
	// title embeds both distinct numbers (S02E07). Pinning these catches
	// accidental edits to the sibling args on lines 104-110.
	if !strings.Contains(args, "track=7") {
		t.Fatalf("track metadata arg should be EpisodeNumber (7); full args:\n%s", args)
	}
	if !strings.Contains(args, "title=S02E07 - Test Episode") {
		t.Fatalf("title metadata arg should embed S02E07; full args:\n%s", args)
	}
}

func restoreFFmpegCommand(t *testing.T, exitCode, stderr string) {
	t.Helper()
	original := ffmpegCommand
	ffmpegCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			"GO_HELPER_EXIT_CODE="+exitCode,
			"GO_HELPER_STDERR="+stderr,
		)
		return cmd
	}
	t.Cleanup(func() {
		ffmpegCommand = original
	})
}

func restoreFFmpegCommandSentinel(t *testing.T, invoked *bool) {
	t.Helper()
	original := ffmpegCommand
	ffmpegCommand = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		*invoked = true
		t.Fatal("ffmpeg should not be invoked on invalid mux input")
		return exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperProcess")
	}
	t.Cleanup(func() {
		ffmpegCommand = original
	})
}

func TestTrackTitle(t *testing.T) {
	tests := []struct {
		locale string
		want   string
	}{
		{locale: "ja-JP", want: "日本語"},
		{locale: "en-US", want: "English"},
		{locale: "xx-XX", want: "xx-XX"},
	}
	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			got := TrackTitle(tt.locale)
			if got != tt.want {
				t.Fatalf("TrackTitle(%q) = %q, want %q", tt.locale, got, tt.want)
			}
		})
	}
}

func TestTrackTitleAllLocales(t *testing.T) {
	for code := range locale.LanguageNames {
		title := TrackTitle(code)
		if title == "" {
			t.Fatalf("TrackTitle(%q) returned empty string", code)
		}
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if msg := os.Getenv("GO_HELPER_STDERR"); msg != "" {
		_, _ = os.Stderr.WriteString(msg)
	}
	// Arg capture: if GO_HELPER_ARGS_FILE is set, write the ffmpeg args (one
	// per line) to that path so the parent test can assert on the assembled
	// metadata args. This is the args-capture seam for regression tests that
	// need to inspect what MergeEverything passed to FFmpeg without invoking a
	// real ffmpeg binary.
	if argsFile := os.Getenv("GO_HELPER_ARGS_FILE"); argsFile != "" {
		// args[0] is the command name ("ffmpeg"); write the trailing args.
		_ = os.WriteFile(argsFile, []byte(strings.Join(os.Args[2:], "\n")), 0644)
	}
	if sleep := os.Getenv("GO_HELPER_SLEEP"); sleep != "" {
		duration, err := time.ParseDuration(sleep)
		if err != nil {
			os.Exit(2)
		}
		time.Sleep(duration)
	}
	if os.Getenv("GO_HELPER_EXIT_CODE") == "0" {
		os.Exit(0)
	}
	os.Exit(1)
}

func writeTempFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func writeEmptyFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	return path
}

func testEpisodeInfo() *api.EpisodeInfo {
	return &api.EpisodeInfo{
		EpisodeMetadata: api.EpisodeMetadata{
			SeriesTitle:   "Test Series",
			SeasonNumber:  1,
			EpisodeNumber: 1,
		},
		Title: "Test Episode",
	}
}

func captureStdout(t *testing.T, fn func()) string {
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
