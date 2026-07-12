package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"crunchyroll-downloader/internal/api"
	"crunchyroll-downloader/internal/config"
	"crunchyroll-downloader/internal/diag"
	"crunchyroll-downloader/internal/download"
	"crunchyroll-downloader/internal/drm"
	"crunchyroll-downloader/internal/output"
)

var (
	audioLang     = flag.String("audio-lang", "ja-JP", "Audio language(s), comma-separated for multiple (e.g. \"ja-JP,en-US\"). First is the default track")
	subtitlesLang = flag.String("subs-lang", "en-US", "Subtitle language(s), comma-separated for multiple (e.g. \"en-US,es-419\"). First is the default track")
	videoQuality  = flag.String("video-quality", "1080p", "Video quality")
	audioQuality  = flag.String("audio-quality", "192k", "Audio quality")
	seasonNumber  = flag.Int("season", 0, "Season number. Not used if an episode link is entered")
	etpRt         = flag.String("etp-rt", "", "The \"etp_rt\" cookie value of your account")
	debug         = flag.Bool("debug-manifest", false, "Log raw episode playback JSON and manifest XML")
	workers       = flag.Int("workers", 10, "Number of concurrent segment download workers")
	outputDir     = flag.String("output-dir", "", "Custom output directory for downloads")
	widevineDev   = flag.String("widevine-device", "", "Path to .wvd file or directory with client_id.bin + private_key.pem")
	jsonMode      = flag.Bool("json", false, "Output progress as NDJSON")
	quietMode     = flag.Bool("quiet", false, "Suppress progress output (errors still print)")
	logLevel      = flag.String("log-level", "info", "Diagnostic log level (debug|info|warn|error)")
	logFile       = flag.String("log-file", "", "Diagnostic log file path (default ./logs/animeheaven.log)")
	jellyfinMeta  = flag.Bool("jellyfin-metadata", false, "Generate Jellyfin-compatible NFO metadata and artwork")
)

func parseLangs(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// resolveLangs resolves a language list through the D-01/D-02 precedence:
// explicit CLI flag > config array > default. The explicitFlags map should be
// built via flag.Visit(). The first element of the returned slice is the
// protected primary track, so slice order is preserved exactly. No language
// environment variables are consulted (no CRUNCHYROLL_AUDIO_LANG /
// CRUNCHYROLL_SUBS_LANG decision exists in Phase 06).
func resolveLangs(explicitFlags map[string]bool, flagName string, flagVal string, configVal []string, defaultVal []string) []string {
	if explicitFlags[flagName] {
		return parseLangs(flagVal)
	}
	if configVal != nil {
		return configVal
	}
	return defaultVal
}

// validateOutputDir checks that the specified output directory exists and is a
// directory. Returns an error message, or empty string if valid.
func validateOutputDir(dir string) string {
	if dir == "" {
		return "" // empty dir means use CWD default — valid
	}
	if fi, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Sprintf("Output directory %s does not exist. Create it first or omit --output-dir to use the current directory.", dir)
	} else if err != nil {
		return fmt.Sprintf("Error accessing output directory %s: %v", dir, err)
	} else if !fi.IsDir() {
		return fmt.Sprintf("Output directory %s is not a directory.", dir)
	}
	return ""
}

// invalidURL holds a URL that failed validation and the reason.
type invalidURL struct {
	URL   string
	Error string
}

// validateURL checks that the URL has a /watch/ or /series/ path with a
// content ID between 9 and 14 characters.
func validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return fmt.Errorf("URL path must contain content type and ID")
	}

	contentType := parts[0]
	contentID := parts[1]

	if contentType != "watch" && contentType != "series" {
		return fmt.Errorf("URL must be /watch/ or /series/")
	}

	if len(contentID) < 9 || len(contentID) > 14 {
		return fmt.Errorf("content ID length must be 9-14 characters (got %d)", len(contentID))
	}

	return nil
}

// validateAllURLs validates all URLs upfront and returns any that failed.
func validateAllURLs(urls []string) []invalidURL {
	var invalid []invalidURL
	for _, u := range urls {
		if err := validateURL(u); err != nil {
			invalid = append(invalid, invalidURL{URL: u, Error: err.Error()})
		}
	}
	return invalid
}

