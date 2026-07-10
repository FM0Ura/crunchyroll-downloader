# Project Research Summary

**Project:** AnimeHeaven (Crunchyroll Downloader) — v1.1 Storage, CLI & Error Handling
**Domain:** Go CLI media downloader (anime) — extending a shipped v1.0 (5,525 LOC) with compression, Bubble Tea TUI, structured logging, organized output, folder metadata, graceful error handling
**Researched:** 2026-07-10
**Confidence:** HIGH

## Executive Summary

This is a **Go CLI download tool** being incrementally extended, not greenfield. The existing v1.0 pipeline (download → Widevine decrypt → FFmpeg `-c copy` mux → temp cleanup) is shipped, tested, and treated as a dependency. The v1.1 milestone adds six features on top: storage compression, interactive Bubble Tea TUI, graceful missing-track handling, Jellyfin-organized output folders, NFO/image folder metadata, and structured slog diagnostic logging. Experts build this kind of tool by keeping the proven streamcopy-mux path byte-identical and layering each new capability as an **additive, toggleable, isolated package** so a v1.1 regression can never corrupt the v1.0 contract.

The recommended approach, consistent across all four research files, is: (1) introduce a small `output.Reporter` interface seam that decouples progress producers from renderers; (2) run compression as a **post-mux separate FFmpeg invocation** (Option B) so `--compress` off = exact v1.0 behavior; (3) use stdlib `log/slog` on stderr + a `lumberjack`-rotated file as a **separate diagnostic plane** from the user-facing `output/` singleton; (4) fix the existing `season_number=EpisodeNumber` latent mux bug *before* mirroring metadata into NFO; and (5) build the Bubble Tea TUI **last**, only after the `Reporter` seam and file logging exist, so it is a pure renderer swap rather than a re-architecture.

Key risks cluster around **three integration collisions** the research flags as highest blast radius: (a) Bubble Tea v2 owns stdio while `output.Global` still writes `fmt.Fprintf` from 8 packages — unresolved this corrupts the TUI and leaks progress; (b) re-encoding already-Crunchyroll-compressed streams can produce *larger* files and visible banding, so compression needs a measured ≥20% size-reduction gate on real episodes, not faith in CRF defaults; and (c) slog defaults (JSON, Info, per-segment) create both a wall-clock regression and a bearer-token PII leak if a redaction wrapper isn't shipped with logging from day one. All three are preventable with the concrete mitigations in PITFALLS.md and are explicitly sequenced into the phase plan below.

## Key Findings

### Recommended Stack

v1.1 adds **4 new external Go modules** and relies on **2 stdlib packages already in Go 1.25**. No CLI framework, no zap/zerolog, no NFO library, no HW encoder — each was explicitly considered and rejected. The stack research was written to align with FEATURES/ARCHITECTURE/PITFALLS (preset table, post-mux Option B, dual-stream logging, `encoding/xml` NFO, latent `season_number` bug all cross-checked).

**New external modules:**
- `charm.land/bubbletea/v2` v2.0.8 — Bubble Tea TUI runtime (Elm Architecture); **v2 not v1** (`charm.land` vanity import, `View() tea.View` not string). v2.0.0 shipped 2026-02-24 in lockstep with bubbles/lipgloss v2.
- `charm.land/bubbles/v2` v2.1.1 — `progress`, `spinner`, `viewport`, `list` sub-packages. v2 uses getter/setter + functional-option constructors; v1 snippets won't compile.
- `charm.land/lipgloss/v2` v2.0.5 — declarative styling/layout; v2 is "pure" (no I/O fight with Bubble Tea). `AdaptiveColor` gone; use `LightDark()`. Do **not** import `compat` from inside a running program.
- `gopkg.in/natefinch/lumberjack.v2` v2.2.1 — log rotation `io.WriteCloser`; drops into `slog.NewTextHandler(lj, opts)`. Stable/complete.

