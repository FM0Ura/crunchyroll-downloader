# Phase 7: Organized Output + Folder Metadata - Context

**Gathered:** 2026-07-10
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase delivers a Jellyfin/Kodi-friendly organized output layout plus NFO metadata, Crunchyroll artwork, and a latent v1.0 MKV metadata bug fix. Output plane (`output.Global`) and diagnostic plane (`diag.*Logger`) remain separate (Phase 6 constraint).

**In scope:** OUT-01, OUT-02, OUT-03, META-01, META-02, META-03.
**Out of scope:** Season-specific posters (`season01-poster.jpg`, META-04 — deferred to v2), online TVDB/anidb NFO enrichment (Out-of-Scope table), online-DB uniqueid sources (`<uniqueid type="crunchyroll">` is enough), resumable `.part` segment state (separate Active requirement).

</domain>

<decisions>
## Implementation Decisions

### Folder Layout

- **D-01: No `(Year)` in the series root folder.** The folder structure is `Series Title/Season 01/Series Title S01E01 - Title.mkv` — no parenthetical year, ever. This deviates from OUT-01's literal `Series Title (Year)/` string and from Jellyfin's canonical disambiguation convention, but the user explicitly rejected adding a year. The deviation is acceptable because each series maps to a single Crunchyroll series ID and title collisions across distinct series are rare; a user can rename a folder manually if needed.
- **D-02: `Series Title` is the resumability key (no disambiguator).** OUT-02's "skip already-downloaded" check keys off the sanitized `Series Title` directory. No Crunchyroll series ID is appended to the folder name. Collision risk between two distinct Crunchyroll series that sanitize to the same title is accepted as low. (Mirrors v1.0 behavior at `internal/download/episode.go:83` which already does `os.Stat` on the output file.)
- **D-03: Single-episode download (`/watch/<id>`) mirrors the season layout.** Every download — single episode or full season — lands in the nested `Series Title/Season 01/` tree, gets `tvshow.nfo` written at the series root, and a per-episode `.nfo` beside the `.mkv`. Uniform structure means Jellyfin/Kodi always see the same tree regardless of how the user invoked the tool, and re-runs identify the series-root identically. (Moves single-episode output from `Series Title/<file>.mkv` flat to nested.)

### Filename

- **D-04: Drop `[{quality}]` from the output filename.** Files are named `Series Title S01E01 - Title.mkv` (no quality bracket). Aligns with OUT-01's literal success criterion. The `*videoQuality` value still drives media selection upstream (`media.GetVideoBaseUrl`) and is logged via `diag.DownloadLogger.Info("episode_start", ...)` per Phase 6 D-18; it just no longer appears in the on-disk filename. Jellyfin ignores any bracket, so dropping it has zero scraper impact.

### season_number Bug Fix (OUT-03)

- **D-05: One-line season_number bug fix.** The fix lands at `internal/mux/mux.go:108` — change `"season_number="+fmt.Sprintf("%v", info.EpisodeMetadata.EpisodeNumber)` to use `info.EpisodeMetadata.SeasonNumber`. `SeasonNumber` already exists on `api.EpisodeMetadata` (`internal/api/types.go:27`) and is populated for both single-episode and season flows (`internal/download/season.go:59`). This must land before any NFO emission so the NFO inherits the correct value. Trivial; included in this phase because the bug is explicitly listed in OUT-03 and must be fixed before metadata is mirrored into NFO.

### NFO Metadata

