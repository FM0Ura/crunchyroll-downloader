# Phase 7: Organized Output + Folder Metadata - Pattern Map

**Mapped:** 2026-07-10
**Files analyzed:** 10 (new + modified)
**Analogs found:** 9 / 10 (1 no-analog: NFO XML marshal — stdlib `encoding/xml` referenced via research)
**Codebase map caveat:** `.planning/codebase/` is dated 2026-07-08 (pre-`internal/` refactor). All excerpts below are from **live source** under `internal/` (current as of Phase 6 completion) and supersede any structural detail in the dated map.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/mux/mux.go` (MODIFY line 108) | service | subprocess/transform | itself (`MergeEverything` lines 104-110) | exact (in-file) |
| `internal/download/episode.go` (MODIFY lines 62-86, 340) | controller | request-response + file-I/O | itself (path-build lines 62-86; mux call 340) | exact (in-file) |
| `internal/api/types.go` (MODIFY `EpisodeMetadata`; ADD series type) | model | serialization | itself (`Seasons`/`Season` lines 53-60) | exact (in-file) |
| `internal/api/episode.go` (no code change beyond enriched decode) | service | request-response | itself (`GetEpisodeInfo` lines 47-75) | exact (in-file) |
| `internal/api/season.go` (ADD `GetSeriesInfo`) | service | request-response | `GetSeasons` (same file lines 45-77) | exact (in-file sibling) |
| `internal/download/season.go` (MODIFY `runSeason` pre-loop) | controller | request-response + file-I/O | itself (`runSeason` lines 47-52) | exact (in-file) |
| `internal/nfo/` NEW package (NFO XML emission) | utility/service | file-I/O + marshal | `internal/api/types.go` (struct-tag convention) + stdlib `encoding/xml` | partial (no `encoding/xml` use in repo) |
| `internal/api/artwork.go` NEW (artwork fetch helper) | service | request-response + file-I/O | `media.DownloadSubs` (`internal/media/segment.go:303-332`) | role+data-flow match |
| `main.go` (MODIFY only if `nfo` subsystem logger added) | config/init | startup | `diag.Init` call at `main.go:335` + `diag.go:74-80` | exact (in-file) |
| `internal/nfo/nfo_test.go` + `internal/api/artwork_test.go` NEW | test | unit | `internal/api/season_test.go` + `internal/download/episode_test.go` seam pattern | role-match |

---

## Pattern Assignments

### `internal/mux/mux.go` (service, subprocess/transform) — D-05 one-line fix

**Analog:** itself, `MergeEverything` metadata args.

**The bug line + its neighbors** (lines 104-110):
```go
args = append(args,
	"-metadata:g", "title="+fmt.Sprintf("S%02vE%02v - %s", info.EpisodeMetadata.SeasonNumber, info.EpisodeMetadata.EpisodeNumber, info.Title),
	"-metadata:g", "show="+info.EpisodeMetadata.SeriesTitle,
	"-metadata:g", "track="+fmt.Sprintf("%v", info.EpisodeMetadata.EpisodeNumber),
	"-metadata:g", "season_number="+fmt.Sprintf("%v", info.EpisodeMetadata.EpisodeNumber),  // <-- BUG (line 108): uses EpisodeNumber
	outputFile,
)
```

**D-05 fix:** replace `info.EpisodeMetadata.EpisodeNumber` on line 108 with `info.EpisodeMetadata.SeasonNumber`. `SeasonNumber` already exists on `api.EpisodeMetadata` (`internal/api/types.go:27`) and is populated in both flows (`internal/download/season.go:59`, `episode.go:77`). Keep `fmt.Sprintf("%v", ...)` formatting unchanged.

**Error/cleanup pattern** (lines 115-121) — do NOT change; the file already removes partial output on FFmpeg failure:
```go
if err := cmd.Run(); err != nil {
	cleanupErr := os.Remove(outputFile)
	if cleanupErr != nil && !os.IsNotExist(cleanupErr) {
		return fmt.Errorf("ffmpeg failed: %w: %s; cleanup output: %v", err, stderr.String(), cleanupErr)
	}
	return fmt.Errorf("ffmpeg failed: %w: %s", err, stderr.String())
}
```

**Subprocess seam for tests** (lines 21, 112): the package-level var `ffmpegCommand = exec.CommandContext` is the test seam; override in tests via the `restoreFFmpegCommand` helper (see `mux_test.go:140-157`). NFO phase must NOT add a new subprocess seam — NFO uses in-process `encoding/xml`.

**Test analog** (`internal/mux/mux_test.go`): `testEpisodeInfo()` helper (lines 238-247) shows the canonical `*api.EpisodeInfo` fixture — reuse it for any new D-05 regression test (`TestMergeEverythingSetsCorrectSeasonNumber` would assert the `season_number` arg value via a captured args seam).

---

### `internal/download/episode.go` (controller, request-response + file-I/O) — D-03, D-04, per-episode NFO

**Analog:** itself, the output-path construction block.

**Current path-build pattern** (lines 62-86) — D-03/D-04 edit point:
```go
cleanSeriesTitle := sanitizeFilename(info.EpisodeMetadata.SeriesTitle)
cleanEpisodeTitle := sanitizeFilename(info.Title)