**Stdlib (zero new deps):**
- `log/slog` (Go 1.21+, in our Go 1.25) — `TextHandler`/`JSONHandler`, `LevelVar` (=`--log-level`), `WithGroup` (per-component grouping), `HandlerOptions.ReplaceAttr` (token redaction). Replaces any need for zap/zerolog.
- `encoding/xml` (Go 1.0) — `xml.MarshalIndent` gives free `&`/`<` entity escaping for NFO; ~40-line struct set. The exact property that prevents the `Hidamari Sketch × Honeycomb` XML-injection bug.

**FFmpeg (existing binary, extended role):** v1.1 adds a **second** `exec.CommandContext` invocation (`internal/compress.Compress`) after the unchanged `mux.MergeEverything` `-c copy` pass. Presets: `copy` (default = v1.0), `balanced` (libx264 crf20 medium -tune animation), `space` (libx265 crf24 medium), `best` (libx265 crf20 slow). All presets use `-c:a copy`, `-pix_fmt yuv420p` (8-bit), `-threads 0`. New startup probe: `ffmpeg -hide_banner -encoders` for libx265/libx264 availability **before** download, fail fast.

### Expected Features

**Table stakes (missing = regresses v1.0 expectations):**
- Compression `copy` default preserved (opt-in re-encode; `--compress` off = byte-identical v1.0)
- All v1.0 flags backward-compatible (`--quality`, `--audio`, `--subs`, `--output-dir`, `--quiet`, `--json`, `--workers` untouched)
- FFmpeg encoder-availability probe extended from existing `checkFFmpeg()`
- Graceful missing-track handling: warn+skip non-primary tracks; "all audio missing" stays a hard error
- Organized output: `Series Title (Year)/Season 01/...mkv` (Jellyfin requires `Season 01`, NOT `S01`)
- Non-interactive mode preserved (`--json`/`--quiet`/piped = no TUI)
- SIGINT/SIGTERM cleanup contract preserved through TUI + compress long FFmpeg runs
- `--log-level` + `--log-file` configurable
- Compress progress visible (parse FFmpeg `frame=`/`-progress pipe:2`)

**Differentiators (set this tool apart):**
- Compression presets **named by intent** (`copy`/`balanced`/`space`/`best`), HandBrake-style — not raw codec/CRF
- Anime-tuned encode (`-tune animation`, x264/x265 `animation` quantizer/AQ/deblock for flat regions + line-art)
- HEVC option for ~40-50% size cut (opt-in, needs libx265 probe)
- Interactive Bubble Tea TUI (live progress, episode checklist, track picker) — largest engineering lift
- Folder metadata (`tvshow.nfo` + per-episode `.nfo` + `poster.jpg`/`backdrop.jpg`) keyed to Crunchyroll `uniqueid` so scraper mismatches (CR titles ≠ TVDB/anidb) don't matter
- slog `WithGroup` per-subsystem diagnostic grouping (`download`/`drm`/`media`/`mux`/`api`)
- Dual output: human TUI on stdout + machine slog to rotated file
- Existing-file skip re-keyed to nested organized paths (preserves resumability)

**Anti-features (explicitly NOT building):**
- Default-on re-encoding; 2-pass bitrate-target; audio re-encoding; "download lower quality to compress"; full GUI/web UI; online TVDB/anidb NFO enrichment; batch transcode of pre-existing MKVs; replacing `Outputter` with slog; auto-delete original after compress; raw `--ffmpeg-args` escape hatch; HW accel (NVENC/QSV) as default; `testify`; go-flags/pflag/cobra.

**Defer to v2+:** 10-bit (`yuv420p10le`) presets, `--crf` advanced override, season artwork fetching, HW-accel encoding as opt-in, `--log-format=json` default switch.

### Architecture Approach