- **D-06: Per-episode `.nfo` enriches from the EXISTING `/content/v2/cms/objects/{id}` response.** `GetEpisodeInfo` (`internal/api/episode.go:47`) already calls this endpoint and decodes only `episode_metadata` + `title`. Enriching the per-episode NFO means extending the decoded `EpisodeMetadata` struct with additional fields the response already contains (plot/description, duration, ratings where applicable) — NOT adding a new API call. Researcher confirms which additional fields the endpoint returns before planner locks the struct shape.
- **D-07: `tvshow.nfo` requires a NEW series-level CMS call.** Episode-level metadata carries no series plot/genre/studio. The user wants a richer `tvshow.nfo`, so add `GET /content/v2/cms/objects/{seriesId}` (or the equivalent series endpoint). Researcher confirms the exact endpoint path and response shape (Jellyfin/Kodi NFO schema for tvshow: title, plot, genre, studio, ratings, uniqueid).
- **D-08: NFO uses `<uniqueid type="crunchyroll">` as the stable identifier.** Per META-02 (pre-locked). The Crunchyroll content ID is the uniqueid value. No TVDB/anidb lookup (Out-of-Scope).
- **D-09: NFO writing must not fail the download.** NFO write errors are non-fatal: `output.Global.Warn` + `diag.MuxLogger.Warn` (or a new `nfo` subsystem logger if cleaner) — the download still succeeds. Matches Phase 6 D-09 non-fatal warn convention and the META-03 "artwork 404 does not fail the download" principle extended to NFO.

### Artwork (META-03)

- **D-10: Artwork lives at the series root only.** `poster.jpg` and `backdrop.jpg` are written to `Series Title/` (the series root) — NOT per-season or per-episode. Season-specific posters (`season01-poster.jpg`) are explicitly META-04, deferred to v2.
- **D-11: Artwork URLs come from the existing CMS endpoint response.** Source artwork URLs from the same `/content/v2/cms/objects/{id}` response already fetched for episode info (likely an `images` field, researcher confirms). If the episode-level response does not carry usable poster/backdrop URLs, the series-level call added in D-07 is the fallback source. Avoid a dedicated images endpoint unless researcher confirms neither object response carries artwork.
- **D-12: 404 (or any artwork fetch failure) is non-fatal.** On a 404 or download error for artwork, emit `output.Global.Warn("No artwork available for %s: %v", seriesTitle, err)` AND `diag.APILogger.Warn("artwork_fetch_failed", "series", seriesTitle, "err", err)` — do NOT abort the episode/season. Artwork is best-effort; the MKV + NFO are the deliverable. (Pre-locked by META-03.)

### the agent's Discretion

- Exact struct field names added to `EpisodeMetadata` for the richer NFO (plot/description/duration/ratings) — research must confirm the Crunchyroll CMS response shape first.
- The exact series-level CMS endpoint path and whether it accepts the series ID or season ID (`/content/v2/cms/objects/{id}` vs `/series/{id}/...`) — researcher confirms.
- Whether a dedicated `nfo` subsystem logger is warranted, or NFO warnings ride `diag.MuxLogger` / `diag.APILogger`.
- NFO XML correctness: researcher verifies Jellyfin AND Kodi both scan the produced `tvshow.nfo` and per-episode `.nfo` (success criterion 4 calls for "a real Jellyfin and a real Kodi scan").
- Internal package location: extend `internal/mux/` with NFO code, new `internal/nfo/` package, or `internal/metadata/` — agent's call (mux is FFmpeg-focused; nfo is XML emission, may sit clean outside mux).
- Filename builder seam: whether to extract a single `buildOutputPath()` helper now (would help Phase 8 inject `[compressed]` later) or just edit `internal/download/episode.go:75` inline. Either is fine — the user did not require the seam.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & Roadmap
- `.planning/REQUIREMENTS.md` §Output Organization (OUT-01..OUT-03), §Metadata (META-01..META-03), and the Out-of-Scope table (season-specific posters META-04 deferred, online TVDB/anidb enrichment excluded, no online uniqueid sources).
- `.planning/ROADMAP.md` §"Phase 7: Organized Output + Folder Metadata" — goal, `Depends on: Phase 6` (uses the new non-fatal logger for artwork warnings), 5 success criteria.
- `.planning/PROJECT.md` — Key Decisions table (`output.Global` singleton, separate planes for Outputter vs slog), Constraints (Go 1.25, no new framework deps), Context (Phase 6 complete: `internal/diag/` package exists).