outputBase := cleanSeriesTitle
if outputDir != "" {
	outputBase = filepath.Join(outputDir, cleanSeriesTitle)
}

if err := os.MkdirAll(outputBase, 0777); err != nil {
	return fmt.Errorf("creating output directory: %w", err)
}

outputFile := filepath.Join(outputBase, fmt.Sprintf("%s S%02dE%02d - %s [%s].mkv",
	cleanSeriesTitle,
	info.EpisodeMetadata.SeasonNumber,
	info.EpisodeMetadata.EpisodeNumber,
	cleanEpisodeTitle,
	*videoQuality,   // <-- D-04: drop this + the "[%s]" bracket
))

if _, err := os.Stat(outputFile); err == nil {   // <-- D-02 resumability key
	output.Global.Info("[Episode %d/%d] %s ... skipped (already downloaded)", info.EpisodeMetadata.EpisodeNumber, totalEpisodes, info.Title)
	return nil
}
```

**D-03 nested-layout target shape:**
```go
seasonDir := fmt.Sprintf("Season %02d", info.EpisodeMetadata.SeasonNumber)
seriesDir := outputBase            // = cleanSeriesTitle (+outputDir prefix)
seasonPath := filepath.Join(seriesDir, seasonDir)
if err := os.MkdirAll(seasonPath, 0777); err != nil {
	return fmt.Errorf("creating season directory: %w", err)
}
outputFile := filepath.Join(seasonPath, fmt.Sprintf("%s S%02dE%02d - %s.mkv",  // no [%s]
	cleanSeriesTitle, info.EpisodeMetadata.SeasonNumber, info.EpisodeMetadata.EpisodeNumber, cleanEpisodeTitle))
