# Phase 6: Track Hardening + Structured Logging - Pattern Map

**Mapped:** 2026-07-10
**Files analyzed:** 11 (new + modified)
**Analogs found:** 11 / 11 (every new file has a role/analog match — most modifications ARE their own analog)

> Phase 6 has NO RESEARCH.md. File list extracted from `06-CONTEXT.md` `<canonical_refs>` §Source files and `<code_context>` §Integration Points.
> Recommended package name: **`internal/diag/`** (CONTEXT.md "the agent's Discretion" notes `internal/log/` may collide with stdlib `log`; `diag` avoids the collision and reads as "diagnostics").

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/diag/diag.go` (NEW) | utility / singleton logger | file-I/O (rotated writes) | `internal/output/output.go` (Global + Init + interface) | exact-role |
| `internal/diag/redact.go` (NEW) | utility / slog.Handler wrapper | transform (attr interception) | `internal/output/output.go` `humanOutput` wrapping a sink | role-match |
| `internal/config/config.go` (MODIFY) | model / config struct | request-response (load/merge) | itself (pointer-field + Merge pattern) | exact |
| `main.go` (MODIFY) | config / cli init | request-response (flag parse) | itself (flag block + resolveString + output.Init) | exact |
| `internal/download/episode.go` (MODIFY) | service | batch / transform (locale loop) | itself (`guidByLocale` loop + audio version loop) | exact |
| `internal/mux/mux.go` (MODIFY) | service | file-I/O (os.Stat) + process (ffmpeg) | itself (`MergeEverything` arg assembly + `warnRemove` error convention) | exact |
| `internal/output/ndjson.go` (REUSE, no edit) | utility | event (NDJSON emit) | itself (`warn` event type) | exact |
| `internal/api/auth.go` (MODIFY — redaction target) | service | request-response (HTTP) | itself (`fetchAccessToken` header set) | exact |
| `internal/api/client.go` (MODIFY — slog source + redaction target) | service | request-response (retry on 401) | itself (`Do` token-refresh path) | exact |
| `internal/diag/diag_test.go` (NEW) | test | table-driven / stdlib | `internal/mux/mux_test.go`, `internal/config/config_test.go` | exact-role |
| `internal/mux/mux_test.go` + `internal/download/episode_test.go` + `internal/config/config_test.go` (MODIFY/extend) | test | table-driven / stdlib | themselves | exact |

## Pattern Assignments

### `internal/diag/diag.go` (utility, singleton logger, file-I/O)

**Analog:** `internal/output/output.go` — the `Global`/`Outputter`/`Init` triple D-17 directly mirrors.

**Singleton + interface pattern** (`internal/output/output.go:32-43`):
```go
// Global is the application-wide output sink.
// Set once in main() after flag parsing — same pattern as internal/config.
var Global Outputter = &humanOutput{}

// Outputter defines the output interface with five methods.
type Outputter interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Error(format string, args ...any)
	Debug(format string, args ...any)
	Progress(format string, args ...any)
}
```
→ `diag.Global` should be `var Global *slog.Logger` initialized to a no-op default (mirrors `humanOutput{}` default before `Init`), then replaced inside `Init(level, path)` (mirrors `output.Init(m)`). The doc comment style ("Set once in main() after flag parsing — same pattern as internal/config") must be replicated.

**Init-by-mode pattern** (`internal/output/output.go:81-104`):
```go
// Init sets Global to the correct implementation based on mode.
func Init(m Mode) {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	if !isTTY { ANSIReset = ""; ... }
	switch m {
	case ModeHuman:
		Global = &humanOutput{speedTracker: NewSpeedTracker()}
	case ModeJSON:
		Global = &jsonOutput{enc: json.NewEncoder(os.Stdout), speedTracker: NewSpeedTracker()}
	case ModeQuiet:
		Global = &quietOutput{}
	}
}
```
→ `diag.Init(level slog.Level, path string)` mirrors: parse level, build `lumberjack.Writer{Filename: path, MaxSize: 10, MaxBackups: 5, MaxAge: 0, Compress: false}` (D-15), wrap in `slog.NewTextHandler(w, &slog.HandlerOptions{ReplaceAttr: redactAttr, Level: level})`, set `Global = slog.New(handler)`.

**Imports pattern** (project convention — stdlib first, internal grouped):
```go
import (
	"encoding/json"  // output.go:4
	"fmt"
	"os"
	"sync"
	"golang.org/x/term"  // external dep last
)
```
→ `diag.go` imports: `log/slog`, `os`, `gopkg.in/natefinch/lumberjack.v2` (NEW dep per D-16 — must add to go.mod `require` block alongside existing `golang.org/x/term`).

**Per-subsystem WithGroup pattern** (D-17) — there is no existing analog (new pattern); model the accessor on logged-Go convention but follow project var-naming (lowercase package-level `var` like `output.Global`):
```go
// Subsystem loggers — child of Global, set in Init() before first use.
var (
	DownloadLogger *slog.Logger  // group="download"
	DrmLogger      *slog.Logger  // group="drm"
	MediaLogger    *slog.Logger  // group="media"
	MuxLogger      *slog.Logger  // group="mux"
	ApiLogger      *slog.Logger  // group="api"
)
```
Init each as `Global.WithGroup("download")` etc. inside `diag.Init(...)`. Planners: verify ordering so `Global` is non-nil before WithGroup.

---

### `internal/diag/redact.go` (utility, slog.Handler wrapper, transform)

**Analog:** `internal/output/output.go` `humanOutput`/`jsonOutput`/`quietOutput` — three structs wrapping the same 5-method `Outputter` interface. The pattern for D-20/D-22 is identical structurally (one handler struct wrapping a base handler, intercepting one method).

**Handler-wrapping pattern** (`internal/output/output.go:107-124`):
```go
// humanOutput implements Outputter for colored terminal output.
type humanOutput struct {
	mu           sync.Mutex
	speedTracker *SpeedTracker
}

