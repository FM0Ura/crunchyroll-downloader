# Feature Landscape

**Domain:** CLI media downloader (Go) — adding compression, TUI, error hardening, organized output, folder metadata, structured logging to an existing v1.0 anime downloader
**Researched:** 2026-07-10
**Overall confidence:** HIGH (grounded in existing codebase + Go stdlib/Bubble Tea/slog/FFmpeg/Jellyfin conventions)

## Scope Note

This landscape covers ONLY the six v1.1 milestone features. All v1.0 features (download/decrypt/mux pipeline, batch URLs, quality selection, multi-audio/sub, token refresh, SIGINT handling, config file, --quiet/--json) are already shipped and treated as **dependencies**, not features to re-evaluate.
Existing pipeline anchor points (verified in code):
- `internal/mux/mux.go` `MergeEverything` — currently `-c:v copy -c:a copy -c:s copy` (no re-encode). Compression plugs in here.
- `internal/download/episode.go` lines 96-98 (audio locale missing → hard `return error`) and 142-144 (subtitle missing → hard `return error`). Graceful handling plumbs here.
- `internal/download/episode.go` lines 42-57 — flat `ShowName/ShowName S01E01 - Title [quality].mkv` layout. Organized output + metadata target here.
- `internal/output/output.go` `Outputter` interface + `humanOutput`/`jsonOutput`/`quietOutput` + `Global` singleton. TUI + structured logging interact with this.

---

## Table Stakes

Features users expect from a v1.1 "storage + CLI modernization" release of a download tool. Missing = feels incomplete or regressions vs v1.0 expectations.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Compression: lossless/copy mode (default)** | Must not lose quality by surprise; users downloading for archival expect copy to remain the default. v1.0 is `-c copy`. | Low | Already exists. Compression must be **opt-in**. The "copy" preset is table stakes; re-encode presets are the differentiator. |
| **Backward-compatible flags** | PROJECT.md constraint: "CLI flags and behavior must remain backward compatible." `--quality`, `--audio`, `--subs`, `--output-dir`, `--quiet`, `--json`, `--workers` must keep working untouched. | Low | TUI/compression/logging add flags (`--tui`, `--compress <preset>`, `--log-file`, `--log-level`) — must NOT rename/break existing ones. |
| **FFmpeg availability check extended to encoder** | v1.0 already checks FFmpeg exists at startup. If a compression preset needs `libx265`/`libvpx`, must verify the build supports it before starting (fail fast, not after downloading GBs). | Med | Probe `ffmpeg -encoders` or test-encode a tiny input. Surface a clear error: "preset 'hevc' requires libx265; your ffmpeg build lacks it." |
| **Graceful missing audio/subtitle: warn + skip optional tracks** | Users asking for `--subs en-US,es-ES` on an episode that only has `en-US` currently get a hard abort halfway through. A downloader should warn and skip, not nuke the whole run. | Med | Convert the two hard `return error` sites into warn+continue for non-primary tracks. Distinguish "all audio missing" (real error) from "1 of N subs missing" (warn). |
| **Organized output: series → season folders** | Jellyfin/Kodi/Emby convention is `Series Name (Year)/Season 01/Series Name S01E01 - Title.mkv`. v1.0 dumps flat `ShowName/*.mkv`. Any library manager user expects nesting. | Med | Reshape `outputBase` in episode.go to `Series Dir / Season XX / file`. Jellyfin explicitly says season folders must be `Season 01`, NOT `S01` (S01 fails scraping). |
| **Non-interactive mode preserved (`--quiet`/`--json`)** | Programmatic/CI users depend on these. TUI must be disableable; `--json` output contract (NDJSON events) must remain stable. | Low | TUI is a fourth `Mode` OR runs only when stdout is a TTY and no `--json`/`--quiet`/piped. Guard with `term.IsTerminal`. |
| **SIGINT/SIGTERM still cleans up** | v1.0 does graceful shutdown. TUI's Ctrl+C and compression's long FFmpeg run must not regress this — temp files released, partial output removed, stream tokens deleted. | Med | Bubble Tea intercepts Ctrl+C; must translate to ctx-cancel + `tea.Quit` and run the existing cleanup defer. Compression FFmpeg must use the same `context.Context`. |
| **Structured log levels configurable** | "Strategic logging for bug/error identification" implies at least Debug/Info/Warn/Error with a `--log-level` flag and a `--log-file` destination. Users debugging a failed run expect to crank verbosity. | Low-Med | `slog` gives this for free via `LevelVar` + `HandlerOptions.Level`. |
| **Progress visible during compression** | Re-encoding a 24-min episode takes minutes (vs seconds for copy). Silent multi-minute stalls look like a hang. | Med | Parse FFmpeg stderr progress (`out_time_ms`/`frame=`) and feed into the same progress surface (TUI spinner / humanOutput line / json event). |