```
The `os.Stat` skip check (D-02) reuses the **deeper** `outputFile` path verbatim — no logic change, just the path prefix shifts from `seriesDir` to `seasonPath`. `--output-dir` semantics preserved: `outputBase` still nests under `outputDir` if set.

**Per-episode NFO insertion point** (line 340-343): after a successful merge, before the success log:
```go
if err := episodeMerge(ctx, videoFile, audioTracks, subTracks, outputFile, info); err != nil {
	return fmt.Errorf("muxing episode: %w", err)
}
completed = true
// <-- D-06/D-08: write per-episode .nfo beside the .mkv HERE (non-fatal)
```
The `.nfo` path is `strings.TrimSuffix(outputFile, ".mkv") + ".nfo"`. NFO write failure is non-fatal — see **Shared Patterns §Non-fatal Warn**.

**New seam for testability:** the `var (...)` seam block at lines 27-47 (e.g. `episodeMerge = mux.MergeEverything`) is the place to add `episodeWriteNfo = nfo.WriteEpisode` (package-level indirection var). Tests override it the same way `episodeMerge` is overridden in `episode_test.go:407-409`.

**`sanitizeFilename`** (lines 49-60) — REUSE for nested folder name + filename; no new sanitization:
```go
func sanitizeFilename(s string) string {
	if s == "" { return "Unknown" }
	illegal := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|", "'", "’", "`", "“", "”"}
	res := s
	for _, char := range illegal { res = strings.ReplaceAll(res, char, "_") }
	res = multiUnderscore.ReplaceAllString(res, "_")
	return strings.TrimRight(res, " .")
}
```

**Test analog** (`internal/download/episode_test.go`):
- `TestOutputDirCreatesSeriesSubfolderInOutputDir` (lines 262-294) is the structural template; extend with a nested-season assertion by adapting `expectedDir := filepath.Join(outputDir, sanitizeFilename("Test Series"))` → add `/Season 01`.
- `restoreEpisodeTestSeams` (lines 366-422) is the seam-injection pattern for any new `episodeWriteNfo` / `episodeWriteArtwork` seam.
- `TestSanitizeFilenameCollapsesMultiUnderscore` (lines 238-260) is the table-driven test shape.

---

### `internal/api/types.go` (model, serialization) — D-06, D-07

**Analog:** itself, the `Season`/`Seasons` struct pair (the closest sibling to a new series-level type).

**Current type pair pattern** (lines 53-60):
```go
type Seasons struct {
	Data []Season `json:"data"`
}

type Season struct {
	ID           string `json:"id"`
	SeasonNumber int    `json:"season_number"`
}
```

**D-06 enrich `EpisodeMetadata`** (lines 24-31): add fields per researcher's confirmation of the `/content/v2/cms/objects/{id}` response. Keep the existing json tags; new fields follow the same lowercase-snake convention:
```go
type EpisodeMetadata struct {
	AudioLocale        string        `json:"audio_locale"`
	EpisodeNumber      int           `json:"episode_number"`
	SeasonNumber       int           `json:"season_number"`
	SeriesTitle        string        `json:"series_title"`
	AvailabilityStarts string        `json:"availability_starts"`
	Versions           []*DubVersion `json:"versions"`
	// NEW (D-06): the response already contains these — researcher confirms exact keys
	// e.g. Description string `json:"description"` / Synopsis string `json:"synopsis"`
	//      DurationMs int    `json:"duration_ms"`    / Rating ... `json:"rating"`
	//      Slug string `json:"slug"` (the Crunchyroll content id used as <uniqueid>)
}
```
**Reminder to planner:** the episode-level enrichment adds NO new HTTP call — `GetEpisodeInfo` (`internal/api/episode.go:47`) already decodes this body; extending the struct is sufficient because `json.Unmarshal` ignores unknown fields by default and populates newly-tagged fields automatically.

**D-07 new series-level type** — add a sibling pair modeled on `Seasons`/`Season`:
```go
type SeriesInfoResponse struct {
	Data []SeriesInfo `json:"data"`
}