func processURL(ctx context.Context, client *api.Client, rawURL string, outputDir string, audioLangs, subsLangs []string, generateJellyfinMetadata bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		output.Global.Error("Invalid URL: %v", err)
		return
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		output.Global.Error("Invalid URL format: %s", rawURL)
		return
	}
	contentType := parts[0]
	contentID := parts[1]
	if len(contentID) < 9 || len(contentID) > 14 {
		output.Global.Error("Invalid URL format: %s", rawURL)
		return
	}
	if contentType != "watch" && contentType != "series" {
		output.Global.Error("Invalid URL (must be /watch/ or /series/): %s", rawURL)
		return
	}

	if len(audioLangs) == 0 {
		audioLangs = []string{"ja-JP"}
	}

	primaryAudio := audioLangs[0]
	primarySubs := "en-US"
	if len(subsLangs) > 0 {
		primarySubs = subsLangs[0]
	}

	if contentType == "watch" {
		info, err := client.GetEpisodeInfo(ctx, contentID)
		if err != nil {
			output.Global.Error("Error fetching episode info: %v", err)
			return
		}
		if err := download.Episode(ctx, client, contentID, info, audioLangs, subsLangs, videoQuality, audioQuality, *workers, outputDir, 1, generateJellyfinMetadata); err != nil {
			output.Global.Error("Error downloading episode: %v", err)
		}
	} else {
		seasons, err := client.GetSeasons(ctx, contentID, primaryAudio, primarySubs)
		if err != nil {
			output.Global.Error("Error fetching seasons: %v", err)
			return
		}

		if *seasonNumber != 0 {
			var seasonID string
			for _, season := range seasons {
				if season.SeasonNumber == *seasonNumber {
					seasonID = season.ID
					break
				}
			}
			if seasonID == "" {
				output.Global.Warn("This anime has no season %v!", *seasonNumber)
				return
			}

			episodes, err := client.GetSeasonEpisodes(ctx, seasonID, primaryAudio, primarySubs)
			if err != nil {
				output.Global.Error("Error fetching episodes: %v", err)
				return
			}
			if err := download.Season(ctx, client, videoQuality, audioQuality, audioLangs, subsLangs, episodes, *workers, outputDir, generateJellyfinMetadata); err != nil {
				output.Global.Warn("Season completed with errors: %v", err)
			}
		} else {
			output.Global.Info("No season number specified, downloading all seasons...")

			for _, season := range seasons {
				episodes, err := client.GetSeasonEpisodes(ctx, season.ID, primaryAudio, primarySubs)
				if err != nil {
					output.Global.Error("Error fetching episodes for season %v: %v", season.SeasonNumber, err)
					continue
				}
				if err := download.Season(ctx, client, videoQuality, audioQuality, audioLangs, subsLangs, episodes, *workers, outputDir, generateJellyfinMetadata); err != nil {
					output.Global.Warn("Season %v completed with errors: %v", season.SeasonNumber, err)
				}
			}
		}
	}
}

// checkFFmpeg validates that FFmpeg is available on the system PATH
// and can be executed. Returns an actionable error if not.
func checkFFmpeg() error {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("FFmpeg not found: install FFmpeg and ensure it is on $PATH. See https://ffmpeg.org/download.html")
	}

	cmd := exec.Command("ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("FFmpeg found at %s but failed to run: %w", path, err)
	}

	return nil
}

// isAllNilConfig returns true if all pointer fields in cfg are nil,
// indicating the config file did not exist or was empty.
func isAllNilConfig(cfg *config.Config) bool {
	return cfg.AudioLang == nil &&
		cfg.SubsLang == nil &&
		cfg.VideoQuality == nil &&
		cfg.AudioQuality == nil &&
		cfg.Workers == nil &&
		cfg.OutputDir == nil &&
		cfg.EtpRt == nil &&
		cfg.WidevineDevice == nil && cfg.LogLevel == nil && cfg.LogFile == nil
}

// resolveString resolves a string value through the precedence hierarchy:
// explicit CLI flag > env var > config value > default value.
// The explicitFlags map should be built via flag.Visit().
func resolveString(explicitFlags map[string]bool, flagName string, flagVal string, envName string, configVal *string, defaultVal string) string {
	if explicitFlags[flagName] {
		return flagVal
	}
	if v, ok := os.LookupEnv(envName); ok && v != "" {
		return v
	}
	if configVal != nil && *configVal != "" {
		return *configVal
	}
	return defaultVal
}

// resolveEtpRt resolves the etp_rt value through the precedence hierarchy:
// explicit --etp-rt CLI flag > CRUNCHYROLL_ETP_RT env var > config file value.
func resolveEtpRt(explicitFlags map[string]bool, flagVal string, configVal *string) string {
	if explicitFlags["etp-rt"] && flagVal != "" {
		return flagVal
	}
	if v, ok := os.LookupEnv("CRUNCHYROLL_ETP_RT"); ok && v != "" {
		return v
	}
	if configVal != nil && *configVal != "" {
		return *configVal
	}
	return ""
}