---

## Differentiators

Features that set this tool apart or materially improve the v1.0 experience. Not expected, but valued.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Compression presets named by intent, not codecs** | Users don't know "CRF 23 libx265 medium." They know "save space" / "balanced" / "best quality." HandBrake's preset model (Fast 1080p, HQ, Space-Saver) is the gold standard for presenting compression to non-experts. | Med | 3-4 named presets mapping to codec+CRF+preset+tune. See **Compression Preset Presentation** below. Big UX win, low code cost. |
| **Anime-tuned x264/x265 encode** | Anime has flat color regions + line art; default encoder settings produce banding and ringing. `tune=animation` (x264/x265) + `-aq 2` materially improves output at a given bitrate vs generic settings. | Med | `libx264 -tune animation -crf N` and `libx265 -tune animation -crf N`. Only meaningful when re-encoding (copy ignores tune). Unique to this tool's domain. |
| **HEVC/H.265 option for ~40-50% size cut** | Anime compresses very well with HEVC. A "max compression" preset using `libx265 -tune animation -crf 24 -preset slow` can halve file size vs the AVC source at near-transparent quality. | Med-High | Slow (encode time ≫ download time), needs libx265 in the FFmpeg build, and 10-bit handling. Offer as opt-in preset, never default. |
| **Interactive Bubble Tea TUI for selection + progress** | Live progress bars, quality/track pickers, a season episode checklist with live ✓/✗ as each finishes — replaces squinting at scrolling `\r` lines. | High | Elm-architecture model: `Update` handles keypresses + progress Msgs from the download goroutine; `View` renders spinner/progress/list. Biggest engineering lift of the milestone. See **TUI Architecture** below. |
| **Folder metadata (tvshow.nfo + poster/fanart)** | Writes NFO + images so Jellyfin/Kodi/Emby show correct title/plot/cover **without relying on the media server's online scraper** matching "Season 01" — critical because Crunchyroll titles ≠ TVDB/anidb titles, so scraper mismatches are common. | Med | Fetch series metadata + images from Crunchyroll API, write `tvshow.nfo` (series root), per-episode `.nfo`, `poster.jpg`, `backdrop.jpg`, season posters. NFO XML schema (Kodi/Jellyfin compatible). |
| **Structured logs with component grouping (slog WithGroup)** | Diagnostic logs scoped per subsystem (`download`, `drm`, `media`, `mux`, `api`) so a log dump for a failed run reads `level=INFO msg=... drm.pssh=... drm.keys=2` vs an undifferentiated wall of text. | Low-Med | `slog.WithGroup("drm")` per package logger. Cheap, high debuggability ROI. |
| **Dual log output (human TUI + machine file)** | TUI on stdout for the user, structured JSON/Text slog to `--log-file` for post-mortem analysis. Best of both: pretty UX + greppable diagnostics. | Med | `slog.MultiHandler` or two handlers: TextHandler→stderr-ish (or file), JSONHandler→log file. Separate concern from the `Outputter` UI surface. |
| **Per-episode `.nfo` with Crunchyroll uniqueid** | Lets re-scrapes pin to the correct episode even when titles drift, via `<uniqueid type="crunchyroll">`. | Low | Small XML file alongside each `.mkv`. Pairs with folder metadata. |
| **Existing-file skip now keyed to organized path** | v1.0 skips re-download if the output file exists (episode.go L59-62). Moving to nested folders must keep this so re-running a series resumes. | Trivial | Comes free with the path change, but must be tested — it's the resumability contract. |

---

## Anti-Features