type SeriesInfo struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	// researcher confirms: Description/Synopsis, Genres []string, Studio.provider.name,
	// Rating, Images ... — used for tvshow.nfo <plot>/<genre>/<studio>/<ratings>
}
```

---

### `internal/api/episode.go` (service, request-response) — D-06 decode only

**Analog:** itself, `GetEpisodeInfo` is the canonical authenticated GET pattern.

**Canonical `episodes`-info call** (lines 47-75) — NO code change here; the enriched `EpisodeMetadata` struct (types.go) auto-populates via `json.Unmarshal`:
```go
func (c *Client) GetEpisodeInfo(ctx context.Context, id string) (*EpisodeInfo, error) {
	req, err := c.newRequest(ctx, http.MethodGet,
		c.url(fmt.Sprintf("/content/v2/cms/objects/%s?ratings=true&preferred_audio_language=ja-JP&locale=en-US", id)), nil)
	if err != nil { return nil, err }
	resp, err := c.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil { return nil, err }
	var info EpisodeMetadataResponse
	if err := json.Unmarshal(body, &info); err != nil { return nil, err }
	if len(info.Data) == 0 {
		return nil, fmt.Errorf("no episode info found for id: %s", id)
	}
	return &info.Data[0], nil
}
```
**Note the query-string flags already there:** `?ratings=true` — D-06's ratings enrichment likely needs no extra request parameter.

**Test analog** (`internal/api/episode_test.go` lines 11-34): template for any new enrich-field assertion test — `httptest.NewServer` + `newTestClient(server.URL)` + a JSON body matching the enriched response shape; assert the decoded field equals the expected value.

---

### `internal/api/season.go` (service, request-response) — D-07 NEW `GetSeriesInfo`

**Analog:** `GetSeasons` is the closest sibling — same package, same file, same authenticated-CMS-series pattern. Copy verbatim, swap endpoint + decoded type.

**Pattern to copy** (lines 45-77):
```go
func (c *Client) GetSeasons(ctx context.Context, contentId, audioLocale, subLocale string) ([]Season, error) {
	if audioLocale == "" { audioLocale = "ja-JP" }
	if subLocale == ""    { subLocale = "en-US" }

	req, err := c.newRequest(ctx, http.MethodGet,
		c.url(fmt.Sprintf("/content/v2/cms/series/%s/seasons?force_locale=&preferred_audio_language=%s&locale=%s",
			contentId, audioLocale, subLocale)), nil)
	if err != nil { return nil, err }
	resp, err := c.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil { return nil, err }
	var seasons Seasons
	if err := json.Unmarshal(body, &seasons); err != nil { return nil, err }
	return seasons.Data, nil
}
```

**Target for `GetSeriesInfo`:** same skeleton; researcher confirms whether the path is `/content/v2/cms/objects/{seriesId}` (matching episode-info) or `/content/v2/cms/series/{id}` (matching `GetSeasons`' path convention). The decoded type is the NEW `SeriesInfoResponse` from `types.go`. Locale-default guards (`audioLocale`/`subLocale`) preserved for consistency.

**Test analog** (`internal/api/season_test.go` lines 11-34) — copy verbatim, change path assertion to the series endpoint and the JSON body to a `{"data":[{"id":"...","title":"..."}]}` shape:
```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/content/v2/cms/series/G123456789/seasons" { ... }
	if got := r.Header.Get("Authorization"); got != "Bearer initial-token" { ... }
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"data":[{"id":"season-1","season_number":1}]}`)
}))
```

---

### `internal/download/season.go` (controller, request-response + file-I/O) — once-per-series hook

**Analog:** itself, `runSeason`.

**The episode loop** (lines 47-76): the once-per-series `tvshow.nfo` + `poster.jpg`/`backdrop.jpg` write belongs BEFORE the `for _, ep := range episodes` loop (line 55), after the empty-episodes guard (line 48) and the "Downloading season N..." info (line 52):
```go
func runSeason(ctx context.Context, client *api.Client, videoQuality, audioQuality *string, audioLangs, subsLangs []string, episodes []api.SeasonEpisode, workers int, outputDir string, downloadEpisode episodeDownloader) error {
	if len(episodes) == 0 { return nil }
	output.Global.Info("Downloading season %d of %s (%d episodes)", episodes[0].SeasonNumber, episodes[0].SeriesTitle, len(episodes))

	// <-- D-07/D-10/D-12: fetch series info, write tvshow.nfo + poster/backdrop
	//     to the SERIES root ONCE (not per season, not per episode).
	//     Non-fatal on 404/NFO error (see Shared Patterns §Non-fatal Warn).

	var failures []episodeError
	for _, ep := range episodes { ... }
```
**Critical decision (per CONTEXT D-10):** if multiple seasons are downloaded in one invocation (`main.go:199-208` loops seasons), `tvshow.nfo` should be written once-per-series-root, NOT once-per-season. The planner must decide whether to hoist the series-metadata call out of `runSeason` to `main.go`'s season loop, OR guard the `tvshow.nfo` write with an `os.Stat` existence check inside `runSeason` (mirroring D-02 resumability). The latter keeps `runSeason` self-contained — recommended pattern, consistent with `episode.go:83` skip idiom.