The existing system is a linear per-episode pipeline (`download.Episode` → `media.DownloadParts`/`DownloadSubs` → `mux.MergeEverything` `-c copy` → cleanup) with two singletons (`config`, `output.Global` called from 8 packages). The v1.1 architecture inserts **new packages as peers, not replacements**: `internal/logging` (slog, separate from `output/`), `internal/compress` (post-mux re-encode, separate from `mux/`), `internal/tui` (Bubble Tea, a renderer behind the new `Reporter` seam), `internal/nfo` + `internal/dirorganizer` (pure filesystem ops). Five concrete patterns govern the integration: (1) a `Reporter` interface in `output/` (`Info/Warn/Error/Debug/Progress/SegmentProgress`) that all producers call instead of `output.Global` directly, letting the TUI/human/json/quiet sinks swap behind it; (2) goroutine→TUI bridge via `*tea.Program.Send(msg)` with the existing 1/sec `lastProgressNanos` throttle reused; (3) Bubble Tea v2 declarative `View() tea.View` with feature fields (alt-screen/mouse as struct fields, not commands); (4) **Option B post-mux compression** — remux fast to temp MKV unchanged, then second FFmpeg re-encodes to a *distinct* temp, `os.Rename` on success so a compress failure leaves the good remux intact; (5) **dual-stream** slog-on-stderr/rotated-file + output-on-stdout, never sharing a writer.

**Major components (new + modified):**
1. `internal/logging` (NEW) — `logging.Init(mode, level)` → `slog.SetDefault` with `LevelVar`, `ReplaceAttr` redaction, `lumberjack` writer; default TextHandler on stderr.
2. `internal/output` (modified) — keeps `Outputter`; adds `Reporter` interface + `SegmentProgress`; `output.Global` becomes a `Reporter`. TUI injects a `bridge{*tea.Program}` impl.
3. `internal/compress` (NEW) — `Compress(ctx, in, out, cfg, info)`; separate FFmpeg invocation; re-applies metadata; Option B.
4. `internal/tui` (NEW) — `tea.Model` (episodes, stage, progress, spinner, logViewport); `bridge` implements `output.Reporter`; Ctrl+C → `tea.Quit` → ctx cancel → existing cleanup defer.
5. `internal/nfo` (NEW) — `encoding/xml` marshals `tvshow`/`episodedetails` structs; `<uniqueid type="crunchyroll">` attribute pair; free entity escaping.
6. `internal/dirorganizer` (NEW) — `Series (Year)/Season NN/<file>` layout; reuses `sanitizeFilename`; non-fatal image fetch via existing `api.Client` transport.
7. `internal/download` (modified) — calls `compress`/`nfo`/`dirorganizer` after `mux`; converts the two hard `return error` sites (episode.go:97, :143) to warn+skip for non-primary tracks.
8. `internal/mux` (modified) — accepts optional `ProgressReporter`; **fix `season_number=EpisodeNumber` → `SeasonNumber` latent bug at mux.go:87 BEFORE NFO**.

### Critical Pitfalls

The top pitfalls, all flagged **[INTEGRATION]** (highest blast radius = where v1.1 collides with v1.0 invariants):