func (h *humanOutput) Warn(format string, args ...any) {
	h.mu.Lock()
	fmt.Fprintf(os.Stdout, "\033[K%s⚠%s %s\n", ANSIYellow, ANSIReset, fmt.Sprintf(format, args...))
	h.mu.Unlock()
}
```
→ `redactingHandler` struct embeds/wraps `slog.Handler`:
```go
type redactingHandler struct{ inner slog.Handler }
func (h redactingHandler) Handle(ctx context.Context, r slog.Record) error {
	r2 := r.Clone()
	r2.Attrs(func(a slog.Attr) bool { /* redact-by-key D-20 */ ...; return true })
	return h.inner.Handle(ctx, r2)
}
func (h redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return redactingHandler{h.inner.WithAttrs(attrs)} }
func (h redactingHandler) WithGroup(name string) slog.Handler        { return redactingHandler{h.inner.WithGroup(name)} }
func (h redactingHandler) Enabled(ctx context.Context, l slog.Level) bool { return h.inner.Enabled(ctx, l) }
```
Embedded-interface wrapping is idiomatic; the `humanOutput`-as-concrete-struct style in output.go favors concrete types over interfaces — keep `redactingHandler` concrete likewise.

**Redaction key set** (D-21 — exact 6 keys, `device_id`/URLs explicitly NOT redacted):
```go
var redactKeys = map[string]struct{}{
	"token": {}, "cookie": {}, "etp_rt": {}, "client_id": {},
	"private_key": {}, "authorization": {},
}
```
Redact by `ReplaceAttr` callback returning `slog.String(key, "[REDACTED]")` when key is in the set.

---

### `internal/config/config.go` (model, request-response, MODIFY)

**Analog:** itself — the pointer-field + Merge pattern is exactly what `LogLevel`/`LogFile` (D-19) and the array-typed `AudioLang`/`SubsLang` (D-02) follow.

**Config struct / pointer-field pattern** (`internal/config/config.go:10-21`):
```go
// Config represents the JSON config file at ./config.json (project root).
// Pointer fields enable explicit-only overrides: nil = absent from file.
type Config struct {
	AudioLang      *string `json:"audio_lang,omitempty"`
	SubsLang       *string `json:"subs_lang,omitempty"`
	VideoQuality   *string `json:"video_quality,omitempty"`
	AudioQuality   *string `json:"audio_quality,omitempty"`
	Workers        *int    `json:"workers,omitempty"`
	OutputDir      *string `json:"output_dir,omitempty"`
	EtpRt          *string `json:"etp_rt,omitempty"`
	WidevineDevice *string `json:"widevine_device,omitempty"`
}
```
**D-02 array migration** — `AudioLang` becomes `[]string` (`json:"audio_lang,omitempty"`); backward compat (single string tolerable) per CONTEXT.md D-02. The struct field comment must explain "nil = absent; empty slice = explicit empty". `SubsLang` symmetrically.
**D-19 additions** — add two pointer-field lines for parity:
```go
	LogLevel *string `json:"log_level,omitempty"`
	LogFile  *string `json:"log_file,omitempty"`