**Seam-injection pattern** (`season.go:13`): the `episodeDownloader` function type is the seam template for a new `seriesMetadataWriter func(ctx, client, seriesTitle, outputBase string) error` parameter (or package-level var). The test file already overrides it via the inline-func argument at `season_test.go:74-87`.

**Test analog** (`internal/download/season_test.go` lines 50-108): `TestRunSeasonContinuesAfterEpisodeFailure` is the seam-injection template — the inner `func(...)` is replaced; assert it was called once with the expected `seriesTitle`/`outputBase`.

---

### `internal/nfo/` NEW package (utility/service, file-I/O + marshal)

**Analog:** NO `encoding/xml` use exists in the repo. Package-shape analog: `internal/diag/diag.go`. Marshal-style analog: `encoding/json` struct tags in `internal/api/types.go`.

**Package-shape reference** (`internal/diag/diag.go` — singleton package pattern): a small package with exported entry functions and a package-level seam var for tests:
```go
package diag
var Global *slog.Logger = slog.New(discardHandler{})  // line 17
func Init(level slog.Level, path string) { ... }        // line 29
var (
	DownloadLogger *slog.Logger   // line 21
	...
)
```
Apply the same shape to `internal/nfo/`:
```go
package nfo
// WriteTVShow(ctx, path string, info *api.SeriesInfo) error       — emits tvshow.nfo
// WriteEpisode(ctx, path string, info *api.EpisodeInfo) error      — emits per-episode .nfo
// (optional package-level seam vars for test override, mirroring episode.go:46)
```

**Marshal pattern** (modeled on `internal/api/types.go:24-31` json tags — swap `json:` for `xml:`):
```go
// internal/nfo/types.go (or types in nfo.go)
type tvshow struct {
	XMLName  xml.Name `xml:"tvshow"`
	Title    string   `xml:"title"`
	Plot     string   `xml:"plot"`
	Genre    []string `xml:"genre"`     // repeated element
	Studio   string   `xml:"studio"`
	UniqueID uniqueID `xml:"uniqueid"`
	// ...ratings
}
type uniqueID struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}
```
Emission via stdlib (PITFALLS.md Pitfall 6 — NEVER `fmt.Sprintf`):
```go
out, err := xml.MarshalIndent(doc, "", "  ")
if err != nil { return err }
out = append([]byte(xml.Header), out...)
return os.WriteFile(path, out, 0644)
```
`xml.MarshalIndent` + `xml.Header` gives free entity escaping (`&`→`&amp;`, `<`→`&lt;`) — the exact property that prevents the `Hidamari Sketch × Honeycomb`/`Re:Zero` injection bug (per research STACK.md lines 29/137). The `<uniqueid type="crunchyroll">...</uniqueid>` attribute pair uses `xml:"type,attr"` + `xml:",chardata"`.

**Imports convention** (no project path aliases; stdlib + internal concern imports):
```go
import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"

	"crunchyroll-downloader/internal/api"
	"crunchyroll-downloader/internal/diag"   // if a new NfoLogger is added (agent's discretion)
	"crunchyroll-downloader/internal/output"
)
```

**Test analog** (`internal/diag/diag_test.go:11-33`): table-driven stdlib tests with `t.Run` sub-tests:
```go
tests := []struct {
	name string
	in   string
	want slog.Level
}{ ... }
for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) { ... })
}
```
NFO tests must include cases for `&`, `<`, `>`, `"`, `'`, emoji, CJK in title/plot fields; assert the marshaled output contains the entity-escaped form (`&amp;`, `&lt;`) per PITFALLS.md Pitfall 6.

---

### `internal/api/artwork.go` NEW (service, request-response + file-I/O) — D-10, D-11, D-12

**BEST analog:** `media.DownloadSubs` (`internal/media/segment.go:303-332`) — exactly an authenticated GET → `io.ReadAll` → `os.WriteFile` to a caller-named destination, using the `httpDoer` interface for testability.