1. **Re-encoding already-Crunched streams can produce *larger* files + banding** (Pitfall 1) — Crunchyroll source is already quantized; naive CRF 23 re-encode adds bits preserving source artifacts. **Prevention:** opt-in `--compress` only; run a codec-matrix bench on 3 real episodes before shipping any preset; define a **≥20% size-reduction gate** on ≥1 episode or the preset is cut; prefer `libx265 -preset slow -crf 24-26` for storage; `-c:a copy` always. Verify with A/B dark-scene comparison, not just file size.
2. **Bubble Tea v2 steals stdio while `output.Global` still `fmt.Fprintf`s from 8 packages** (Pitfall 2 — highest blast radius) — corrupted TUI frames, broken progress, alt-screen vanishing, ANSI leaking into `--json`. v2-specific footgun: `View() string` (v1) is now `View() tea.View` (v2); copy-paste from v1 tutorials fails. **Prevention:** the `output.Global` → `tea.Msg` bridge refactor is the **first TUI task, not polish**; gate TUI on `term.IsTerminal && !--json && !--quiet`; route diagnostics to slog/file, never `fmt.Println` inside the program.
3. **`Update` loop blocks on download I/O → frozen UI** (Pitfall 3) — wiring `download.Episode` inline in `Update` runs it on the render goroutine. **Prevention:** all long work is a `tea.Cmd` (`func() tea.Msg`); emit `progressMsg` via `p.Send` from inside the existing `errgroup` at the `segment.go:198` call site; throttle to ~10-15fps via existing `SpeedTracker`; ctx derived from model-quit cancels `exec.CommandContext` + errgroup + HTTP client (same chain as v1.0).
4. **Missing/mismatched tracks → broken or empty MKVs** (Pitfall 4) — v1.0 mux uses positional `-map 1+i`/`1+len(audio)+j`; "soft skip a missing locale" shrinks the slice and silently mislabels dubs (Japanese track stamped Portuguese). **Prevention:** muxer `os.Stat`s each input, rejects empty before FFmpeg (`ErrEmptyTrack{Locale,Kind}`); build input+map+metadata per-track in one pass; surface missing best-effort tracks in season summary; `ffprobe`-stream-count assertion in tests. **Must land before compression** — a broken partial-mux that gets re-encoded bakes the breakage into a transcoded file.
5. **slog default JSON + per-segment logging = perf tax + PII leak** (Pitfall 5) — Info-level per-segment JSON marshal on a 13-ep season = tens of thousands of records; auth path dumps bearer tokens to disk in cleartext. **Prevention:** default Info with **strategic** events only (episode start/finish, FFmpeg summary, token refresh, season failure); per-segment stays in the in-process `Reporter`/TUI path; `lumberjack` rotation at 5-10MB, 3 backups, `~/.config/animeheaven/logs/`; `ReplaceAttr` redaction wrapper for `Authorization`/`Cookie`/`etp_rt`/`client_id`/`private_key`; raw body-dumps behind a separate `--debug-raw`. **Ship redaction WITH logging, not after** — PII leak is a ship-blocker.
6. **Folder metadata writes wrong scope / fights cleanup / surfaces latent `season_number` bug** (Pitfall 6) — `tvshow.nfo` per-season re-tags show with first season's metadata; cleanup glob deletes `poster.jpg`; `fmt.Sprintf` NFO fails on `&`/`<`/CJK; `mux.go:87 season_number=EpisodeNumber` is a v1.0 latent bug that NFO surfaces as "Season 1 = season 5 episodes" in Plex. **Prevention:** `tvshow.nfo` only at series root; **fix the one-line `season_number` bug before mirroring into NFO**; `encoding/xml` marshal only; cleanup **exact-path, never glob**; artwork out-of-band, non-fatal.
7. **TUI tests false-green** (Pitfall 7) — `output_test.go` stays green testing a dead code path while production uses the bridge; `teatest` exercises only isolated `Update`/`View`, missing the `p.Send`→`Model` integration. **Prevention:** at least one `teatest` integration test (fake segment producer → `progressMsg` through real `Program` → assert View shows counts + `tea.Quit` cancels ctx within bounded timeout); CI runs both headless (`CI=true`) and pty legs; rewrite ANSI-literal assertions now renderer-controlled.

## Implications for Roadmap

Based on the cross-file dependency analysis (FEATURES.md MVP ordering, ARCHITECTURE.md build order, PITFALLS.md phase-to-pitfall mapping), the suggested phase structure is **6 phases**. The ordering rationale is fixed by three hard constraints: (a) **track-hardening before compression** — a broken partial-mux that gets re-encoded bakes the bug in; (b) **logging before TUI** — the TUI forces slog off stdout, so the file-logging path must exist; (c) **`output.Reporter` seam + `season_number` fix before folder metadata** — NFO targets the organized layout and correct metadata.