```

**Merge explicit-only-override pattern** (`internal/config/config.go:83-123`):
```go
func Merge(base, overlay *Config) *Config {
	result := &Config{}
	if base != nil { *result = *base } // shallow copy
	if overlay == nil { return result }
	if overlay.AudioLang != nil { result.AudioLang = overlay.AudioLang }
	if overlay.SubsLang != nil  { result.SubsLang = overlay.SubsLang }
	// ... one branch per pointer field
	return result
}
```
→ Add two branches `if overlay.LogLevel != nil { result.LogLevel = overlay.LogLevel }` and `LogFile` symmetrically. For array fields, `if overlay.AudioLang != nil { result.AudioLang = overlay.AudioLang }` (slice-nil test still works for explicit-empty).

**WriteSkeleton default values pattern** (`internal/config/config.go:59-81`) — update skeleton to array form per D-02: `"audio_lang": []string{"ja-JP"}`, `"subs_lang": []string{"en-US"}` and (per D-19 default) add `"log_level": "info"`.

---

### `main.go` (config/cli init, request-response, MODIFY)

**Analog:** itself — the flag block, `resolveString`, `flag.Visit`, and `output.Init` block are ALL the patterns being extended/reused.

**Flag declaration pattern** (`main.go:22-35`) — append two lines keeping the `flagString("name", default, "desc")` convention:
```go
var (
	audioLang     = flag.String("audio-lang", "ja-JP", ...)
	// ... existing flags ...
	jsonMode      = flag.Bool("json", false, ...)
	quietMode     = flag.Bool("quiet", false, ...)
	// ADD:
	logLevel = flag.String("log-level", "info", "Diagnostic log level (debug|info|warn|error)")
	logFile  = flag.String("log-file", "", "Diagnostic log file path (default ./logs/animeheaven.log)")
)
```

**resolveString precedence pattern** (`main.go:223-237`) — D-19 reuses verbatim:
```go
// resolveString resolves a string value through the precedence hierarchy:
// explicit CLI flag > env var > config value > default value.
// The explicitFlags map should be built via flag.Visit().
func resolveString(explicitFlags map[string]bool, flagName string, flagVal string, envName string, configVal *string, defaultVal string) string {
	if explicitFlags[flagName] { return flagVal }
	if v, ok := os.LookupEnv(envName); ok && v != "" { return v }
	if configVal != nil && *configVal != "" { return *configVal }
	return defaultVal
}
```
→ Two new call sites mirroring `resolveString(explicitFlags, "output-dir", *outputDir, "OUTPUT_DIR", cfg.OutputDir, "")` at `main.go:313`:
```go
resolvedLogLevel := resolveString(explicitFlags, "log-level", *logLevel, "CRUNCHYROLL_LOG_LEVEL", cfg.LogLevel, "info")
resolvedLogFile  := resolveString(explicitFlags, "log-file",  *logFile,  "CRUNCHYROLL_LOG_FILE",  cfg.LogFile,  "./logs/animeheaven.log")
```
Then `diag.Init(parseLevel(resolvedLogLevel), resolvedLogFile)`.

**flag.Visit explicit-tracking pattern** (`main.go:271-275`):
```go
// Track explicitly-set flags via flag.Visit()
explicitFlags := make(map[string]bool)
flag.Visit(func(f *flag.Flag) {
	explicitFlags[f.Name] = true
})
```
→ No change required — this is the upstream feeder for `resolveString`; works for `log-level`/`log-file` automatically.

**Singleton-init insertion point** (`main.go:277-285`) — place `diag.Init` IMMEDIATELY AFTER `output.Init` (D-13: separate plane, but parallel init order):
```go
switch {
case *jsonMode:   output.Init(output.ModeJSON)
case *quietMode:  output.Init(output.ModeQuiet)
default:          output.Init(output.ModeHuman)
}
// ADD: diag.Init(resolvedLogLevel, resolvedLogFile)  — but note log-file resolution
// happens later (after cfg load at :294-309). Init logger AFTER config load to use cfg.LogLevel/LogFile.
```
**Ordering note for planner:** `output.Init` runs at line 278 BEFORE config load (lines 293-309) because output mode doesn't depend on config. But `diag.Init` DOES depend on config (`cfg.LogLevel`/`cfg.LogFile`), so `diag.Init` must be inserted AFTER the config-load block (after line 309) and AFTER the `resolveString` calls for log-level/log-file (insert near line 313 next to existing resolvedEtpRt/resolvedOutputDir lines).

**parseLangs array helper** (`main.go:37-45`) — D-02 keeps `--audio-lang` as comma-separated string; `parseLangs` already produces `[]string`. No change needed, but planner should confirm `audioLangs[0]` access (D-01 primary) is guarded against empty slice.

**parseLangs → audioLangs[0]** already consumed at `main.go:134` (`primaryAudio := audioLangs[0]`) and `:138` (`primarySubs = subsLangs[0]`) — both have implicit "audioLangs non-empty" preconditions. The Phase 6 ERR-03 hard-error path should make these explicit (guard at `processURL`, not deep in `download.Episode`).

---

### `internal/download/episode.go` (service, batch/transform, MODIFY)

**Analog:** itself — the `guidByLocale` map + the audio `for _, locale` loop at `:94-100` and the subtitle loop at `:141-145` are the **exact two insertion points** CONTEXT.md `code_context` calls out.

**Audio locale-resolution loop (PRIMARY hard-error target)** (`internal/download/episode.go:64-100`):
```go
guidByLocale := map[string]string{}
if info.EpisodeMetadata.AudioLocale != "" {
	guidByLocale[info.EpisodeMetadata.AudioLocale] = baseContentID
}
for _, v := range info.EpisodeMetadata.Versions {
	guidByLocale[v.AudioLocale] = v.GUID
}
// ... "all" expansion at :72-87 ...