**Pattern to copy** (`internal/media/segment.go:303-332`):
```go
func DownloadSubs(ctx context.Context, client httpDoer, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil { return "", err }
	req.Header.Set("Origin", "https://static.crunchyroll.com")
	req.Header.Set("Referer", "https://static.crunchyroll.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")

	resp, err := client.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil { return "", err }
	filename, err := getFilename(nil)
	if err != nil { return "", err }
	if err := os.WriteFile(filename, body, 0644); err != nil { return "", err }
	return filename, nil
}
```
**`httpDoer` interface** (`segment.go:27-29`): `type httpDoer interface { Do(*http.Request) (*http.Response, error) }` — `*api.Client` satisfies it.

**D-12 (404 non-fatal) adaptation:** the artwork helper must NOT return a hard error on 404 — wrap with status-check before draining body:
```go
resp, err := client.Do(req)
if err != nil { return err }       // transport-level: non-fatal at caller
defer resp.Body.Close()
if resp.StatusCode == http.StatusNotFound {
	return ErrArtworkNotFound   // sentinel — caller logs warn, continues
}
if resp.StatusCode != http.StatusOK {
	return fmt.Errorf("artwork fetch: status %d", resp.StatusCode)
}
out, err := io.ReadAll(resp.Body)
// ... os.WriteFile(path, out, 0644)
```
The caller (in `download/season.go` or `download/episode.go`) emits BOTH `output.Global.Warn("No artwork available for %s: %v", seriesTitle, err)` AND `diag.APILogger.Warn("artwork_fetch_failed", ...)` per CONTEXT D-12, then returns nil — see **Shared Patterns §Non-fatal Warn**.

**Authenticated-request analog** (`internal/api/client.go:84-95`) — for using the bearer-token client rather than the static-CDN headers `DownloadSubs` uses:
```go
func (c *Client) newRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	if ctx == nil { ctx = context.Background() }
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil { return nil, err }
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}
```
If the artwork helper is a method on `*api.Client` (recommended — reuses bearer retry in `c.Do` at `client.go:97-138`), use `c.newRequest` + `c.Do` exactly like `GetEpisodeInfo`. A standalone function in `internal/api/artwork.go` is also acceptable; pick the method form to inherit the 401-retry transparently.

**Test analog** (`internal/api/episode_test.go:11-34` & `internal/media/segment_test.go:72-95`): `httptest.NewServer` returning a 404 handler — assert the helper returns `ErrArtworkNotFound` and the caller-path warns. The 404-case test is the highest-value Phase 7 test (per CONTEXT success criterion "artwork 404 does not fail the download").

---

### `main.go` (config/init, startup) — conditional, only if new `nfo` subsystem logger

**Analog:** the existing `diag.Init` call and subsystem-logger population.

