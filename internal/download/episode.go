package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"crunchyroll-downloader/internal/api"
	"crunchyroll-downloader/internal/diag"
	"crunchyroll-downloader/internal/drm"
	"crunchyroll-downloader/internal/media"
	"crunchyroll-downloader/internal/mux"
	"crunchyroll-downloader/internal/nfo"
	"crunchyroll-downloader/internal/output"
	"github.com/iyear/gowidevine"
	"github.com/unki2aut/go-mpd"
	"golang.org/x/sync/errgroup"
)

var multiUnderscore = regexp.MustCompile(`_{2,}`)

var (
	episodeGetEpisode = func(ctx context.Context, client *api.Client, id string) (*api.Episode, error) {
		return client.GetEpisode(ctx, id)
	}
	episodeDeleteStream = func(ctx context.Context, client *api.Client, id, token string) (bool, error) {
		return client.DeleteStream(ctx, id, token)
	}
	episodeFetchManifest = func(ctx context.Context, client *api.Client, url string) ([]byte, error) {
		return client.FetchManifest(ctx, url)
	}
	episodeParseManifest = media.ParseManifest
	episodeGetPssh       = drm.GetPssh
	episodeGetLicense    = drm.GetLicense
	episodeDownloadParts = func(ctx context.Context, client *api.Client, baseURL, representationID *string, set *mpd.AdaptationSet, keys []*widevine.Key, workers int, streamLabel string) (string, error) {
		return media.DownloadParts(ctx, client, baseURL, representationID, set, keys, workers, streamLabel)
	}
	episodeDownloadSubs = func(ctx context.Context, client *api.Client, url string) (string, error) {
		return media.DownloadSubs(ctx, client, url)
	}
	episodeMerge = mux.MergeEverything
	// NFO seams (Phase 7): package-level indirection so download/episode.go tests
	// can stub the emitter + the GetSeriesInfo call without a live HTTP server.
	// Defaults point at the real functions; tests override via assignment +
	// t.Cleanup restore (mirroring episodeMerge).
	episodeWriteNfo       = nfo.WriteEpisode
	episodeWriteTvshowNfo = nfo.WriteTVShow
	episodeGetSeriesInfo  = func(ctx context.Context, client *api.Client, seriesID, audioLocale, subLocale string) (*api.SeriesInfo, error) {
		return client.GetSeriesInfo(ctx, seriesID, audioLocale, subLocale)
	}
	episodeFetchArtwork = func(ctx context.Context, client *api.Client, artworkURL, destPath string) error {
		return client.FetchArtwork(ctx, artworkURL, destPath)
	}
)

func sanitizeFilename(s string) string {
	if s == "" {
		return "Unknown"
	}
	illegal := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|", "'", "’", "`", "“", "”"}
	res := s
	for _, char := range illegal {
		res = strings.ReplaceAll(res, char, "_")
	}
	res = multiUnderscore.ReplaceAllString(res, "_")
	return strings.TrimRight(res, " .")
}