type audioVersion struct { locale, contentId string }
var versions []audioVersion
for _, locale := range audioLangs {
	guid, ok := guidByLocale[locale]
	if !ok {
		return fmt.Errorf("audio locale %s is not available for episode %v", locale, info.EpisodeMetadata.EpisodeNumber)
	}
	versions = append(versions, audioVersion{locale: locale, contentId: guid})
}
```
**D-03/D-04/D-01 rewrite pattern** — the existing `return fmt.Errorf(...)` for EVERY missing locale becomes:
```go
for i, locale := range audioLangs {
	guid, ok := guidByLocale[locale]
	if !ok {
		if i == 0 {
			// D-03: missing PRIMARY = hard error
			return fmt.Errorf("primary audio locale %s not available for episode %d", locale, info.EpisodeMetadata.EpisodeNumber)
		}
		// D-04: missing SECONDARY = warn + skip
		output.Global.Warn("Skipping %s audio: not available for episode %d", locale, info.EpisodeMetadata.EpisodeNumber)
		if diag.ApiLogger != nil { diag.ApiLogger.Warn("skip non-primary audio", "episode", info.EpisodeMetadata.EpisodeNumber, "locale", locale) }
		// emit warn NDJSON event for --json mode (D-08)
		continue
	}
	versions = append(versions, audioVersion{locale: locale, contentId: guid})
}
if len(versions) == 0 {
	return fmt.Errorf("no audio tracks available for episode %d (ERR-03)", info.EpisodeMetadata.EpisodeNumber)  // hard error, no silent empty MKV
}
```
The error-formatting convention (`fmt.Errorf("...: %w", err)` / `fmt.Errorf("... %v", ...)` plain) is established throughout this file (lines 48, 98, 125, 152, 198, 289).

**Subtitle loop (symmetric PRIMARY/SECONDARY)** (`internal/download/episode.go:141-156`):
```go
for _, locale := range subsLangs {
	if firstEpisode.Subtitles[locale] == nil {
		return fmt.Errorf("subtitle locale %s is not available for episode %v", locale, info.EpisodeMetadata.EpisodeNumber)
	}
}
var subTracks []mux.MediaTrack
for _, locale := range subsLangs {
	output.Global.Info("Downloading subtitles for %s...", mux.TrackTitle(locale))
	file, err := media.DownloadSubs(ctx, client, firstEpisode.Subtitles[locale].URL)
	if err != nil {
		return fmt.Errorf("downloading subtitles for %s: %w", locale, err)   // D-06: download failure = hard error (keep)
	}
	// ...
}
```
**D-05 + D-06 rewrite** — split presence-check (D-05: primary missing=hard error, secondary missing=warn+skip) from download (D-06: download failure=hard error regardless). Note `firstEpisode.Subtitles[locale]` nil-check at `:142` is the presence test; `media.DownloadSubs` failure at `:151` is the download-failure test — these two error paths are kept distinct.

**Output interface usage** (`internal/output/output.go:38-42` + `ndjson.go`) — `output.Global.Warn(...)` already emits `Type:"warn"` NDJSON in JSON mode (D-08 reuse = no contract change):
```go
// ndjson.go:158-162 — jsonOutput.Warn via emitEvent(event{Type:"warn", Message:...})
// ndjson.go:10-44 — event struct (DO NOT ADD new fields; D-08 = "zero change to NDJSON contract")
```
**D-09 human-mode warn** — `output.Global.Warn` (human impl `output.go:120-124`) already renders `⚠` yellow line; reuse exactly. No new visual style.

**Clean error-return / error-wrapping convention** — every error in this file is `return fmt.Errorf("contextual msg: %w", err)` or `return fmt.Errorf("contextual msg %v", value)`. The Phase 6 skips use `output.Global.Warn(...)` + `continue` (mirroring how the file already warns via `output.Global.Warn("Failed to remove stream %s: %v", id, err)` at `:113`).

**`completed` flag / partial-tracking** (`internal/download/episode.go:106, 299-312`) — D-07 partial marker: insert `skippedTracks []string` accumulator in scope around `:106`, append per skipped locale (both audio + subtitle), and after the success line at `:307` emit a conditional `output.Global.Warn("Episode %d downloaded partially: %d track(s) skipped (%s)", ...)` when `len(skippedTracks) > 0` and `completed` is still true.

---

### `internal/mux/mux.go` (service, file-I/O + process, MODIFY)

**Analog:** itself — `MergeEverything` is the **single FFmpeg invocation point** (D-12), and `warnRemove` is the established error-non-fatal convention.

**Insertion point + arg-building pattern** (`internal/mux/mux.go:29-48`):
```go
func MergeEverything(ctx context.Context, videoFile string, audioTracks, subTracks []MediaTrack, outputFile string, info *api.EpisodeInfo) error {
	if ctx == nil { ctx = context.Background() }
	args := []string{"-i", videoFile}
	for _, audio := range audioTracks { args = append(args, "-i", audio.File) }
	for _, sub := range subTracks    { args = append(args, "-i", sub.File) }

	args = append(args, "-map", "0:v:0")
	for i := range audioTracks { args = append(args, "-map", fmt.Sprintf("%d:a:0", 1+i)) }
	for j := range subTracks    { args = append(args, "-map", fmt.Sprintf("%d", 1+len(audioTracks)+j)) }
	// ... rest of args ...
```
**D-10/D-12 insertion** — insert `os.Stat` size-only validation IMMEDIATELY after `ctx = context.Background()` and BEFORE `args := []string{...}`:
```go
// D-10/D-12: reject any empty input — never invoke FFmpeg with zero-byte files
// (positional -map mislabeling root cause, ERR-04).
inputs := append([]string{videoFile}, tracksToPaths(audioTracks, subTracks)...)
for _, path := range inputs {
	if path == "" { continue }  // tolerate absent optional inputs
	fi, err := os.Stat(path)
	if err != nil { return fmt.Errorf("mux input %s: %w", path, err) }
	if fi.Size() == 0 { return fmt.Errorf("mux input %s is empty (0 bytes): refusing to invoke FFmpeg", path) }
}
```
`os` is already imported (`mux.go:7`). `os.Stat` returns `os.IsNotExist` — the established pattern at `mux.go:96` (`!os.IsNotExist(cleanupErr)`) shows how this file discriminates absence.

**Hard-error convention** — `MergeEverything` returns `error`; episode.go wraps via `return fmt.Errorf("muxing episode: %w", err)` at `:297`. D-11 "empty input = hard episode error" flows naturally: the os.Stat failure returns from MergeEverything, propagates up through `download.Episode`, marks episode failed by NOT setting `completed=true` (`episode.go:299`), so the deferred cleanup at `:107-119` removes partial output. **No new convention needed** — the existing cleanup-on-return path already handles it.

**FFmpeg-exec variableref pattern** (`internal/mux/mux.go:20, 91`):
```go
var ffmpegCommand = exec.CommandContext
// ...
cmd := ffmpegCommand(ctx, "ffmpeg", args...)
```
This is swappμέ for tests (see mux_test.go below) — Phase 6 os.Stat validation runs BEFORE `ffmpegCommand`, so the existing test-injection seam is preserved (validation runs against real `t.TempDir()` files, the swap only affects the eventual ffmpeg call).

**`warnRemove` non-fatal-warn convention** (`internal/mux/mux.go:114-118`):
```go
func warnRemove(path string) {
	if err := os.Remove(path); err != nil {
		output.Global.Warn("Failed to remove temporary file %s: %v", path, err)
	}
}
```
→ Mirror this for any D-12 logging: `output.Global.Warn(...)` for non-fatal, `return fmt.Errorf(...)` for fatal. ALSO add parallel `diag.MuxLogger.Warn(...)` calls alongside (D-17 strategic events).

**FFmpeg summary strategic event (D-18)** — `mux.go:92-100` already captures `stderr bytes.Buffer` for the error path; D-18 wants an Info-level strategic event on SUCCESS too ("FFmpeg summary (stderr excerpt)"). The success path is at `:102-108` (loop removing temp files); insert `if diag.MuxLogger != nil { diag.MuxLogger.Info("ffmpeg finished", "stderr", stderr.String()) }` near there (stderr is non-empty in successful ffmpeg runs as a progress log).

---

### `internal/output/ndjson.go` (utility, event, REUSE — no edit)

**Analog:** itself — D-08 explicitly says "Reuse existing `warn` event type. Zero change to the NDJSON contract."

**`warn` event shape** (`internal/output/ndjson.go:10-44, 54-62`):
```go
type event struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	// ... Episode/Progress/Completion/Summary fields (all omitempty) ...
	Message string `json:"message,omitempty"`
	Success bool   `json:"success,omitempty"`
	Fatal   bool   `json:"fatal,omitempty"`
	// ...
}
func emitEvent(evt event) {
	evt.Timestamp = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(evt)
	if err != nil { return }  // silently skip un-marshalable
	os.Stdout.Write(data)
	os.Stdout.Write([]byte{'\n'})
}
```
**Emission path for skip events** — callers do `output.Global.Warn("skipping %s dub: not available", locale)`; the `jsonOutput.Warn` impl (`output.go:158-162`) maps `Warn` → `emitEvent(event{Type:"warn", Message:...})`. **No new event type, no new field.** Planner: do NOT add a `Type:"skip"` or `Partial bool` field — D-08 forbids contract change.

---

### `internal/api/auth.go` (service, request-response, MODIFY — redaction target)

**Analog:** itself — the hardcoded Basic Auth constant + cookie set lines are the PII surface D-20/D-21 address.

**Hardcoded Basic Auth redaction target** (`internal/api/auth.go:16-31`):
```go
const defaultClientAuth = "Basic bm9haWhkZXZtXzZpeWcwYThsMHE6"
// ...
func getClientAuth() string {
	if auth, ok := os.LookupEnv("CRUNCHYROLL_CLIENT_AUTH"); ok && auth != "" { return auth }
	return defaultClientAuth
}
```
→ The `Authorization` request header value here IS the redaction target. Logging must never dump `req.Header`. If any slog attr records `authorization` key, `redactingHandler` (D-20) replaces value with `[REDACTED]`. **No code change to auth.go for redaction itself** — the handler layer intercepts. But if Phase 6 adds a slog call in auth.go like `diag.ApiLogger.Debug("token request", "auth", getClientAuth())`, the `auth`/`authorization` key must be in the redact set (it is — D-21).

**Cookie-set PII surface** (`internal/api/auth.go:49-50`):
```go
req.AddCookie(&http.Cookie{Name: "device_id", Value: c.deviceID})
req.AddCookie(&http.Cookie{Name: "etp_rt", Value: c.etpRt})
```
→ `etp_rt` redacted (D-21); `device_id` NOT redacted (D-21 explicit: "not user data"). Planner: when adding `diag.ApiLogger.Info("token request sent", "etp_rt", c.etpRt)`, the redact set catches it. Prefer logging `device_id` only.

---

### `internal/api/client.go` (service, request-response, retry-on-401, MODIFY — slog source + redaction target)

**Analog:** itself — the `Do` token-refresh path is the strategic-event source (D-18) and Bearer header is the redaction target (D-20).

**Bearer header + 401 refresh path** (`internal/api/client.go:91-122`):
```go
req.Header.Set("Authorization", "Bearer "+c.token)
// ... in Do(), on 401:
output.Global.Debug("Access token expired. Refetching one...")  // line 103 — existing
token, err := c.fetchAccessToken(req.Context())
if err != nil { return nil, fmt.Errorf("refreshing token: %w", err) }
c.token = token
retryReq := req.Clone(req.Context())
// ...
retryReq.Header.Set("Authorization", "Bearer "+c.token)  // line 122
```
**D-18 strategic-event insertion** — token refresh is Info-level per D-18. Augment the existing `output.Global.Debug(...)` at `:103` with a parallel slog.Info strategic event:
```go
diag.ApiLogger.Info("token refreshed", "episode", /* id if available */, "status", "401-recovered")
```
Keep `output.Global.Debug` for the user-facing plane (D-13: separate planes, both coexist).

**Bearer redaction target** — `"Bearer "+c.token` is logged only if explicitly passed as an slog attr. If Phase 6 adds `diag.ApiLogger.Debug("request", "url", u, "authorization", req.Header.Get("Authorization"))`, the `authorization` key is already in the redact set (D-21) — value → `[REDACTED]`. **No code change to client.go for redaction itself**; the handler layer intercepts.

**Error-wrapping convention** (`internal/api/client.go:106, 130`):
```go
return nil, fmt.Errorf("refreshing token: %w", err)
// ...
return nil, fmt.Errorf("unauthorized after token refresh retry")
```
→ A 401-refresh-retry still failing is a season-failure strategic event per D-18. Mirror the wrapping style when surfacing.

---

### `internal/diag/diag_test.go` (test, table-driven stdlib, NEW)

**Analog:** `internal/mux/mux_test.go` (table-driven + helper-process seam) and `internal/config/config_test.go` (table-driven config scenarios).

**Table-driven test pattern** (`internal/mux/mux_test.go:97-114`):
```go
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
```
→ `diag_test.go` table-driven cases for: `parseLevel` (`"debug"→slog.LevelDebug`, `"info"→slog.LevelInfo`, unknown→LevelInfo), and `redactKeys` membership (`token`→redacted, `device_id`→NOT redacted, etc.).

**Helper + t.Cleanup pattern** (`internal/mux/mux_test.go:78-95`):
```go
func restoreFFmpegCommand(t *testing.T, exitCode, stderr string) {
	t.Helper()
	original := ffmpegCommand
	ffmpegCommand = func(...) *exec.Cmd { ... }
	t.Cleanup(func() { ffmpegCommand = original })
}
```
→ `diag_test.go` mirrors for `diag.Global` (save original, swap, `t.Cleanup` restore) — required because `diag.Init` mutates the package global, exactly like `output.Init`.

**Env-var test isolation pattern** (`internal/config/config_test.go:41-57`):
```go
origMyKey, hadMyKey := os.LookupEnv("MY_KEY")
t.Cleanup(func() {
	if hadMyKey { os.Setenv("MY_KEY", origMyKey) } else { os.Unsetenv("MY_KEY") }
})
os.Unsetenv("MY_KEY")
```
→ `diag_test.go` must mirror this for `CRUNCHYROLL_LOG_LEVEL`/`CRUNCHYROLL_LOG_FILE` since `diag.Init` precedence (via `resolveString`) reads them.

**t.TempDir + writeTempFile pattern** (`internal/mux/mux_test.go:21-22, 145-152`):
```go
dir := t.TempDir()
videoFile := writeTempFile(t, dir, "video.mp4")
// writeTempFile uses os.WriteFile(path, []byte("test"), 0644)
```
→ `diag_test.go` uses `t.TempDir()` for the log-file path, then reads back the log file content to assert redaction (`[REDACTED]` substring present, raw secret absent). For D-10/D-11 os.Stat validation tests in `mux_test.go`, reuse `writeTempFile` and add a `writeEmptyFile` variant (`os.WriteFile(path, []byte{}, 0644)`).

---

### `internal/mux/mux_test.go`, `internal/download/episode_test.go`, `internal/config/config_test.go`, `main_test.go` (test, MODIFY/extend)

**Analog:** themselves — extend existing table-driven cases.

**mux_test.go** — add cases for ERR-04 os.Stat validation: `TestMergeEverythingRejectsEmptyVideo`, `TestMergeEverythingRejectsEmptyAudioTrack` (write 0-byte file, assert `err` contains "empty", assert ffmpeg never invoked — verified by NOT setting `GO_WANT_HELPER_PROCESS` and asserting no helper ran). Reuse `restoreFFmpegCommand` to make ffmpeg a sentinel that fails the test if called.

**episode_test.go** — add cases for D-03/D-04/D-05/D-06: primary missing = hard error, secondary missing = warn+skip, download failure = hard error. The episode_test.go existing structure (see `episode_test.go` file) is the analog; mirror its `api.Client` seam (likely a fake client) for missing-locale scenarios.

**config_test.go** — extend `TestMerge`-style cases for `LogLevel`/`LogFile` pointer overlay, and D-02 array-typed `AudioLang` backward-compat (single string accepted, array accepted).

**main_test.go** (likely NEW or extend) — `resolveString` precedence tests (`TestResolveString`): flag wins over env, env over config, config over default, default last. Mirror the env-isolation pattern from `config_test.go:41-57`.

---

## Shared Patterns

### Authentication / Authorization (not authenticated — but PII redaction is the analog guard)
**Source:** `internal/api/auth.go:16-31` (Basic Auth), `internal/api/client.go:91,122` (Bearer)
**Apply to:** ALL slog calls in download/drm/media/mux/api packages + `internal/diag/redact.go`
```go
// D-20/D-21 redaction set — apply via slog HandlerOptions.ReplaceAttr
var redactKeys = map[string]struct{}{
	"token": {}, "cookie": {}, "etp_rt": {}, "client_id": {},
	"private_key": {}, "authorization": {},
}
// D-21 EXPLICIT exclusions: device_id (UUID, not user PII), URLs, content IDs
```
Redaction lives in `internal/diag/redact.go` (D-22: "producers never think about PII"). Every subsystem logger is the child of a `Global` whose handler is `redactingHandler{inner: textHandler}` — so all 5 subsystem groups inherit redaction for free.

### Error Handling / Wrapping
**Source:** every `return fmt.Errorf("contextual: %w", err)` across `internal/download/episode.go`, `internal/mux/mux.go`, `internal/api/client.go`
**Apply to:** all new/modified service files
```go
// Hard errors (D-03, D-05, D-06, D-11): return fmt.Errorf("...: %w", err) — propagates to episode failure
// Soft skips (D-04, D-08): output.Global.Warn("skipping ...: %s", locale) + continue — NEVER return error
```
Established: `fmt.Errorf` wrapping (`%w`) for chained errors; `fmt.Errorf("...%v", value)` for value-only context. Files never `panic` (CONTEXT.md `code_context` "Established Patterns").

### Singleton Init Order
**Source:** `main.go:277-313`
**Apply to:** `main.go` (logger init), `internal/diag/diag.go` (Init func), `internal/diag/diag_test.go` (test isolation)
```go
// main.go init order (existing + Phase 6 insertion):
//   1. config.LoadDotenv (main.go:258)
//   2. flag.Parse + explicitFlags (main.go:264, 272-275)
//   3. output.Init (main.go:278-285)            ← output plane (no config dep)
//   4. config.Load (main.go:294-309)            ← cfg populated
//   5. resolveString for etpRt, outputDir (main.go:312-313)
//   6. [+NEW] resolveString for logLevel, logFile  ← AFTER cfg load
//   7. [+NEW] diag.Init(resolvedLogLevel, resolvedLogFile)  ← AFTER resolution
//   8. checkFFmpeg (main.go:316) — after logger ready so startup failures are logged
//   9. parseLangs (main.go:322-323)
```
**Critical ordering:** `diag.Init` MUST run after step 4 (config load) because `cfg.LogLevel`/`cfg.LogFile` are inputs to `resolveString`. Steps 6-7 are the ONLY new insertions; everything else is existing context.

### NDJSON Output Contract Stability (TUI-03 / D-08)
**Source:** `internal/output/ndjson.go:10-44` (event struct), `output.go:158-162` (jsonOutput.Warn → `Type:"warn"`)
**Apply to:** `internal/download/episode.go` (skip emission), `internal/mux/mux.go` (no NDJSON events needed there)
```go
// D-08: skip → output.Global.Warn("skipping %s dub: not available", locale)
//   in --json mode: emits event{Type:"warn", Message:"..."}  ← EXISTING shape, no new field
//   in human mode: yellow ⚠ line (output.go:120-124)         ← EXISTING render
// FORBIDDEN: adding Type:"skip", Partial bool, SkippedTracks []string to event struct
```

### Validation Pattern (os.Stat size-only)
**Source:** `internal/mux/mux.go:96` (`!os.IsNotExist(cleanupErr)`), `internal/mux/mux_test.go:33` (`os.Stat` assertion)
**Apply to:** `internal/mux/mux.go` (D-10/D-12), `internal/mux/mux_test.go` (empty-input cases)
```go
fi, err := os.Stat(path)
if err != nil        { return fmt.Errorf("mux input %s: %w", path, err) }
if fi.Size() == 0    { return fmt.Errorf("mux input %s is empty (0 bytes)", path) }  // D-10 size-only, no ffprobe
```
`os` already imported in mux.go (`:7`). No new dependency (CONTEXT.md `deferred`: ffprobe rejected).

## No Analog Found

Files with no close in-codebase match (planner falls back to D-decisions + slog stdlib reference):

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `internal/diag/redact.go` (NEW) | utility / slog.Handler impl | transform | No existing `slog.Handler` impl in repo (slog is new to this phase). `redactingHandler` is novel — modeled structurally on `output.Outputter` impls (concrete-struct-wrapping-sink pattern from `output.go:107-180`) but the slog.Handle/WithAttrs/WithGroup/Enabled tetrad has no precedent. Planner: use slog stdlib docs for the 4-method contract; use the struct-wrapping style from output.go for the body. |
| `internal/diag/diag.go` per-subsystem `WithGroup` accessors | utility / grouping | n/a | No existing multi-instance-grouped logger. `output.Global` is a single global, not per-subsystem. The 5 exported `*slog.Logger` vars (one per subsystem) are the new pattern — model the var-block on `output.Global` (line doc-comment style) but accept they're a Phase 6 invention per D-17. |
| `gopkg.in/natefinch/lumberjack.v2` integration | utility / rotation | file-I/O | NEW dep (D-16) — `go.mod` currently has no rotation lib. Planner action: `go get gopkg.in/natefinch/lumberjack.v2@latest`, add to `go.mod` `require` alongside `golang.org/x/term`. No codebase analog for the `lumberjack.Writer{...}` config; defer to library docs + D-15 values. |

## Metadata

**Analog search scope:** `internal/{output,config,download,mux,api}/`, `main.go`, `go.mod`, `go.sum`, plus all `*_test.go` under `internal/`.
**Files scanned:** 14 source/log files + 14 test files + go.mod/go.sum.
**Pattern extraction date:** 2026-07-10.
**Key observation:** Phase 6 is unusually self-analogous — 8 of 11 files are modifications to existing files that ALREADY contain the pattern being extended (config pointer-fields, resolveString precedence, output.Global singleton, MergeEverything arg-build, episode locale loop, NDJSON warn event). Only `internal/diag/{diag,redact}.go` and the lumberjack dep are truly novel; even those are structurally modeled on `internal/output/output.go`. Planner should expect MOST actions to read more like "extend lines X-Y of file F following the adjacent pattern" than "create new file copying analog A".