### Phase 1: Track Hardening + Structured Logging (slog)
**Rationale:** Foundational; cheapest; unblocks TUI (needs file logging) and unblocks the redaction wrapper (ship-blocker). The warn+skip path produces exactly the Warn-level structured logs that slog is for — do them together. No behavioral risk to the proven mux path (still `-c copy`), only the two error sites in episode.go become warn+skip for non-primary tracks.
**Delivers:** `internal/logging` package (slog + lumberjack rotation + `ReplaceAttr` redaction); graceful missing-track handling; `--log-level`/`--log-file` flags; per-subsystem `WithGroup` loggers (`download`/`drm`/`media`/`mux`/`api`); grep-verified no PII in `--debug` log.
**Addresses FEATURES:** graceful missing-track handling, structured log levels configurable, structured logs with component grouping, dual log output.
**Avoids PITFALLS:** 4 (missing/mismatched tracks), 5 (slog perf tax + PII).
**Uses STACK:** `gopkg.in/natefinch/lumberjack.v2` v2.2.1, stdlib `log/slog`.

### Phase 2: Organized Output + Folder Metadata (NFO/images)
**Rationale:** Pure filesystem logic, no new deps, reuses `sanitizeFilename` + `EpisodeInfo`. Must land **before** compression (which would bake a wrong `season_number` into a transcoded file) and depends on the Phase-1 logger for "artwork fetch failed (non-fatal)" warnings. Fixes the latent `season_number=EpisodeNumber` bug at mux.go:87 as a one-line correctness fix first.
**Delivers:** `internal/dirorganizer` (Series (Year)/Season NN/ layout), `internal/nfo` (tvshow.nfo + per-episode .nfo + uniqueid, `encoding/xml`), artwork fetch via `api.Client` transport (non-fatal on 404), `season_number` fix, cleanup switched to exact-path-only with a "poster.jpg survives cleanup" regression test.
**Addresses FEATURES:** organized output structure, folder metadata, per-episode .nfo with Crunchyroll uniqueid, existing-file skip re-keyed to organized path.
**Avoids PITFALLS:** 6 (folder metadata scope, cleanup conflict, latent `season_number` bug, XML entity escaping).
**Uses STACK:** stdlib `encoding/xml`.

### Phase 3: Compression Presets (post-mux, default OFF)
**Rationale:** Independent of TUI; high "reduce storage" headline value. After Phase 1 (logging visible) and Phase 2 (correct metadata). **Build is gated on a 1-2 day spike first** measuring real Crunchyroll episodes (1080p action, 1080p slice-of-life, 720p legacy) against the ≥20% size-reduction gate; cut any preset that fails. The spike de-risks the CRF/preset choices before implementation cost.
**Delivers:** `internal/compress.Compress` (Option B — post-mux separate invocation, distinct temp, `os.Rename` on success, remux survives on failure); `--compress copy|balanced|space|best` flag; encoder-availability probe (`ffmpeg -encoders`) fail-fast before download; FFmpeg `-progress pipe:2` stderr parsing into the `Reporter` seam; `-threads 0`, `-c:a copy`, `-pix_fmt yuv420p` (8-bit) defaults.
**Addresses FEATURES:** compression presets named by intent, anime-tuned encode, HEVC option, FFmpeg availability check extended, progress visible during compression.
**Avoids PITFALLS:** 1 (double-compression of already-Crunched streams) via measurement gate + opt-in default + `-c:a copy` + `-tune animation`; recovery strategy (distinct temp + atomic rename) so failure leaves the good remux.

### Phase 4: Output Reporter Seam (refactor, zero behavioral change)
**Rationale:** Mechanical refactor that un-blocks the TUI; introduce the `Reporter` interface in `output/`, route existing `output.Global.X` calls through it, keep all v1.0 tests green. Isolating this into its own phase keeps the TUI phase from being a re-architecture. *(Can be merged into Phase 1 if scoped tightly, but research recommends it as its own step because it touches 8 packages and benefits from focused review.)*
**Delivers:** `internal/output/reporter.go` (`Reporter` iface incl. `SegmentProgress`); `output.Global` typed as `Reporter`; all 8 call sites compile-clean through the interface; TUI injection point ready (`bridge` impl is Phase 5).
**Addresses FEATURES:** TUI coexistence with `--json`/`--quiet` (seam enables it).
**Avoids PITFALLS:** precondition for 2, 3, 7 — the bridge cannot exist without this seam.
**Uses STACK:** none new.