### Prior Phase Context
- `.planning/phases/06-track-hardening-structured-logging/06-CONTEXT.md` — D-08 (reuse `warn` NDJSON event / `output.Global.Warn` for non-fatal skips), D-09 (human warn format), D-13 (slog separate plane), D-17 (`log.Global` + `diag.*Logger` per-subsystem), D-18 (strategic events = Info), D-19 (resolveString precedence). Phase 7's artwork/NFO non-fatal warnings follow these.

### Codebase maps (note: dated 2026-07-08, pre-refactor to internal/ packages — structural details stale, but the `season_number=EpisodeNumber` bug and PII/singleton concerns are still valid)
- `.planning/codebase/ARCHITECTURE.md` §Output Layer / Media Multiplexing — `mergeEverything()` FFmpeg subprocess with `-metadata:g season_number=...` — the latent bug location.
- `.planning/codebase/CONCERNS.md` §Known Bugs — references the broader class of v1.0 bugs (OUT-03 is the metadata one fixed in this phase).

### Source files (live code — read before planning)
- `internal/download/episode.go` — output-path construction at lines 62-86 (folder + filename; the D-04 edit point and D-03 nesting change point), skip-already-downloaded at lines 83-86 (D-02 resumability key), `episodeMerge` call at line 340.
- `internal/mux/mux.go` — `MergeEverything` at line 30, the `season_number=EpisodeNumber` bug at line 108 (D-05 fix), FFmpeg `-metadata:g title/show/track/season_number` args at lines 104-110.
- `internal/api/types.go` — `EpisodeMetadata` (line 24), `SeasonEpisode` (line 42), `Season` (line 57). D-06 adds enrichment fields here; D-07 likely adds a new series-level response type.
- `internal/api/episode.go` — `GetEpisodeInfo` at line 47 (already calls `/content/v2/cms/objects/{id}` — the endpoint D-06 enriches from the existing response).
- `internal/api/season.go` — `GetSeasons` at line 45 (the season-list call; D-07's series-level call is a sibling here).
- `internal/config/config.go` — `Config` struct (line 13); no new config fields are required for Phase 7 unless research surfaces a need (e.g., `--no-artwork` opt-out, currently NOT requested).
- `internal/download/season.go` — season flow at line 47 (per-episode `downloadEpisode` invocation; D-03 single-episode-mirrors-season is implemented in `internal/download/episode.go`, but `Season` may gain a once-per-series `tvshow.nfo` + artwork write before the episode loop).
- `main.go` — URL dispatch (`processUrl`) and `--output-dir` flag; neither changes in Phase 7, but the design must keep `--output-dir` semantics intact (the new lay nests under `outputDir` if set).
- `internal/diag/diag.go` — `APILogger` / `MuxLogger` / `DownloadLogger` subsystem loggers; Phase 7 non-fatal artwork/NFO warnings route through these.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `output.Global.Warn` / `output.Global.Info` (`internal/output/output.go`) — the user-facing non-fatal warn channel for artwork 404 and NFO write failure (carried from Phase 6 D-09).
- `diag.APILogger` / `diag.MuxLogger` (`internal/diag/diag.go`) — slog subsystem loggers for non-fatal diagnostic events (Phase 6 D-17); artwork fetch failure and NFO write failure route here. A new `nfo` subsystem logger may be added if cleaner (agent's discretion per D-09 discretion note).
- `sanitizeFilename` (`internal/download/episode.go:49`) — already strips illegal filename chars and collapses double-underscores; reused for the nested folder name and filename (no new sanitization needed).
- `os.MkdirAll` + `os.Stat` skip check (`internal/download/episode.go:71-86`) — D-02 resumability reuses this exact pattern, just with a deeper path.
- `api.Client.Do` / `newRequest` / Bearer-token injection (`internal/api/episode.go`, `internal/api/client.go`) — any new series-level CMS call (D-07) plugs into the same authenticated client.
- `xml` stdlib — NFO emission needs `encoding/xml` (stdlib, no new dep; consistent with Constraints "Go 1.25, no new framework deps").

### Established Patterns
- **Pointer-field config + resolveString precedence (Phase 6 D-19):** not needed for Phase 7 unless a new flag emerges (none currently requested).
- **Error returns, not panic:** NFO/artwork errors must return-warn, never panic — consistent with v1.0 + Phase 6 conventions.
- **Table-driven stdlib tests only (v1.0 D-03):** any new `nfo` / `metadata` / `artwork` package gets stdlib tests; no testify.
- **Singleton init in main():** `output.Init` / `diag.Init` pattern; if an NFO subsystem logger is added it follows `diag` init in `main()`.
- **`internal/` package modularity:** newer packages follow `internal/<concern>/` shape — a new `internal/nfo/` or `internal/metadata/` follows the same convention (agent's discretion).

### Integration Points
- `internal/download/episode.go:62-86` — the single seam where output path is built; D-03/D-04 both edit here.
- `internal/mux/mux.go:108` — the OUT-03 bug fix location (D-05); must land before NFO emission.
- `internal/api/episode.go:47` (`GetEpisodeInfo` existing call) — D-06 enriches the decoded type of this same response (no new network call for episode NFO).
- `internal/api/season.go:45` (`GetSeasons`) — sibling location for the new series-level metadata call (D-07).
- `internal/download/season.go:47` (`runSeason` episode loop) — likely insertion point for a once-per-series `tvshow.nfo` + `poster.jpg`/`backdrop.jpg` write before the per-episode loop begins (avoids repeating artwork fetch per episode).
- `internal/download/episode.go:340` (`episodeMerge`) — somewhere after successful mux is the natural point to write the per-episode `.nfo` beside the `.mkv`.

</code_context>

<specifics>
## Specific Ideas

- Folder tree (final): `Series Title/Season 01/Series Title S01E01 - Title.mkv` — no year, no quality bracket, no series-id disambiguator.
- Single-episode flow produces the SAME nested tree + NFO + artwork as a season run — uniform for Jellyfin regardless of invocation.
- `tvshow.nfo` lives at `Series Title/tvshow.nfo` (series root); per-episode `.nfo` lives beside its `.mkv` (`Series Title/Season 01/Series Title S01E01 - Title.nfo`).
- Artwork (`poster.jpg`, `backdrop.jpg`) at `Series Title/` root only — never per-season, never per-episode.
- OUT-03 is the smallest task: one line at `internal/mux/mux.go:108`; must precede any NFO emission so NFO inherits correct season number.

</specifics>

<deferred>
## Deferred Ideas

- **Season-specific posters (`season01-poster.jpg`)** — META-04, deferred to v2 per REQUIREMENTS.md §Metadata (advanced). Phase 7 writes only series-root `poster.jpg`/`backdrop.jpg`.
- **Online TVDB/anidb NFO enrichment** — Out-of-Scope per REQUIREMENTS.md. `<uniqueid type="crunchyroll">` solves scraper mismatch without an external DB dependency.
- **`--no-artwork` opt-out flag** — NOT requested during discussion; artwork is always written when available and non-fatal when not. If a user wants no artwork they can delete the files. (Could be added in a future phase if requested.)
- **Filename builder seam for Phase 8 compression marker** — user dropped `[{quality}]` outright (D-04) rather than keeping a hook for future `[compressed]` vs `[copy]`. Phase 8 will reintroduce a filename seam if it needs to mark compression outcome in the name; that's Phase 8's call, not Phase 7's.
- **Resumable `.part` segment state (the broader "Resumable downloads" Active requirement)** — still unmapped to any phase; OUT-02 only preserves the v1.0 "skip already-downloaded episode" behavior in the new nested layout, not per-segment resume.

</deferred>

---

*Phase: 7-Organized Output + Folder Metadata*
*Context gathered: 2026-07-10*