Features to explicitly NOT build this milestone.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|----------|-------------------|
| **Default-on re-encoding / compression** | Re-encoding is lossy, slow (minutes/episode), CPU-heavy, and changes the mux contract v1.0 users rely on. Surprising users with smaller-but-lossy files breaks trust. | Keep `-c copy` as the default "preset." Compression is opt-in via `--compress <preset>`; absent = copy. |
| **Two-pass bitrate-target encoding** | Two-pass needs a full first pass over the source = doubles FFmpeg time and complicates the streaming pipeline (we pipe decrypted segments into FFmpeg, not a complete file until mux). CRF (constant rate factor) is single-pass and the standard for size-with-quality-target. | Use CRF-based presets exclusively. Reserve 2-pass for a future "exact target size" feature. |
| **Re-encoding audio** | Audio is tiny relative to video; re-encoding it risks quality loss and sync issues for negligible size savings. The win is in the video stream. | Always `-c:a copy` even when video is re-encoded. Only transcode audio if a future format-conversion need appears. |
| **Downloading a lower quality to "compress"** | Tempting but wrong: bandwidth is the same, and Crunchyroll's lower tiers are bitrate-starved (worse than re-encoding 1080p with a good CRF). Compression here = post-process the chosen quality, never downshift the download. | Compression is a *post-download FFmpeg stage*, orthogonal to `--quality`. |
| **Full GUI / web interface** | PROJECT.md Out of Scope: "CLI-only by design." The TUI *is* the GUI — a terminal one. Don't scope-creep into a web/server UI. | Bubble Tea TUI only. |
| **Online scraper NFO enrichment (TVDB/anidb lookups)** | Adds network deps, API keys, rate limits, and mismatch complexity. Crunchyroll's own metadata is enough for a self-describing folder. | Write NFO from Crunchyroll API metadata only. Leave provider-id fields empty or with Crunchyroll's own id; users can re-scrape in their media server if desired. |
| **Batch transcode of pre-existing MKVs** | This is a download tool, not a general transcoder. Adding "transcode library mode" dilutes scope. | Compression only runs as part of a download. |
| **Replacing the `Outputter` interface with slog** | `Outputter` is the *user-facing UI* (human/json/quiet); slog is *machine-facing diagnostics*. Conflating them ruins both. | Keep `Outputter` for UX; add slog as a separate diagnostic plane. The TUI becomes a 4th `Outputter` implementation (or wraps the download phase). |
| **Auto-deleting original after compression** | Lossy steps must not destroy the source silently. | Compression writes the final `.mkv` directly (FFmpeg re-encodes into the output, temp decrypted files still cleaned up by existing logic). Never a "compress a copy then delete original" two-step that could leave nothing on failure. |

---

## Feature Dependencies

Dependencies on the existing v1.0 download pipeline (verified against code):

```
Compression preset
  → internal/mux/mux.go MergeEverything (swap -c:v copy → -c:v libx264/libx265 + flags)
  → internal/output Outputter.Progress (parse FFmpeg stderr for re-encode %)
  → FFmpeg build capability check (extend existing checkFFmpeg in main.go)
  → context.Context cancellation (SIGINT during long encode)

Bubble Tea TUI
  → internal/output Outputter interface (TUI = new Mode or wrapper)
  → internal/output/speed.go SpeedTracker (feed Bps/ETA into TUI model)
  → internal/download/episode.go progress calls (emit Msgs instead of fmt)
  → Structured logging (logs go to FILE, not stdout — stdout is TUI's)
  → term.IsTerminal guard (disable when piped/--json/--quiet)
  → SIGINT handling (translate tea.Quit → ctx cancel → existing cleanup defer)

Graceful missing-track handling
  → internal/download/episode.go L96-98 (audio) and L142-144 (subs): error → warn+skip
  → Manifest adaptation-set inspection (distinguish "no tracks at all" vs "requested subset missing")
  → Structured logging (Warn-level with track + episode attrs)

Organized output structure
  → internal/download/episode.go output paths L42-57 (Series/Season NN/filename)
  → existing sanitizeFilename (reuse; Jellyfin reserved chars already covered)
  → EpisodeInfo metadata (SeriesTitle, SeasonNumber, EpisodeNumber, Title — all present)
  → existing "skip if exists" check L59-62 (must remain valid under new paths)

Folder metadata (NFO + images)
  → Organized output structure (NFO lives in series/season folders)
  → api.EpisodeInfo + series metadata (need plot, art URLs — may require API extension)
  → Jellyfin/Kodi NFO XML schema + image filename conventions
  → HTTP image download (reuse api.Client transport)

Structured logging (slog)
  → internal/output Outputter (coexist; slog is separate diagnostic plane)
  → config precedence chain (add --log-level, --log-file; support config.json keys)
  → TUI integration (when TUI active, slog MUST go to file, stdout occupied)
  → drm/media/mux packages (inject grouped, component-scoped loggers)
```