### Phase 5: Bubble Tea TUI (v2)
**Rationale:** Largest lift; depends on Phase 4 (Reporter seam) and Phase 1 (file logging, so stdout is free for the TUI). The `output.Global` → `tea.Msg` bridge is the **first task, not last**. By this phase, progress already flows through `Reporter` from the download/mux/compress producers, so the TUI is a pure renderer swap.
**Delivers:** `internal/tui` package — `tea.Model` (episodes, stage, progress.Model, spinner.Model, viewport.Model); `bridge{*tea.Program}` implementing `Reporter` via `p.Send`; `--tui` flag gated on `term.IsTerminal(stdout) && !--json && !--quiet`; Ctrl+C → `tea.Quit` → ctx cancel → existing cleanup defer (no second signal handler); `teatest` integration test (fake producer → `progressMsg` → assert View shows counts → `tea.Quit` cancels ctx bounded); CI runs both headless (`CI=true`) and pty legs.
**Addresses FEATURES:** interactive Bubble Tea TUI, non-interactive mode preserved, SIGINT cleanup preserved through TUI.
**Avoids PITFALLS:** 2 (stdio collision — first task), 3 (Update loop blocks — Cmds only), 7 (false-green tests — teatest integration + pty CI).
**Uses STACK:** `charm.land/bubbletea/v2` v2.0.8, `charm.land/bubbles/v2` v2.1.1, `charm.land/lipgloss/v2` v2.0.5.

### Phase 6 (stretch): 10-bit encoding + HW accel research + season artwork
**Rationale:** Marked as stretch/research, not MVP. Defer per FEATURES.md "Defer" list.
**Delivers:** 10-bit (`yuv420p10le`) preset variants with documented HW-decode caveat, `--crf` advanced override, season artwork, optional NVENC/QSV opt-in (research-only — breaks cross-platform consistency so never default).
**Addresses FEATURES:** stretch differentiators.
**Avoids PITFALLS:** HW-accel cross-platform variance (Performance Traps).

### Phase Ordering Rationale

- **Track-hardening (Phase 1) before compression (Phase 3):** a broken partial-mux that gets re-encoded bakes the breakage into a transcoded file that's expensive to undo (PITFALLS.md Pitfall 4 phase-mapping).
- **Logging (Phase 1) before TUI (Phase 5):** the TUI owns stdout; slog must already have the file/rotation/redaction path so diagnostics don't vanish or leak (PITFALLS.md Pitfall 2 + 5).
- **Organized output (Phase 2) together with folder metadata (Phase 2):** NFO/images target the nested folder layout; can't write `tvshow.nfo` into a series root that doesn't exist yet (FEATURES dependency constraints).
- **`season_number` fix (Phase 2) before NFO (Phase 2):** the latent v1.0 bug is one-line; fix file metadata correctness before mirroring it into folder metadata (PITFALLS.md Pitfall 6).
- **Reporter seam (Phase 4) before TUI (Phase 5):** TUI must be a renderer swap, not a re-architecture — the bridge needs the interface to exist.
- **Compression (Phase 3) is parallelizable with Phase 4:** compression doesn't need the Reporter; FFmpeg progress can route through it once it exists. If the roadmapper wants to compress phases, Phase 3 and 4 have no interdependency.
- **TUI last (Phase 5):** biggest new surface, highest blast radius (Pitfalls 2/3/7); doing it last means the surface it renders is already feature-complete and progress already flows through the seam.

### Research Flags