func Episode(ctx context.Context, client *api.Client, baseContentID string, info *api.EpisodeInfo, audioLangs, subsLangs []string, videoQuality, audioQuality *string, workers int, outputDir string, totalEpisodes int) error {
	cleanSeriesTitle := sanitizeFilename(info.EpisodeMetadata.SeriesTitle)
	cleanEpisodeTitle := sanitizeFilename(info.Title)

	outputBase := cleanSeriesTitle
	if outputDir != "" {
		outputBase = filepath.Join(outputDir, cleanSeriesTitle)
	}

	if err := os.MkdirAll(outputBase, 0777); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	seasonDir := fmt.Sprintf("Season %02d", info.EpisodeMetadata.SeasonNumber)
	seasonPath := filepath.Join(outputBase, seasonDir)
	if err := os.MkdirAll(seasonPath, 0777); err != nil {
		return fmt.Errorf("creating season directory: %w", err)
	}

	outputFile := filepath.Join(seasonPath, fmt.Sprintf("%s S%02dE%02d - %s.mkv",
		cleanSeriesTitle,
		info.EpisodeMetadata.SeasonNumber,
		info.EpisodeMetadata.EpisodeNumber,
		cleanEpisodeTitle,
	))

	if _, err := os.Stat(outputFile); err == nil {
		output.Global.Info("[Episode %d/%d] %s ... skipped (already downloaded)", info.EpisodeMetadata.EpisodeNumber, totalEpisodes, info.Title)
		return nil
	}

	guidByLocale := map[string]string{}
	if info.EpisodeMetadata.AudioLocale != "" {
		guidByLocale[info.EpisodeMetadata.AudioLocale] = baseContentID
	}
	for _, v := range info.EpisodeMetadata.Versions {
		guidByLocale[v.AudioLocale] = v.GUID
	}

	if len(audioLangs) == 1 && audioLangs[0] == "all" {
		audioLangs = make([]string, 0, len(guidByLocale))
		if primaryLocale := info.EpisodeMetadata.AudioLocale; primaryLocale != "" {
			if _, ok := guidByLocale[primaryLocale]; ok {
				audioLangs = append(audioLangs, primaryLocale)
			}
		}
		for locale := range guidByLocale {
			if locale != info.EpisodeMetadata.AudioLocale {
				audioLangs = append(audioLangs, locale)
			}
		}
		if len(audioLangs) > 1 {
			sort.Strings(audioLangs[1:])
		}
	}

	type audioVersion struct {
		locale    string
		contentId string
	}
	var versions []audioVersion
	var skippedTracks []string
	for i, locale := range audioLangs {
		guid, ok := guidByLocale[locale]
		if !ok {
			if i == 0 {
				return fmt.Errorf("primary audio locale %s not available for episode %d", locale, info.EpisodeMetadata.EpisodeNumber)
			}
			output.Global.Warn("Skipping %s audio: not available for episode %d", locale, info.EpisodeMetadata.EpisodeNumber)
			if diag.DownloadLogger != nil {
				diag.DownloadLogger.Warn("skipped track", "episode", info.EpisodeMetadata.EpisodeNumber, "kind", "audio", "locale", locale)
			}
			skippedTracks = append(skippedTracks, locale+" dub")
			continue
		}
		versions = append(versions, audioVersion{locale: locale, contentId: guid})
	}
	if len(versions) == 0 {
		return fmt.Errorf("no audio tracks available for episode %d", info.EpisodeMetadata.EpisodeNumber)
	}

	output.Global.Info("[Episode %d/%d] %s (S%02dE%02d) ...", info.EpisodeMetadata.EpisodeNumber, totalEpisodes, info.Title, info.EpisodeMetadata.SeasonNumber, info.EpisodeMetadata.EpisodeNumber)
	if diag.DownloadLogger != nil {
		diag.DownloadLogger.Info("episode_start", "ep", info.EpisodeMetadata.EpisodeNumber, "season", info.EpisodeMetadata.SeasonNumber)
	}

	activeStreams := map[string]string{}
	var tempFiles []string
	completed := false
	defer func() {
		output.Global.Debug("Cleaning up episode resources...")
		cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelCleanup()
		for id, sToken := range activeStreams {
			if _, err := episodeDeleteStream(cleanupCtx, client, id, sToken); err != nil {
				output.Global.Warn("Failed to remove stream %s: %v", id, err)
			}
		}
		if !completed {
			cleanupEpisodeArtifacts(outputFile, tempFiles)
		}
	}()

	episodeStart := time.Now()

	firstEpisode, err := episodeGetEpisode(ctx, client, versions[0].contentId)
	if err != nil {
		return fmt.Errorf("fetching first episode: %w", err)
	}
	activeStreams[versions[0].contentId] = firstEpisode.Token
	episodeByContentID := map[string]*api.Episode{
		versions[0].contentId: firstEpisode,
	}

	if len(subsLangs) == 1 && subsLangs[0] == "all" {
		subsLangs = make([]string, 0, len(firstEpisode.Subtitles))
		for locale, sub := range firstEpisode.Subtitles {
			if sub != nil && sub.URL != "" {
				subsLangs = append(subsLangs, locale)
			}
		}
		sort.Strings(subsLangs)
	}

	output.Global.Info("Audio locales: %s | Subtitle locales: %s", strings.Join(audioLangs, ", "), strings.Join(subsLangs, ", "))

	var subTracks []mux.MediaTrack
	seenSubURLs := map[string]bool{}
	for j, locale := range subsLangs {
		type subtitleCandidate struct {
			sub         *api.Subtitle
			sourceAudio string
			alternate   bool
		}
		var candidates []subtitleCandidate
		if sub := firstEpisode.Subtitles[locale]; sub != nil && sub.URL != "" {
			candidates = append(candidates, subtitleCandidate{sub: sub, sourceAudio: versions[0].locale})
			seenSubURLs[locale+"\x00"+sub.URL] = true
		}
		for _, version := range versions {
			if version.locale != locale || version.contentId == versions[0].contentId {
				continue
			}
			episode, err := episodeGetEpisode(ctx, client, version.contentId)
			if err != nil {
				output.Global.Warn("Skipping %s subtitles from %s audio: %v", locale, mux.TrackTitle(version.locale), err)
				if diag.DownloadLogger != nil {
					diag.DownloadLogger.Warn("skipped subtitle source", "episode", info.EpisodeMetadata.EpisodeNumber, "locale", locale, "audio_locale", version.locale, "err", err)
				}
				continue
			}
			activeStreams[version.contentId] = episode.Token
			episodeByContentID[version.contentId] = episode
			if sub := episode.Subtitles[locale]; sub != nil && sub.URL != "" {
				urlKey := locale + "\x00" + sub.URL
				if !seenSubURLs[urlKey] {
					candidates = append(candidates, subtitleCandidate{sub: sub, sourceAudio: version.locale, alternate: len(candidates) > 0})
					seenSubURLs[urlKey] = true
				}
			}
		}

		if len(candidates) == 0 {
			if j == 0 {
				return fmt.Errorf("primary subtitle locale %s not available for episode %d", locale, info.EpisodeMetadata.EpisodeNumber)
			}
			output.Global.Warn("Skipping %s subtitles: not available for episode %d", locale, info.EpisodeMetadata.EpisodeNumber)
			if diag.DownloadLogger != nil {
				diag.DownloadLogger.Warn("skipped track", "episode", info.EpisodeMetadata.EpisodeNumber, "kind", "subtitle", "locale", locale)
			}
			skippedTracks = append(skippedTracks, locale+" sub")
			continue
		}

		for _, candidate := range candidates {
			title := mux.TrackTitle(locale)
			if candidate.alternate {
				title = fmt.Sprintf("%s (%s audio)", title, mux.TrackTitle(candidate.sourceAudio))
			}
			output.Global.Info("Downloading subtitles for %s...", title)
			file, err := episodeDownloadSubs(ctx, client, candidate.sub.URL)
			if err != nil {
				return fmt.Errorf("downloading subtitles for %s: %w", locale, err)
			}
			tempFiles = append(tempFiles, file)
			subTracks = append(subTracks, mux.MediaTrack{File: file, Locale: locale, Title: title})
		}
	}
	if len(subTracks) > 0 {
		output.Global.Info("Downloaded subtitles!")
	}

	var videoFile string
	var audioTracks []mux.MediaTrack

	// ===== Phase A: Video + first audio (sequential, i==0) =====
	version := versions[0]

	var manifest *mpd.MPD
	manifestData, err := episodeFetchManifest(ctx, client, firstEpisode.ManifestURL)
	if err != nil {
		return fmt.Errorf("fetching manifest for %s: %w", version.locale, err)
	}

	manifest, err = episodeParseManifest(manifestData)
	if err != nil {
		return fmt.Errorf("parsing manifest for %s: %w", version.locale, err)
	}
	media.SetCachedManifest(version.contentId, manifest)

	pssh := episodeGetPssh(manifest)
	if pssh == nil {
		return fmt.Errorf("PSSH not found for %s", version.locale)
	}

	keys, err := episodeGetLicense(ctx, client, *pssh, version.contentId, firstEpisode.Token)
	if err != nil {
		return fmt.Errorf("getting license for %s: %w", version.locale, err)
	}

	audioSet := manifest.Period[0].AdaptationSets[1]
	output.Global.Info("Downloading %s audio...", mux.TrackTitle(version.locale))
	audioBaseUrl, audioRepresentationId := media.GetAudioBaseUrl(audioSet, *audioQuality)
	if audioBaseUrl == nil {
		return fmt.Errorf("failed to get audio base URL for %s", version.locale)
	}

	audioFile, err := episodeDownloadParts(ctx, client, audioBaseUrl, audioRepresentationId, audioSet, keys, workers, mux.TrackTitle(version.locale)+" audio")
	if err != nil {
		return fmt.Errorf("downloading audio for %s: %w", version.locale, err)
	}
	tempFiles = append(tempFiles, audioFile)
	audioTracks = append(audioTracks, mux.MediaTrack{File: audioFile, Locale: version.locale})

	// Download video (always first version)
	videoSet := manifest.Period[0].AdaptationSets[0]
	output.Global.Info("Downloading video...")
	baseUrl, representationId := media.GetVideoBaseUrl(videoSet, *videoQuality)
	if baseUrl == nil {
		return fmt.Errorf("failed to get video base URL")
	}
	videoFile, err = episodeDownloadParts(ctx, client, baseUrl, representationId, videoSet, keys, workers, "video")
	if err != nil {
		return fmt.Errorf("downloading video: %w", err)
	}
	tempFiles = append(tempFiles, videoFile)

	// ===== Phase B: Parallel audio (versions [1..N]) =====
	if len(versions) > 1 {
		g, gctx := errgroup.WithContext(ctx)
		var mu sync.Mutex

		for idx := 1; idx < len(versions); idx++ {
			idx := idx
			version := versions[idx]
			g.Go(func() error {
				manifest := media.GetCachedManifest(version.contentId)
				var episodeToken string
				var cachedEpisode *api.Episode
				mu.Lock()
				cachedEpisode = episodeByContentID[version.contentId]
				if cachedEpisode != nil {
					episodeToken = cachedEpisode.Token
				}
				mu.Unlock()
				if manifest == nil {
					episode := cachedEpisode
					if episode == nil {
						var err error
						episode, err = episodeGetEpisode(gctx, client, version.contentId)
						if err != nil {
							return fmt.Errorf("fetching episode for %s: %w", version.locale, err)
						}
						episodeToken = episode.Token

						mu.Lock()
						activeStreams[version.contentId] = episode.Token
						episodeByContentID[version.contentId] = episode
						mu.Unlock()
					}

					manifestData, err := episodeFetchManifest(gctx, client, episode.ManifestURL)
					if err != nil {
						return fmt.Errorf("fetching manifest for %s: %w", version.locale, err)
					}

					manifest, err = episodeParseManifest(manifestData)
					if err != nil {
						return fmt.Errorf("parsing manifest for %s: %w", version.locale, err)
					}
					media.SetCachedManifest(version.contentId, manifest)
				}

				pssh := episodeGetPssh(manifest)
				if pssh == nil {
					return fmt.Errorf("PSSH not found for %s", version.locale)
				}

				if episodeToken == "" {
					mu.Lock()
					episodeToken = activeStreams[version.contentId]
					mu.Unlock()
				}

				keys, err := episodeGetLicense(gctx, client, *pssh, version.contentId, episodeToken)
				if err != nil {
					return fmt.Errorf("getting license for %s: %w", version.locale, err)
				}

				audioSet := manifest.Period[0].AdaptationSets[1]
				mu.Lock()
				output.Global.Info("Downloading %s audio...", mux.TrackTitle(version.locale))
				mu.Unlock()
				audioBaseUrl, audioRepresentationId := media.GetAudioBaseUrl(audioSet, *audioQuality)
				if audioBaseUrl == nil {
					return fmt.Errorf("failed to get audio base URL for %s", version.locale)
				}

				audioFile, err := episodeDownloadParts(gctx, client, audioBaseUrl, audioRepresentationId, audioSet, keys, workers, mux.TrackTitle(version.locale)+" audio")
				if err != nil {
					return fmt.Errorf("downloading audio for %s: %w", version.locale, err)
				}

				mu.Lock()
				tempFiles = append(tempFiles, audioFile)
				audioTracks = append(audioTracks, mux.MediaTrack{File: audioFile, Locale: version.locale})
				mu.Unlock()
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return fmt.Errorf("audio versions: %w", err)
		}
	}

	// Phase C: Stream cleanup is handled by the deferred function above.
	// All activeStreams entries are released in the deferred DeleteStream loop.

	if err := episodeMerge(ctx, videoFile, audioTracks, subTracks, outputFile, info); err != nil {
		return fmt.Errorf("muxing episode: %w", err)
	}
	completed = true

	// D-08/D-09: write per-episode .nfo beside the .mkv (non-fatal).
	// The .nfo path is strings.TrimSuffix(outputFile, ".mkv") + ".nfo".
	// NFO write failure never aborts the download — warn on both channels.
	nfoPath := strings.TrimSuffix(outputFile, ".mkv") + ".nfo"
	if err := episodeWriteNfo(ctx, nfoPath, info, baseContentID); err != nil {
		output.Global.Warn("Failed to write NFO for %s: %v", info.Title, err)
		if diag.MuxLogger != nil {
			diag.MuxLogger.Warn("nfo_write_failed", "episode", info.EpisodeMetadata.EpisodeNumber, "path", nfoPath, "err", err)
		}
	}

	// D-03 + D-07 (LOCKED): single-episode tvshow.nfo at the series root.
	// os.Stat guard (D-02 resumability): a re-run skips the re-fetch.
	// DEFAULT path fires episodeGetSeriesInfo(info.EpisodeMetadata.SeriesID);
	// title-only fallback is written ONLY when GetSeriesInfo errors (D-09).
	tvshowNfoPath := filepath.Join(outputBase, "tvshow.nfo")
	seriesInfo := (*api.SeriesInfo)(nil)
	seriesInfoFetched := false
	fetchSeriesInfo := func() (*api.SeriesInfo, error) {
		if seriesInfoFetched {
			return seriesInfo, nil
		}
		seriesInfoFetched = true
		var err error
		seriesInfo, err = episodeGetSeriesInfo(ctx, client, info.EpisodeMetadata.SeriesID, "", "")
		return seriesInfo, err
	}
	if _, err := os.Stat(tvshowNfoPath); err != nil {
		seriesInfo, serr := fetchSeriesInfo()
		if serr != nil || seriesInfo == nil {
			if serr != nil {
				output.Global.Warn("Failed to fetch series metadata for %s: %v", info.EpisodeMetadata.SeriesTitle, serr)
				if diag.ApiLogger != nil {
					diag.ApiLogger.Warn("series_info_fetch_failed", "series", info.EpisodeMetadata.SeriesTitle, "err", serr)
				}
			} else {
				output.Global.Warn("No series metadata returned for %s", info.EpisodeMetadata.SeriesTitle)
			}
			// Title-only error-fallback (D-09): still write a tvshow.nfo so
			// Jellyfin/Kodi have SOMETHING; a write error here also non-fatal-warns.
			fallbackInfo := &api.SeriesInfo{
				ID:    info.EpisodeMetadata.SeriesID,
				Title: info.EpisodeMetadata.SeriesTitle,
			}
			if werr := episodeWriteTvshowNfo(ctx, tvshowNfoPath, fallbackInfo); werr != nil {
				output.Global.Warn("Failed to write tvshow.nfo for %s: %v", info.EpisodeMetadata.SeriesTitle, werr)
				if diag.MuxLogger != nil {
					diag.MuxLogger.Warn("tvshow_nfo_write_failed", "path", tvshowNfoPath, "err", werr)
				}
			}
		} else {
			if werr := episodeWriteTvshowNfo(ctx, tvshowNfoPath, seriesInfo); werr != nil {
				output.Global.Warn("Failed to write tvshow.nfo for %s: %v", info.EpisodeMetadata.SeriesTitle, werr)
				if diag.MuxLogger != nil {
					diag.MuxLogger.Warn("tvshow_nfo_write_failed", "path", tvshowNfoPath, "err", werr)
				}
			}
		}
	}
	artwork := seriesArtworkFromInfo(seriesInfo)
	if artwork.empty() {
		artwork = seriesArtwork{
			posterURL:   info.EpisodeMetadata.PosterURL,
			backdropURL: info.EpisodeMetadata.BackdropURL,
		}
	}
	if needsSeriesArtwork(outputBase) && artwork.empty() {
		if _, serr := fetchSeriesInfo(); serr != nil {
			output.Global.Warn("No artwork available for %s: %v", info.EpisodeMetadata.SeriesTitle, serr)
			if diag.ApiLogger != nil {
				diag.ApiLogger.Warn("artwork_fetch_failed", "series", info.EpisodeMetadata.SeriesTitle, "err", serr)
			}
		}
		artwork = seriesArtworkFromInfo(seriesInfo)
		if artwork.empty() {
			artwork = seriesArtwork{
				posterURL:   info.EpisodeMetadata.PosterURL,
				backdropURL: info.EpisodeMetadata.BackdropURL,
			}
		}
	}
	writeSeriesArtwork(ctx, client, info.EpisodeMetadata.SeriesTitle, outputBase, artwork, episodeFetchArtwork)

	// Per-episode success result line
	duration := time.Since(episodeStart).Round(time.Second)
	var fileSizeStr string
	var fileSize int64
	if fi, err := os.Stat(outputFile); err == nil {
		fileSize = fi.Size()
		fileSizeStr = formatFileSize(fileSize)
	}
	output.Global.Info("%s[Episode %d/%d] %s ... %s%s %s%s %s",
		output.ANSIGreen,
		info.EpisodeMetadata.EpisodeNumber, totalEpisodes, info.Title,
		output.ANSIGreen, "✓", output.ANSIReset,
		fileSizeStr, formatDuration(duration))
	if diag.DownloadLogger != nil {
		diag.DownloadLogger.Info("episode_finish", "ep", info.EpisodeMetadata.EpisodeNumber, "duration", duration.String(), "size_bytes", fileSize)
	}
	if len(skippedTracks) > 0 {
		output.Global.Warn("Episode %d downloaded partially: %d track(s) skipped (%s)",
			info.EpisodeMetadata.EpisodeNumber, len(skippedTracks), strings.Join(skippedTracks, ", "))
	}
	return nil
}

func cleanupEpisodeArtifacts(outputFile string, tempFiles []string) {
	for _, path := range append(tempFiles, outputFile) {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			output.Global.Warn("Failed to remove partial file %s: %v", path, err)
		}
	}
}

func formatFileSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