Cross-feature ordering constraints (inform phase structure):
- **Organized output BEFORE folder metadata** — NFO/images target the folder layout; can't write `tvshow.nfo` into a series root that doesn't exist yet.
- **Graceful missing-track + structured logging TOGETHER** — the warn+skip path is exactly what produces useful Warn-level structured logs; do them in the same phase.
- **Structured logging BEFORE TUI** — the TUI forces logs off stdout; you need the file-logging path working before the TUI takes over stdout.
- **Compression is independent** — can be its own phase, but it depends on FFmpeg progress parsing which overlaps with the TUI progress work; sequence after or alongside TUI.

---

## Compression Preset Presentation

How to present compression quality/size tradeoffs to users (HandBrake-informed, opinionated):

**Principle:** Name presets by *intent/outcome*, not by codec/CRF numbers. Users choose a goal; the tool maps to encoder settings. Always show the tradeoff in one line.

| Preset name | Codec | CRF | x264/x265 preset | Tune | Audio | Typical vs source | When to use |
|-------------|-------|-----|------------------|------|-------|-------------------|-------------|
| `copy` *(default)* | — | — | — | — | copy | identical (0% change) | Archival, fastest, lossless. The v1.0 behavior. |
| `balanced` | libx264 | 20 | medium | animation | copy | ~30-40% smaller, visually transparent on anime | General use; plays everywhere; fast encode (~1x realtime). |
| `space` | libx265 | 24 | medium | animation | copy | ~50-60% smaller, near-transparent | Storage-constrained; modern device playback (HEVC support assumed). Slower (~0.3x realtime). |
| `best` | libx265 | 20 | slow | animation | copy | ~30-40% smaller, effectively transparent | Quality-first archival w/ some savings. Slowest (~0.1x realtime). |

CLI surface: `--compress copy|balanced|space|best` (default `copy`). Unknown value → error with the valid list. `--crf N` optional override for power users (advanced; document the range).

**Why these choices (research-grounded):**
- **CRF not bitrate/2-pass:** single-pass, no full-source pre-read (matches our streaming segment pipeline). CRF is the x264/x265-recommended mode for quality-target encoding.
- **`-tune animation`:** both x264 and x265 ship an `animation` tune that adjusts quantizer/AQ/deblock for flat regions + edges — measurable benefit on anime vs generic `film` tune.
- **CRF 18 = "visually lossless", 20 ≈ transparent, 23 ≈ good balance, 28 ≈ noticeable.** We pick 20/24 for quality-preserving presets; never >26 for anime (banding in skies/gradients).
- **`libx264` baseline compatibility, `libx265` for space:** HEVC's anime compression efficiency at CRF 24 routinely beats the source AVC at lower bitrate. Guard with encoder-availability probe.
- **10-bit:** anime benefits from 10-bit depth (avoids banding) but complicates playback compat. Keep 8-bit in `balanced` (broad playback), consider `-pix_fmt yuv420p10le` only in `best`/`space` with a documented caveat. *Phase research flag: verify HW decode support claims.*

**Anti-pattern to avoid:** do NOT expose raw `ffmpeg` flags via `--ffmpeg-args`. That makes presets unmaintainable, breaks the "intent" UX, and users WILL shoot themselves in the foot (e.g. `-crf 0` infinite size, or re-encoding audio). The named-preset approach with an optional `--crf` override is the right escape hatch.

---

## TUI Architecture (Bubble Tea)

Concrete shape for the differentiator feature:

- **Elm Architecture:** `model` struct holds download state (current episode, segments done/total, speed, per-episode status list for a season); `Init()` returns the first download `tea.Cmd`; `Update(msg)` handles `tea.KeyPressMsg` (Ctrl+C/q → quit + cleanup Cmd) and custom progress Msgs; `View()` renders progress bars + episode list.
- **Background work as Cmds:** the download/decrypt/mux pipeline runs in a goroutine launched via a `tea.Cmd`; it sends progress Msgs over a channel the program listens on (`tea.Cmd` returning `func() tea.Msg`). Keep the existing `errgroup` concurrency intact.
- **stdout is sacred:** while the TUI runs, ALL diagnostic logging MUST go to a `--log-file` (slog) — never `fmt.Println` to stdout. This is the hard constraint that forces **structured logging to land before/with the TUI**.
- **Coexist with --json:** `--json` and piped (non-TTY) stdout → keep current `jsonOutput`/`humanOutput`, do NOT start Bubble Tea. Gate: `if term.IsTerminal(os.Stdout) && !*json && !*quiet && *tui { startTUI() }`.
- **Components:** use Bubbles `spinner`, `progress`, and optionally `list` for episode pickers. Lip Gloss for layout. Never hand-roll the renderer.
- **Cancellation:** Ctrl+C → `tea.Quit`; the `Run()` return must still trigger the existing deferred cleanup (delete streams, remove temp files) because the download goroutine's context is canceled. Verify ctx propagation into `mux.MergeEverything` (already takes `ctx`) and `media.DownloadParts`.
- **Phase research flag:** v1→v2 Bubble Tea migration / API specifics (e.g. `tea.NewView` in v2 vs string return in v1) need a phase-level spike to pin the exact API surface against the version we vendor.

---

## MVP Recommendation

Prioritize (phase ordering, lowest-risk-highest-value first):

1. **Graceful missing-track handling + structured logging (slog)** — foundational; cheapest; unblocks TUI (needs file logging); immediately improves user experience for partial-library episodes. Do together.
2. **Organized output structure (series → season folders + Jellyfin naming)** — pure path logic in episode.go, no new deps, reuses sanitizeFilename + EpisodeInfo. Unblocks folder metadata. High user value for library-manager users.
3. **Folder metadata (NFO + images)** — builds directly on (2); needs API metadata + image fetch + NFO XML writer. Delivers the "self-describing anime library" differentiator.
4. **Compression presets (copy/balanced/space/best)** — independent of TUI; touches mux.go + FFmpeg; high "reduce storage" value (the milestone's headline goal). Comes after logging so encode progress is diagnostically visible.
5. **Bubble Tea TUI** — largest lift; depends on logging (file output) and ideally reuses compression/progress parsing. Do last so the surface it renders is already feature-complete.

Defer: **HEVC 10-bit modes**, **`--crf` advanced override**, **season artwork fetching** — mark as stretch/phase-research items, not MVP.

---

## Sources

- **Existing codebase** (HIGH): `internal/mux/mux.go`, `internal/download/episode.go`, `internal/output/output.go`, `internal/output/speed.go` — direct inspection of v1.0 pipeline anchor points.
- **PROJECT.md / ROADMAP.md** (HIGH): milestone scope, constraints (Go 1.25, FFmpeg dep, backward compat, cross-platform), v1.0 validated requirements list.
- **docs/ARCHITECTURE.md** (HIGH): component boundaries, data flow, the Phase A/B/C download pipeline that compression/TUI must integrate with.
- **Bubble Tea** (MEDIUM — context7/web): `github.com/charmbracelet/bubbletea` README + v2 upgrade guide — Elm Architecture, Model/Update/View, Cmds, `tea.LogToFile` (stdout-owned-by-TUI constraint), Bubbles/Lip Gloss ecosystem. v2.0.8 current.
- **Go log/slog** (MEDIUM — context7/pkg.go.dev): stdlib `log/slog` — Levels, `LevelVar`, Text/JSON/DiscardHandler, `Logger.With`/`WithGroup`, `MultiHandler`, `LogValuer`, perf guidance (defer evaluation).
- **Jellyfin TV Shows docs** (HIGH — web): `jellyfin.org/docs/general/server/media/shows` — folder structure (`Series Name (Year)/Season 01/S01E01...mkv`, `Season 00` for specials, NO `S01` abbreviation), NFO + image filename conventions (poster/folder/cover/backdrop/fanart/logo/banner/thumb), reserved chars.
- **FFmpeg encoding conventions** (MEDIUM — domain knowledge; ffmpeg wiki partially blocked by Anubis bot-wall): CRF mode, `-tune animation`, x264/x265 preset/CRF ranges, encoder-availability probing. *Phase research flag: re-verify exact CRF/preset values against current x264/x265 recommendations and the project's FFmpeg build.*
- **HandBrake preset model** (MEDIUM — domain knowledge): intent-named presets (Fast 1080p, HQ, Space-Saver) as the UX pattern for presenting compression to non-experts.