func resolveInputTargets(rawURL, filePath, urlsPath string) (string, string, error) {
	if filePath != "" && urlsPath != "" && filePath != urlsPath {
		return "", "", fmt.Errorf("--file and --urls specify different files")
	}
	if urlsPath != "" {
		filePath = urlsPath
	}
	return rawURL, filePath, nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if _, err := config.LoadDotenv(); err != nil {
		output.Global.Warn("Warning: error loading .env file: %v", err)
	}

	url := flag.String("url", "", "URL of the episode/season to download")
	urlsFile := flag.String("file", "", "Path to a text file with one URL per line")
	urlsFileAlias := flag.String("urls", "", "Path to a text file with one URL per line (alias for --file)")
	flag.Parse()

	resolvedURL, resolvedURLsFile, err := resolveInputTargets(*url, *urlsFile, *urlsFileAlias)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if resolvedURL == "" && resolvedURLsFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Track explicitly-set flags via flag.Visit()
	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

	// Resolve output mode (after flag.Parse)
	switch {
	case *jsonMode:
		output.Init(output.ModeJSON)
	case *quietMode:
		output.Init(output.ModeQuiet)
	default:
		output.Init(output.ModeHuman)
	}

	// Resolve config path
	cfgPath, err := config.ConfigPath()
	if err != nil {
		output.Global.Warn("Warning: cannot determine config path: %v — skipping config file", err)
	}

	// Load config (if config path was resolved)
	cfg := &config.Config{}
	if err == nil {
		cfg, err = config.Load(cfgPath)
		if err != nil {
			// Invalid JSON: warn and continue with defaults
			output.Global.Warn("%v — using defaults", err)
			cfg = &config.Config{}
		} else if isAllNilConfig(cfg) {
			// Config file does not exist: create skeleton
			if err := config.WriteSkeleton(cfgPath); err != nil {
				output.Global.Warn("Could not create default config: %v", err)
			} else {
				output.Global.Info("Created default config at %s", cfgPath)
			}
		}
	}

	// Resolve precedence: CLI flag > env var > config file > default
	resolvedEtpRt := resolveEtpRt(explicitFlags, *etpRt, cfg.EtpRt)
	resolvedOutputDir := resolveString(explicitFlags, "output-dir", *outputDir, "OUTPUT_DIR", cfg.OutputDir, "")
	resolvedLogLevel := resolveString(explicitFlags, "log-level", *logLevel, "CRUNCHYROLL_LOG_LEVEL", cfg.LogLevel, "info")
	resolvedLogFile := resolveString(explicitFlags, "log-file", *logFile, "CRUNCHYROLL_LOG_FILE", cfg.LogFile, "./logs/animeheaven.log")
	diag.Init(diag.ParseLevel(resolvedLogLevel), resolvedLogFile)

	// Validate FFmpeg availability before any download (D-18, D-19)
	if err := checkFFmpeg(); err != nil {
		output.Global.Error("%v", err)
		os.Exit(1)
	}

	// Parse language flags once after config resolution (QOL-08, D-22).
	// Config audio_lang/subs_lang arrays feed runtime selection unless the
	// matching CLI flag is explicit (D-01/D-02); first element is primary.
	audioLangs := resolveLangs(explicitFlags, "audio-lang", *audioLang, cfg.AudioLang, []string{"ja-JP"})
	subsLangs := resolveLangs(explicitFlags, "subs-lang", *subtitlesLang, cfg.SubsLang, []string{"en-US"})

	// Resolve Widevine device path through precedence and set it before
	// any API client call, ensuring sync.Once uses the correct path.
	resolvedWidevineDev := resolveString(explicitFlags, "widevine-device", *widevineDev, "WIDEVINE_DEVICE_PATH", cfg.WidevineDevice, "")
	if resolvedWidevineDev != "" {
		drm.SetWidevinePath(resolvedWidevineDev)
	}

	client, err := api.NewWithContext(ctx, resolvedEtpRt)
	if err != nil {
		output.Global.Error("Failed to initialize API client: %v", err)
		os.Exit(1)
	}
	client.Debug = *debug
	// Validate output directory exists if specified (D-11)
	if errMsg := validateOutputDir(resolvedOutputDir); errMsg != "" {
		output.Global.Error("%s", errMsg)
		os.Exit(1)
	}

	if resolvedURLsFile != "" {
		file, err := os.Open(resolvedURLsFile)
		if err != nil {
			output.Global.Error("Failed to open URLs file: %s", err)
			os.Exit(1)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		var urls []string
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && strings.HasPrefix(line, "http") {
				urls = append(urls, line)
			}
		}

		// Validate all URLs upfront before any downloads (D-17)
		if invalid := validateAllURLs(urls); len(invalid) > 0 {
			output.Global.Info("Invalid URLs found:")
			for _, inv := range invalid {
				output.Global.Info("  %s — %s", inv.URL, inv.Error)
			}
			os.Exit(1)
		}

		output.Global.Info("Found %d URLs to download\n", len(urls))
		for i, u := range urls {
			output.Global.Info("=== [%d/%d] %s ===", i+1, len(urls), u)
			processURL(ctx, client, u, resolvedOutputDir, audioLangs, subsLangs, *jellyfinMeta)
			output.Global.Info("")
		}
	} else {
		processURL(ctx, client, resolvedURL, resolvedOutputDir, audioLangs, subsLangs, *jellyfinMeta)
	}
}