Phases likely needing deeper `/gsd-plan-phase --research-phase <N>` during planning:
- **Phase 3 (Compression):** HIGH — needs a hands-on CRF/preset matrix spike on 3 real Crunchyroll episodes (1080p action / 1080p SoL / 720p legacy) measuring source-size vs transcoded-size vs wall-clock vs A/B dark-scene quality. Research gives the *starting point* but double-compression of already-quantized source can flip results — only a bench settles it. The ≥20% size-reduction gate is defined pre-implementation so a failed preset can be cut.
- **Phase 5 (TUI):** MEDIUM-HIGH — Bubble Tea v2 shipped Feb 2026; few non-Charm code examples exist yet. Pin the exact v2 API surface (`tea.View` field names, `progress.WithColors(color.Color)` signature, `teatest` v2 usage, `tea.RequestBackgroundColor`+`tea.BackgroundColorMsg` pairing) with a minimal spike before broad implementation.
- **Phase 2 (Folders/NFO):** MEDIUM — validate one real Jellyfin AND one real Kodi scan correctly reads the generated `tvshow.nfo` + `Season 01/` + `<uniqueid type="crunchyroll">`. NFO scanner precedence varies by server; schema is community-documented, not a published standard.
- **Phase 1 (Logging redaction):** MEDIUM — run a live `--debug` auth flow and grep the log for token-like substrings. Unit tests can't prove redaction complete; an obtuse manual grep is the prescribed verification.

Phases with standard patterns (skip research-phase):
- **Phase 4 (Reporter seam):** mechanical interface refactor; existing tests guard behavior; no novel API to discover.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Charm v2 versions verified against GitHub release pages (2026-07-10); slog/encoding/xml confirmed in Go 1.25 stdlib; FFmpeg flags aligned cross-file with FEATURES/PITFALLS (x264/x265 docs). Lumberjack confirmed stable/complete. |
| Features | HIGH | Grounded in existing codebase anchor points (mux.go:50, episode.go:42-57/96-98/142-144, output.go singleton) + Jellyfin folder-structure docs + HandBrake preset UX model + Bubble Tea/slog/FFmpeg conventions. |
| Architecture | MEDIUM-HIGH | Bubble Tea v2 / Bubbles v2 / slog verified against current docs; integration seams (output.go, mux.go, segment.go, main.go) read directly from source. Anime-specific CRF tuning flagged MEDIUM — needs phase-specific validation. |
| Pitfalls | HIGH | Codebase-grounded (8 call sites of `output.Global`, positional `-map` indices, `season_number=EpisodeNumber` latent bug, `warnRemove` cleanup, auth path token-dump sites all verified in source). Bubble Tea v2 facts verified against v2.0.8 README + UPGRADE_GUIDE_V2. |

**Overall confidence:** HIGH

### Gaps to Address

- **Compression CRF/preset values are not research-settled.** Research says "libx265 -preset slow -crf 24-26 + `-tune animation`" is the right *starting point*, but double-compression of already-Crunchyroll-quantized source can flip the size result. **Handle:** Phase 3 starts with a 1-2 day measured spike and a ≥20% size-reduction gate before any preset ships. Cut losers before implementation.
- **Bubble Tea v2 ecosystem is young (Feb 2026).** Few non-Charm examples exist; StackOverflow answers are v1. **Handle:** Phase 5 begins with a minimal API-surface spike (`tea.View` fields, `progress.WithColors(color.Color)`, `teatest` v2) before broad implementation.
- **NFO scanner precedence varies by server (Jellyfin/Kodi/Emby/Plex).** Schema is community-documented, not a published standard. **Handle:** Phase 2 validates against one real Jellyfin AND one real Kodi scan.
- **Log redaction completeness is not unit-testable.** Tests prove the rules you thought of, not the ones you missed. **Handle:** Phase 1 ships with a grep-literal-token CI rule + a manual obtuse grep of a `--debug` log from a live auth flow.
- **10-bit / HW-accel deferred.** Not blocking; flagged for Phase 6 (stretch). 10-bit avoids anime banding but complicates playback compat; HW-accel breaks cross-platform/FFmpeg-consistency. Neither is MVP.

## Sources