**Existing seam** (`main.go:335`):
```go
diag.Init(diag.ParseLevel(resolvedLogLevel), resolvedLogFile)
```
**Subsystem-logger population analog** (`internal/diag/diag.go:74-80`):
```go
func initSubsystemLoggers() {
	DownloadLogger = Global.WithGroup("download")
	DrmLogger      = Global.WithGroup("drm")
	MediaLogger    = Global.WithGroup("media")
	MuxLogger      = Global.WithGroup("mux")
	ApiLogger      = Global.WithGroup("api")
}
```
**Decision (agent's discretion, CONTEXT D-09 note):** the simplest path is NFO warnings ride `diag.MuxLogger` (already initialized; no `main.go` change). If a dedicated `diag.NfoLogger` is added: add `NfoLogger *slog.Logger` to `diag.go:19-26`, add `NfoLogger = Global.WithGroup("nfo")` to `initSubsystemLoggers`, and **NO `main.go` change is needed** — `diag.Init(...)` already triggers `initSubsystemLoggers()`. So `main.go` modification is OPTIONAL only if a new flag emerges (none requested — defer).

**Must NOT change:** `--output-dir` flag (line 32), `processURL` dispatch (lines 126-211), `validateOutputDir` (lines 66-80). The new nested layout nests under `outputDir` if set — already handled inside `episode.go:67-69`.

---

## Shared Patterns

### Error Handling (returns, never panic)
**Source:** `internal/download/episode.go:164` (and everywhere); `internal/mux/mux.go:48-52`
**Apply to:** ALL new/modified files in Phase 7.
```go
// Every error path wraps with %w — never log-and-continue for hard errors:
return fmt.Errorf("creating output directory: %w", err)
return fmt.Errorf("mux input %s: %w", path, err)
```
NFO/artwork errors are **non-fatal** (see below); all other errors return up.

### Non-fatal Warning (Phase 6 D-09 pattern, extended to NFO/artwork)
**Source:** `internal/download/episode.go:125-128` & `internal/download/episode.go:187-190`
**Apply to:** artwork fetch failure (`download/season.go` pre-loop), NFO write failure (`download/episode.go` post-merge), `tvshow.nfo` write failure (`download/season.go` pre-loop).
```go
output.Global.Warn("Skipping %s audio: not available for episode %d", locale, info.EpisodeMetadata.EpisodeNumber)
if diag.DownloadLogger != nil {
	diag.DownloadLogger.Warn("skipped track", "episode", info.EpisodeMetadata.EpisodeNumber, "kind", "audio", "locale", locale)
}
```
**Phase 7 dual-channel warn convention** (per CONTEXT D-09/D-12): every non-fatal event goes to BOTH `output.Global.Warn` (user plane) AND `diag.<Subsystem>Logger.Warn` (diagnostic plane). The nil-guard `if diag.XLogger != nil` is mandatory (matches Phase 6 D-17). For artwork use `diag.APILogger.Warn("artwork_fetch_failed", "series", seriesTitle, "err", err)`; for NFO write use `diag.MuxLogger.Warn` (or new `diag.NfoLogger` — agent's discretion).

### Resumability Skip (D-02)
**Source:** `internal/download/episode.go:83-86`
**Apply to:** episode `.mkv` (unchanged), per-episode `.nfo` (new — same `os.Stat` shape OR write-overwrite), `tvshow.nfo` (new — `os.Stat` guard so multi-season invocations don't re-fetch series info).
```go
if _, err := os.Stat(outputFile); err == nil {
	output.Global.Info("[Episode %d/%d] %s ... skipped (already downloaded)", ... )
	return nil
}
```

### MkdirAll + Path Build
**Source:** `internal/download/episode.go:71-73`
**Apply to:** `episode.go` (deeper path: `Series Title/Season 01/`), `season.go` pre-loop (`Series Title/` for artwork + `tvshow.nfo`).
```go
if err := os.MkdirAll(outputBase, 0777); err != nil {
	return fmt.Errorf("creating output directory: %w", err)
}
```

### Bearer-Token Authenticated GET
**Source:** `internal/api/client.go:84-95` (request build) + `internal/api/client.go:97-138` (Do with auto-retry on 401)
**Apply to:** new `GetSeriesInfo` method (`internal/api/season.go`), artwork fetch helper (`internal/api/artwork.go`).
```go
req, err := c.newRequest(ctx, http.MethodGet, c.url(...), nil)
// resp, err := c.Do(req)   // inherits 401 refresh+retry for free
```

### Sanitize Filename
**Source:** `internal/download/episode.go:49-60`
**Apply to:** nested folder name (`Series Title/`), season folder (`Season 01/` is static — no sanitize needed), episode filename (unchanged). NO new sanitization helper.
```go
cleanSeriesTitle := sanitizeFilename(info.EpisodeMetadata.SeriesTitle)
```

### Seam Injection for Testability
**Source:** `internal/download/episode.go:27-47` (package-level func vars), `internal/mux/mux.go:21` (`ffmpegCommand` var)
**Apply to:** any new helper called from the episode/season pipeline (`episodeWriteNfo`, `episodeFetchArtwork`, `seriesWriteMetadata`). Add a package-level `var X = pkg.Func` in `episode.go`/`season.go`; override in tests, restore via `t.Cleanup`.
```go
var (
	episodeMerge     = mux.MergeEverything
	episodeWriteNfo  = nfo.WriteEpisode        // NEW Phase 7 seam
	episodeFetchArtwork = api.FetchArtwork     // NEW Phase 7 seam
)
```
**Test restoration analog:** `internal/download/episode_test.go:366-422` (`restoreEpisodeTestSeams`).

### Table-Driven stdlib Tests (no testify)
**Source:** `internal/diag/diag_test.go:11-33`, `internal/download/episode_test.go:238-260`, `internal/mux/mux_test.go:172-189`
**Apply to:** `internal/nfo/nfo_test.go`, `internal/api/artwork_test.go`, any new season/episode flow tests.
```go
tests := []struct {
	name  string
	input string
	want  string
}{
	{name: "...", input: "...", want: "..."},
}
for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		if got := f(tt.input); got != tt.want {
			t.Fatalf("f(%q) = %q, want %q", tt.input, got, tt.want)
		}
	})
}
```
**Mandatory NFO test cases** (per PITFALLS.md Pitfall 6): title/plot containing `&`, `<`, `>`, `"`, `'`, CJK (`×`), emoji — assert escaped output.

### httptest Server + newTestClient
**Source:** `internal/api/episode_test.go:11-27`, `internal/api/season_test.go:11-27`, `internal/api/client_helper.go:13-22`
**Apply to:** `GetSeriesInfo` test, artwork-helper 404 test.
```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/content/v2/cms/..." { t.Fatalf(...) }
	if got := r.Header.Get("Authorization"); got != "Bearer initial-token" { t.Fatalf(...) }
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"data":[...]}`)
}))
defer server.Close()
client := newTestClient(server.URL)   // exported api.NewTestClient for cross-package tests
```

---

## No Analog Found

| File/Concern | Role | Data Flow | Reason | Planner Fallback |
|------|------|-----------|--------|------------------|
| NFO XML struct→marshal emission (`internal/nfo/`) | utility | file-I/O + marshal | No `encoding/xml` use anywhere in repo; only `encoding/json` struct-tag convention exists (`internal/api/types.go`) | Use stdlib `encoding/xml` + `xml.MarshalIndent` + `xml.Header` per research STACK.md lines 29/122/137. Marshal-style modeled on `internal/api/types.go` json tags. PITFALLS.md Pitfall 6 mandates `encoding/xml` (NEVER `fmt.Sprintf`). |
| Series-level CMS response shape | model | serialization | Endpoint path + response fields unconfirmed (D-07 discretion note) — research skipped for this phase | Planner defers exact field names to researcher confirmation OR marks as a discovery spike in PLAN.md. Use `Seasons`/`Season` (`types.go:53-60`) as the structural template once fields are known. |
| Binary (image) download to caller-named path via `*api.Client` | service | request-response + file-I/O | No existing authenticated-binary-download caller-named-path method — `media.DownloadSubs` uses static-CDN headers (no bearer) and writes to a temp file | Combine `api.Client.newRequest`/`Do` (bearer + 401 retry) with `media.DownloadSubs`' body→`os.WriteFile` shape. Method-on-Client form recommended to inherit retry. |

---

## Metadata

**Analog search scope:**
- `internal/mux/` (`mux.go`, `mux_test.go`)
- `internal/download/` (`episode.go`, `episode_test.go`, `season.go`, `season_test.go`)
- `internal/api/` (`types.go`, `episode.go`, `season.go`, `client.go`, `client_helper.go`, `*_test.go`)
- `internal/diag/` (`diag.go`, `diag_test.go`)
- `internal/output/` (`output.go`)
- `internal/media/` (`segment.go` — `DownloadSubs`, `httpDoer`)
- `main.go`

**Files scanned:** 16 live source + test files
**Pattern extraction date:** 2026-07-10
**Codebase map reference (dated, structural-only):** `.planning/codebase/ARCHITECTURE.md` §Output Layer / Media Multiplexing — used only for the OUT-03 bug location cross-reference; all excerpts above are from live `internal/` source.