package mux

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"

	"crunchyroll-downloader/internal/api"
	"crunchyroll-downloader/internal/diag"
	loc "crunchyroll-downloader/internal/locale"
	"crunchyroll-downloader/internal/output"
)

type MediaTrack struct {
	File   string
	Locale string
}

var ffmpegCommand = exec.CommandContext

func TrackTitle(code string) string {
	if name, ok := loc.LanguageNames[code]; ok {
		return name
	}
	return code
}

func MergeEverything(ctx context.Context, videoFile string, audioTracks, subTracks []MediaTrack, outputFile string, info *api.EpisodeInfo) error {
	if ctx == nil {
		ctx = context.Background()
	}

	inputs := []string{videoFile}
	for _, audio := range audioTracks {
		inputs = append(inputs, audio.File)
	}
	for _, sub := range subTracks {
		inputs = append(inputs, sub.File)
	}
	for _, path := range inputs {
		if path == "" {
			continue
		}
		fi, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("mux input %s: %w", path, err)
		}
		if fi.Size() == 0 {
			return fmt.Errorf("mux input %s is empty (0 bytes): refusing to invoke FFmpeg", path)
		}
	}

	args := []string{"-i", videoFile}
	for _, audio := range audioTracks {
		args = append(args, "-i", audio.File)
	}
	for _, sub := range subTracks {
		args = append(args, "-i", sub.File)
	}

	args = append(args, "-map", "0:v:0")
	for i := range audioTracks {
		args = append(args, "-map", fmt.Sprintf("%d:a:0", 1+i))
	}
	for j := range subTracks {
		args = append(args, "-map", fmt.Sprintf("%d", 1+len(audioTracks)+j))
	}

	args = append(args, "-c:v", "copy", "-c:a", "copy")
	if len(subTracks) > 0 {
		args = append(args, "-c:s", "copy")
	}

	for i, audio := range audioTracks {
		args = append(args,
			fmt.Sprintf("-metadata:s:a:%d", i), "language="+loc.LanguageCodes[audio.Locale],
			fmt.Sprintf("-metadata:s:a:%d", i), "title="+TrackTitle(audio.Locale),
		)
	}
	for j, sub := range subTracks {
		args = append(args,
			fmt.Sprintf("-metadata:s:s:%d", j), "language="+loc.LanguageCodes[sub.Locale],
			fmt.Sprintf("-metadata:s:s:%d", j), "title="+TrackTitle(sub.Locale),
		)
	}

	for i := range audioTracks {
		disposition := "0"
		if i == 0 {
			disposition = "default"
		}
		args = append(args, fmt.Sprintf("-disposition:a:%d", i), disposition)
	}
	for j := range subTracks {
		disposition := "0"
		if j == 0 {
			disposition = "default"
		}
		args = append(args, fmt.Sprintf("-disposition:s:%d", j), disposition)
	}

	args = append(args,
		"-metadata:g", "title="+fmt.Sprintf("S%02vE%02v - %s", info.EpisodeMetadata.SeasonNumber, info.EpisodeMetadata.EpisodeNumber, info.Title),
		"-metadata:g", "show="+info.EpisodeMetadata.SeriesTitle,
		"-metadata:g", "track="+fmt.Sprintf("%v", info.EpisodeMetadata.EpisodeNumber),
		"-metadata:g", "season_number="+fmt.Sprintf("%v", info.EpisodeMetadata.SeasonNumber),
		outputFile,
	)

	cmd := ffmpegCommand(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		cleanupErr := os.Remove(outputFile)
		if cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			return fmt.Errorf("ffmpeg failed: %w: %s; cleanup output: %v", err, stderr.String(), cleanupErr)
		}
		return fmt.Errorf("ffmpeg failed: %w: %s", err, stderr.String())
	}
	if diag.MuxLogger != nil {
		diag.MuxLogger.Info("ffmpeg finished", "stderr", stderr.String())
	}

	warnRemove(videoFile)
	for _, audio := range audioTracks {
		warnRemove(audio.File)
	}
	for _, sub := range subTracks {
		warnRemove(sub.File)
	}

	output.Global.Info("\nDownload finished! Output file: %s\n", outputFile)
	return nil
}

func warnRemove(path string) {
	if err := os.Remove(path); err != nil {
		output.Global.Warn("Failed to remove temporary file %s: %v", path, err)
	}
}