### Primary (HIGH confidence)
- **Existing codebase** (`internal/output/output.go`, `internal/mux/mux.go`, `internal/download/episode.go`/`season.go`, `internal/media/segment.go`, `internal/api/{client,episode,manifest}.go`, `main.go`, `go.mod`) — integration seams verified by direct source read; the 8 `output.Global` call sites, positional `-map` indices, `season_number=EpisodeNumber` latent bug, `warnRemove` cleanup, `checkFFmpeg`, `resolveString` precedence, `term.IsTerminal` gate all confirmed.
- **PROJECT.md / ROADMAP.md** — milestone scope, constraints (Go 1.25, FFmpeg dep, backward-compat, cross-platform, "Keep Go stdlib for CLI", D-03 "table-driven stdlib tests only"), v1.0 validated requirements list.
- **charmbracelet/bubbletea GitHub releases** (https://github.com/charmbracelet/bubbletea/releases) — v2.0.8 latest 2026-07-03; v2.0.0 shipped 2026-02-24; `charm.land/bubbletea/v2` vanity import; `View() tea.View` and declarative view struct fields confirmed from v2.0.0 release notes.
- **charmbracelet/bubbles GitHub releases** (https://github.com/charmbracelet/bubbles/releases) — v2.1.1 latest 2026-07-04; v2 getter/setter + functional options + `DefaultKeyMap()` changes.
- **charmbracelet/lipgloss GitHub releases** (https://github.com/charmbracelet/lipgloss/releases) — v2.0.5 latest 2026-07-03; deterministic/pure styles, `LightDark()`, `compat` for non-Bubble-Tea only, `tea.RequestBackgroundColor`+`tea.BackgroundColorMsg`.
- **Bubble Tea v2 UPGRADE_GUIDE_V2.md** — `tea.NewView(s)`, removed `tea.WithAltScreen()`/`tea.EnableMouseCellMotion()`, declarative view struct fields.
- **natefinch/lumberjack GitHub** (https://github.com/natefinch/lumberjack) — v2.2.1 (2023-02-06); `io.WriteCloser`; 5.5k stars; stable/complete.
- **Go stdlib `log/slog`** (pkg.go.dev/log/slog, Go 1.25) — `TextHandler`/`JSONHandler`/`LevelVar`/`WithGroup`/`ReplaceAttr`.
- **Go stdlib `encoding/xml`** (pkg.go.dev/encoding/xml) — `MarshalIndent`/`Encoder` entity escaping.
- **Jellyfin TV Shows docs** (jellyfin.org/docs/general/server/media/shows) — `Series Name (Year)/Season 01/S01E01...mkv` (NO `S01`), NFO + image filename conventions (poster/folder/cover/backdrop/fanart/logo/banner/thumb), reserved chars.

### Secondary (MEDIUM confidence)
- **FFmpeg encoding wiki + docs** (ffmpeg.org/ffmpeg.html) — streamcopy vs transcoding distinction, stream specifiers, `-c copy` vs `-c:v libx264 -crf`; x264/x265 CRF semantics. *Caveat: anime-specific CRF/preset tuning needs phase-specific validation against real Crunchyroll episodes (Phase 3 spike).*
- **HandBrake preset model** — intent-named presets (Fast 1080p, HQ, Space-Saver) as the UX pattern for presenting compression to non-experts (domain knowledge).
- **Bubble Tea / Bubbles v2 README + tutorials + pkg.go.dev** — Elm Architecture, `tea.KeyPressMsg`, ProgramOptions (official, current Jul 2026).
- **Kodi/Jellyfin/Plex NFO scanner conventions** — `tvshow.nfo` placement, `seasonNN.tbn`, `<uniqueid>` (community wiki; cross-check against one scanner's docs before Phase 2 implementation).

### Tertiary (LOW — needs validation)
- **Exact CRF/preset/codec matrix on real Crunchyroll episodes** — research gives the starting point; only a measured bench settles the ≥20% size-reduction gate (Phase 3 spike).
- **Bubble Tea v2 broad API surface** — v2 is new (Feb 2026); few non-Charm code examples; pin with a minimal spike (Phase 5).
- **NFO scanner precedence across Jellyfin AND Kodi AND Emby AND Plex** — community-documented, not a published standard; validate with one real Jellyfin AND one real Kodi scan (Phase 2).

---
*Research completed: 2026-07-10*
*Ready for roadmap: yes*