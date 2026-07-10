# Stack Research

**Domain:** Go CLI media downloader — v1.1 stack additions (compression, Bubble Tea TUI, structured logging, organized output, folder metadata)
**Researched:** 2026-07-10
**Confidence:** HIGH (Charm v2 versions verified directly against GitHub release pages 2026-07-10; slog/encoding/xml confirmed in Go 1.25 stdlib; FFmpeg compress flags aligned with FEATURES.md/PITFALLS.md which grounded anime-tune/CRF in x264/x265 docs)

## Scope

This file covers ONLY the NEW dependencies and version pins the v1.1 features introduce. The existing v1.0 stack (Go 1.25, golang.org/x/sync v0.22.0, golang.org/x/term v0.45.0, gowidevine v0.1.3, go-mpd pseudo-version, FFmpeg subprocess) is **already shipped and deliberately not re-evaluated** — those are the foundation the v1.1 additions sit on top of.

The v1.1 features add **4 new external Go modules** and rely on **2 stdlib packages already present in Go 1.25**. Everything else is FFmpeg flag knowledge and filesystem conventions — no libraries.

## Recommended Stack

### Core Technologies (NEW for v1.1)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `charm.land/bubbletea/v2` | v2.0.8 | Bubble Tea TUI runtime — Elm Architecture (Model/Update/View), `*tea.Program` event loop, goroutine-to-UI `p.Send` bridge | The de-facto Go TUI framework. v2 is the current major (v2.0.0 shipped 2026-02-24; v2.0.8 latest 2026-07-03). Uses the `charm.land` vanity import path, NOT `github.com/charmbracelet/bubbletea/v2`. v2 ships the new Cursed Renderer (ncurses-grade), declarative `View()` fields (alt-screen/mouse are struct fields, not imperative commands), built-in color downsampling, and fixes the v1 I/O-fight with Lip Gloss. Picking v2 over v1 is mandatory because v1 is the prior major and the ecosystem (bubbles v2, lipgloss v2) all moved to `charm.land` paths — mixing a v1 bubbletea with v2 bubbles/lipgloss will not compile. |
| `charm.land/bubbles/v2` | v2.1.1 | Reusable Bubble Tea components — `progress`, `spinner`, `viewport`, `list` sub-packages | The companion component library, released in lockstep with bubbletea v2 and lipgloss v2 (v2.0.0 2026-02-24; v2.1.1 latest 2026-07-04). We need exactly 4 sub-packages: `progress` (download/compress % bar), `spinner` (stage indicator), `viewport` (log-tail / season episode list scroll), `list` (interactive track/episode picker). v2 moved to getter/setter methods (`SetWidth`/`Width()` not exported fields) and functional-option constructors (`viewport.New(viewport.WithWidth(80))`) — copying v1 snippets will fail. |
| `charm.land/lipgloss/v2` | v2.0.5 | Declarative terminal styling/layout — borders, padding, color, table rendering for TUI panels | Required by the TUI for layout (stacking progress + episode list + log tail). When used WITH Bubble Tea v2, color downsampling is automatic and I/O is managed by the tea program — no manual `lipgloss.Println` needed inside a running program. v2 is pure/deterministic (no more fighting Bubble Tea over stdin/stdout, which was a real v1 lock-up bug). v2.0.5 latest 2026-07-03. `AdaptiveColor` is gone — use `lipgloss.LightDark(isDark)` or the `compat` sub-package for the background detection. |
| `gopkg.in/natefinch/lumberjack.v2` | v2.2.1 | Log-file rotation `io.WriteCloser` — size/age/backups-capped, gzip-compressed rotated files | The de-facto Go log rotator (5.5k stars). Stable/complete (latest 2023-02-06, 67 commits — "complete", not "dead"). Implements `io.Writer` so it drops straight into `slog.NewTextHandler(lj, &slog.HandlerOptions{})`. Required by the `--log-file` diagnostic-log feature: a downloader left running overnight on a full season would otherwise write an unbounded log file (PITFALLS.md Pitfall 5). Single-process assumption is fine — this is a CLI. Do NOT roll your own rotation; lumberjack handles atomic rename, timestamp suffixing, and old-file cleanup. |

### Standard Library (already in Go 1.25 — zero new deps)

| Package | In stdlib since | Purpose | Why no external lib |
|---------|-----------------|---------|---------------------|
| `log/slog` | Go 1.21 | Structured logging — `TextHandler`/`JSONHandler`, `LevelVar` (runtime-mutable level), `WithGroup`/`With` (component scoping), `HandlerOptions.ReplaceAttr` (redaction) | Go 1.25 ships the full slog API. No `zap`/`zerolog`/`logrus` needed — they would add a dependency for capability slog already has, and the milestone goal ("structured strategic logging for bug identification") is exactly slog's sweet spot. `LevelVar` gives the `--log-level` dynamic verbosity; `WithGroup("drm")` gives the per-component grouping in FEATURES.md; `ReplaceAttr` gives the token-redaction wrapper in PITFALLS.md Pitfall 5. Confirmed: it is in the `go 1.25.0` stdlib. |
| `encoding/xml` | Go 1.0 | NFO XML generation — `xml.MarshalIndent` / `xml.NewEncoder` with `MarshalXMLAttr` | Jellyfin/Kodi/Emby NFO is a simple, flat XML schema (`<tvshow>`, `<episodedetails>`, a handful of string/int child elements). `encoding/xml` handles entity escaping (`&`→`&amp;`, `<`→`&lt;`) for free — which is precisely the bug we avoid by NOT using `fmt.Sprintf` (PITFALLS.md Pitfall 6: a series title with `&` produces an un-scrapable NFO). No NFO-specific Go library exists that adds value over raw `encoding/xml` for emission; the schema is small enough that a `tvshow` and `episode` struct with `xml` tags is ~40 lines. |

### Existing v1.0 Dependencies (unchanged — listed for compatibility context)

| Technology | Version | Purpose | v1.1 status |
|------------|---------|---------|-------------|
| Go | 1.25.0 | Language runtime (go.mod `go 1.25.0`) | Unchanged. Brings slog in stdlib. |
| `golang.org/x/sync` | v0.22.0 | `errgroup` for parallel audio downloads | Unchanged. TUI must keep the errgroup pattern; progress emits via `p.Send` from inside `g.Go`. |
| `golang.org/x/term` | v0.45.0 | `term.IsTerminal` TTY detection | Unchanged. Already used in `output.Init` (output.go:85). The TUI gate (`--tui` only when `term.IsTerminal(stdout) && !--json && !--quiet`) reuses this exact call. |
| `github.com/iyear/gowidevine` | v0.1.3 | Widevine CDM | Unchanged. Pre-v1 API-stability risk is a v1.0 known constraint; no v1.1 touch. |
| `github.com/unki2aut/go-mpd` | pseudo-version | DASH manifest parsing | Unchanged. |
| FFmpeg (external binary) | runtime dep | Mux (`-c copy`) + new compress pass (`-c:v libx264/libx265`) | **Extended, not replaced.** v1.0 invokes it via `exec.CommandContext` (mux.go:91). v1.1 adds a *second* invocation for compression and a new encoder-availability probe. See FFmpeg section below. |
| `github.com/google/uuid` | v1.6.0 | UUID generation | Unchanged (indirect use). |

### Supporting Libraries

None beyond the 4 core modules above. Specifically:

- **No CLI framework** added — PROJECT.md Key Decision "Keep Go stdlib for CLI" stands; `--tui`/`--compress`/`--log-file`/`--log-level` are plain `flag.String`/`flag.Bool` additions, backward-compatible with the existing `flag.Parse` in main.go.
- **No separate progress library** — `bubbles/v2/progress` is the progress bar; the existing `output.SpeedTracker` (internal/output/speed.go) feeds Bps/ETA into it.
- **No NFO library** — `encoding/xml` + a small struct set is the right tool (see above).
- **No HTTP image-download library** — folder metadata image fetch reuses the existing `api.Client` transport (ARCHITECTURE.md integration point) via `http.Client.Get` on poster/backdrop URLs; a 4th tiny helper, not a dependency.

## FFmpeg Compression Stack (flags, not a Go library)

Compression is an **FFmpeg invocation**, not a Go encode library. ARCHITECTURE.md (Pattern 4) picks **Option B: post-mux separate invocation** — run the proven v1.0 `mux.MergeEverything` (`-c copy`) unchanged into a temp MKV, then call a new `internal/compress.Compress` that runs a *second* `exec.CommandContext("ffmpeg", ...)` to re-encode. This keeps `--compress` off-by-default = byte-identical v1.0 behavior (backward-compat constraint).

### Encoder flags by preset (aligned with FEATURES.md "Compression Preset Presentation" and PITFALLS.md Pitfall 1)

| Preset (`--compress <name>`) | Codec (`-c:v`) | CRF | `-preset` | `-tune` | `-pix_fmt` | Audio | Intent |
|------------------------------|---------------|-----|-----------|---------|------------|-------|--------|
| `copy` *(default, = v1.0)* | `copy` | — | — | — | — | `copy` | Lossless, fastest, archival |
| `balanced` | `libx264` | `20` | `medium` | `animation` | `yuv420p` | `copy` | ~30-40% smaller, transparent on anime, plays everywhere |
| `space` | `libx265` | `24` | `medium` | `animation` | `yuv420p` | `copy` | ~50-60% smaller, near-transparent, HEVC playback assumed |
| `best` | `libx265` | `20` | `slow` | `animation` | `yuv420p` | `copy` | Quality-first, effectively transparent, slowest |

### Why these flag choices (research-grounded; cross-check FEATURES.md + PITFALLS.md)

- **CRF not 2-pass / not bitrate** — CRF is single-pass, needs no full first-pass read of the source. Our pipeline pipes decrypted segments into the muxed file then re-encodes from that file; a 2-pass first pass would double FFmpeg time and fight the streaming model. CRF is the x264/x265-recommended quality-target mode. *(FEATURES.md Anti-Feature; PITFALLS.md confirms "CRF (constant rate factor) is single-pass".)*
- **`-tune animation`** — both `libx264` and `libx265` ship an `animation` tune that adjusts quantizer/AQ/deblocking for flat color regions + hard line-art edges. Materially better on anime than the default `film`/`none` tune. This is the domain-specific win that a generic "compress" can't replicate. *(FEATURES.md differentiator; PITFALLS.md Pitfall 1: "Prefer `libx265 -preset slow -crf 24-26` for storage savings on anime".)*
- **CRF ranges** — x264/x265: 18 = visually lossless, 20 ≈ transparent, 23 ≈ good balance, 28+ ≈ noticeable. We never exceed 26 on anime (banding in skies/gradients). `balanced`=20 (x264), `space`=24 / `best`=20 (x265). *(FEATURES.md presets table; aligns with PITFALLS.md "CRF 24-26" for the storage preset.)*
- **`libx264` for `balanced`, `libx265` for `space`/`best`** — HEVC is ~40% more efficient than AVC at equal quality on anime. `balanced` stays AVC for universal playback; `space`/`best` use HEVC and **must probe the FFmpeg build for libx265 support** before starting (some minimal builds omit it). *(FEATURES.md; PITFALLS.md Pitfall 1 "HEVC requires a player that supports it... Offer AVC fall back".)*
- **`-c:a copy` always** — audio re-encoding burns CPU, risks A/V drift, and saves negligible size vs the video stream. Double-compressing already-low-bitrate Crunchyroll AAC/EC-3 is strictly negative. *(FEATURES.md Anti-Feature; PITFALLS.md Performance Traps.)*
- **`-pix_fmt yuv420p`** — 8-bit 4:2:0 for the v1.1 MVP. 10-bit (`yuv420p10le`) avoids anime banding but complicates playback compatibility; **deferred** to stretch (FEATURES.md "Phase research flag: verify HW decode support claims"). Never ship 10-bit in a "balanced" preset.
- **Thread usage** — add `-threads 0` (auto) so a multi-core box isn't underutilized; libx265 slow on a single thread is the wall-clock bottleneck (PITFALLS.md Performance Traps).

### FFmpeg encoder-availability probe (new startup check)

v1.0 already runs `checkFFmpeg()` (main.go:196) — `exec.LookPath("ffmpeg")` + `ffmpeg -version`. v1.1 **extends** this: when `--compress space|best` is set, probe `ffmpeg -hide_banner -encoders` for `libx265`; when `balanced`, probe for `libx264`. Fail fast with an actionable error (*"preset 'space' requires libx265; your ffmpeg build lacks it — use 'balanced' or rebuild ffmpeg with --enable-libx265"*) **before** downloading GBs. This is table-stakes (FEATURES.md) and prevents a post-download surprise (PITFALLS.md Pitfall 1 warning sign: "Encode takes longer than the download").

### FFmpeg progress parsing for the TUI / human output

Re-encoding a 24-min episode takes minutes, not seconds. Silent multi-minute stalls read as a hang (PITFALLS.md UX Pitfalls). Parse FFmpeg stderr (`frame=`, `out_time_ms=`, `speed=`) or `-progress pipe:2` and feed into the same `output.Reporter`/`bridge.SegmentProgress` seam the download phase uses — the TUI's `progress.Model` then shows a compress-phase bar after the download bar. No new library; reuse the existing throttle (1/sec `lastProgressNanos` in `media/segment.go`).

## Installation

```bash
# NEW v1.1 dependencies (add to go.mod)
go get charm.land/bubbletea/v2@v2.0.8
go get charm.land/bubbles/v2@v2.1.1
go get charm.land/lipgloss/v2@v2.0.5
go get gopkg.in/natefinch/lumberjack.v2@v2.2.1

# NO logging library — log/slog is stdlib in Go 1.25
# NO NFO library — encoding/xml is stdlib
# NO CLI framework — stdlib flag (existing v1.0 decision stands)

# Tidy after adding
go mod tidy
```

### Resulting go.mod `require` block additions

```
require (
    charm.land/bubbletea/v2 v2.0.8
    charm.land/bubbles/v2 v2.1.1
    charm.land/lipgloss/v2 v2.0.5
    gopkg.in/natefinch/lumberjack.v2 v2.2.1
)
```

(Note: the `charm.land/*` modules pull in the `charmbracelet/x` and `charmbracelet/ultraviolet` transitive deps automatically — do not add those manually.)

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| `charm.land/bubbletea/v2` v2.0.8 | `charm.land/bubbles/v2` v2.1.x, `charm.land/lipgloss/v2` v2.0.x | The three Charm v2 majors were released **simultaneously** (2026-02-24) and are designed to operate in lockstep. Bubble Tea v2 manages I/O and hands colors to Lip Gloss v2 (which is now "pure"). Do **not** mix a v1 of any one with a v2 of another — import paths and APIs diverge (v1 = `github.com/charmbracelet/*`, v2 = `charm.land/*/v2`). |
| `charm.land/lipgloss/v2` v2.0.5 | Bubble Tea v2.0.x | Inside a running `tea.Program`, downsampling is automatic — no `lipgloss.Println` needed. The `compat` sub-package is for standalone-Lip-Gloss-without-Bubble-Tea only; do **not** import it from `internal/tui` (it does blocking I/O that fights the program). |
| `charm.land/bubbles/v2` v2.1.1 | Bubble Tea v2.0.x, Lip Gloss v2.0.x | `progress.WithColors()` now takes `color.Color` (from Lip Gloss v2), not a string — the v2 progress color API was overhauled. `viewport.New()` switched from `(width, height)` to functional options. |
| `gopkg.in/natefinch/lumberjack.v2` v2.2.1 | `log/slog` (stdlib) | `*lumberjack.Logger` satisfies `io.Writer`; pass directly to `slog.NewTextHandler(lj, opts)`. No glue code. Single-process only (CLI = fine). |
| `log/slog` (Go 1.25) | `lumberjack.v2`, `output.Global` | slog writes to the lumberjack writer (file). `output.Global` stays separate (user-facing stdout/JSON). The two never share a writer — see ARCHITECTURE.md Pattern 5 (dual stream). |
| `encoding/xml` (Go 1.25) | NFO schema | Defines `xml.Name`, `xml.MarshalIndent`, `xml.StartElement`/`xml.Encoder`. For `<uniqueid type="crunchyroll">` use `xml:"uniqueid,attr"` or a custom `MarshalXML` for the attribute pair. No external NFO lib needed. |
| `golang.org/x/term` v0.45.0 (existing) | Bubble Tea v2 | Bubble Tea does its own TTY setup, but the **gate** (`should I start the TUI at all?`) uses the existing `term.IsTerminal(os.Stdout.Fd())` call in `output.Init` — reusing it keeps the `--json`/`--quiet`/piped fallback identical to v1.0. |

## Alternatives Considered

| Category | Recommended | Alternative | Why Not (for this project) |
|----------|-------------|-------------|----------------------------|
| TUI framework | `charm.land/bubbletea/v2` | `tview` (rivo/tview) | tview is a widget-grid imperative framework, not Elm-Architecture. Better for static layouts; worse for a live-progress download dashboard driven by async worker goroutines. It also pulls in a heavier dep tree and doesn't compose with a slog/log-file plane as cleanly. Bubble Tea's `p.Send(msg)` from errgroup workers is the idiomatic fit for our existing `g.Go` pattern. |
| TUI framework | `charm.land/bubbletea/v2` | `tcell` + hand-rolled rendering | tcell is the low-level cell buffer Bubble Tea builds on; hand-rolling the renderer is months of work for no gain. Pitfall 2/7 (stdio collision, test false-greens) get *worse* without Bubble Tea's tested `Program`. |
| Components | `charm.land/bubbles/v2` | hand-rolled progress bar via `\r` overwrite | v1.0 already does `\r` overwrite (`humanOutput.Progress`, output.go:140). The TUI occupies stdout as a cell buffer, so `\r` overwrites corrupt it (PITFALLS.md Pitfall 2). Bubbles `progress.Model` is the only correct component inside a running `tea.Program`. |
| Styling | `charm.land/lipgloss/v2` | raw ANSI escapes (as v1.0 `output.ANSI*` consts do) | v1.0's hand-defined `ANSIRed`/`ANSIGreen` are fine for the non-TUI human sink. Inside the TUI, Lip Gloss handles width calculation, truncation, and color downsampling so ANSI doesn't leak into `--json`. Keep v1.0's ANSI consts for the non-TUI path; use Lip Gloss only in `internal/tui`. |
| Structured logging | stdlib `log/slog` | `go.uber.org/zap` | zap is faster in ultra-high-throughput service scenarios; this CLI logs *strategic* episode/season/FFmpeg events (PITFALLS.md Pitfall 5: never per-segment). At that volume slog's perf is indistinguishable, and adding zap means a second logging mental model. slog is in stdlib = zero deps, zero churn risk, and `LevelVar`/`WithGroup`/`ReplaceAttr` cover every requirement in FEATURES.md. |
| Structured logging | stdlib `log/slog` | `rs/zerolog` | Same argument; zerolog's `io.Writer`-only model is marginally leaner but duplicates slog's stdlib presence for no net gain on a CLI. |
| Log rotation | `natefinch/lumberjack.v2` | hand-rolled `os.Rename` + size check | Lumberjack is 67 commits of exactly this, including the timestamp-suffix atomic-rename and old-file age/backups cleanup. Re-implementing it is the textbook "not invented here" trap and misses gzip compression of rotated files. |
| NFO generation | stdlib `encoding/xml` | a Kodi/Jellyfin NFO Go library | None exists at quality. The NFO schema (`<tvshow>`, `<episodedetails>`, `<uniqueid>`) is ~8-15 fields. A `encoding/xml` struct set is 40 lines and gives free entity-escaping — the exact property that prevents the PITFALLS.md Pitfall 6 `&`-in-title XML-injection bug. |
| NFO generation | stdlib `encoding/xml` | `fmt.Sprintf` string templating | Explicitly rejected by PITFALLS.md Pitfall 6: `fmt.Sprintf` does NOT escape `&`/`<`/`"`; one `Hidamari Sketch × Honeycomb` title produces an NFO Jellyfin refuses to parse. `encoding/xml` is the only acceptable choice. |
| Compression | FFmpeg subprocess (`libx264`/`libx265`) | Go-native encoder (`x264`/`x265` Go bindings) | No production-grade pure-Go H.264/H.265 encoder exists. FFmpeg's libx264/libx265 are the reference implementations. We already depend on FFmpeg for mux; reusing it for compress adds zero new runtime deps and the same `exec.CommandContext` pattern (mux.go:91). |
| Compression | FFmpeg + `-tune animation` | hardware accel (NVENC/AMF/QSV) | Deferred — breaks the "Linux/macOS/Windows cross-compile, FFmpeg-consistency" constraint (NVENC is NVIDIA-only, QSV Intel-only). SOFTWARE fallback with libx265 `-preset slow` is portable; HW is a future opt-in. *(PITFALLS.md Performance Traps; ARCHITECTURE.md Scaling Priorities.)* |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `github.com/charmbracelet/bubbletea` (v1 import path) | v1 is the **prior major**. The v2 ecosystem (bubbles v2, lipgloss v2) moved to `charm.land/*/v2`. Mixing v1 bubbletea with v2 bubbles/lipgloss will not compile. Also v1's `View() string` and imperative `tea.WithAltScreen()` commands are replaced in v2 by `View() tea.View` and declarative fields — copying v1 tutorials produces compile errors or interface-assertion panics. | `charm.land/bubbletea/v2@v2.0.8` |
| `github.com/charmbracelet/bubbles` (v1) / `github.com/charmbracelet/lipgloss` (v1) | Same — prior majors, different import paths, incompatible APIs (v1 bubbles uses exported `Width`/`Height` fields; v2 uses getter/setter). Library StackOverflow answers predating Feb 2026 are v1. | `charm.land/bubbles/v2@v2.1.1`, `charm.land/lipgloss/v2@v2.0.5` |
| `charm.land/lipgloss/v2/compat` (inside `internal/tui`) | `compat.HasDarkBackground()` does **blocking I/O** outside the Bubble Tea event loop — exactly the v1 lock-up bug v2 was built to fix. The `compat` package is for standalone Lip Gloss use (no Bubble Tea). Inside a running `tea.Program` it fights the program for the TTY. | Inside TUI: `tea.RequestBackgroundColor` cmd + handle `tea.BackgroundColorMsg` in `Update`, then build styles with `lipgloss.LightDark(isDark)`. |
| Bubble Tea v1 `tea.WithAltScreen()` / `tea.EnableMouseCellMotion()` options | Removed in v2. Calling them is a compile error. v2 view features are **declarative struct fields** on `tea.View` (`AltScreen`, `MouseMode`, `ReportFocus`, `WindowTitle`). | `func (m model) View() tea.View { v := tea.NewView(...); v.AltScreen = true; return v }` |
| Bubble Tea v1 `func (m model) View() string` signature | v2 requires `View() tea.View`. A string-returning `View` fails the `tea.Model` interface. (HIGHEST-frequency copy-paste footgun per PITFALLS.md Pitfall 2.) | `tea.NewView(s)` wrapping your rendered string; set feature fields on it. |
| `fmt.Sprintf` for NFO XML | Does not escape `&`, `<`, `>`, `"`, `'`. A series title like `Hidamari Sketch × Honeycomb` or `Re:Zero` yields an NFO Jellyfin/Kodi silently reject, falling back to filename heuristics — undoing the whole metadata feature. (PITFALLS.md Pitfall 6.) | `encoding/xml` `xml.MarshalIndent` / `xml.Encoder`. Test with `&`, `<`, CJK, emoji titles. |
| `zap` / `zerolog` / `logrus` | Adds a dependency for capability `log/slog` (stdlib since Go 1.21, present in our Go 1.25) already provides. `LevelVar` = dynamic `--log-level`; `WithGroup("drm")` = component grouping; `HandlerOptions.ReplaceAttr` = token redaction. No second logging mental model. | stdlib `log/slog` with `slog.NewTextHandler` (default, greppable, ~2x faster than JSON) and an optional `slog.NewJSONHandler` for a future `--log-format=json`. |
| Per-segment `slog.Info` logging to file | Wall-clock regression + multi-GB log files on a full season. The segment loop calls `output.Global.Progress` hundreds of times per episode; routing that to slog marshals+writes a JSON record each time. (PITFALLS.md Pitfall 5.) | Keep per-segment progress in the in-process `output.Reporter`/TUI bar. slog emits **strategic** events only (episode start/finish, FFmpeg invocation summary, token refresh, season failure) at Info; Debug is per-segment but gated behind `--debug`. |
| Unbounded single log file (no rotation) | A downloader left running overnight on a multi-series batch writes an unbounded `debug.log`; on a small disk that fills it. (PITFALLS.md Pitfall 5.) | `lumberjack.v2` `Logger{MaxSize: 5-10, MaxBackups: 3, Compress: true}` at `~/.config/animeheaven/logs/animeheaven.log`. |
| `tea.LogToFile` directly (without redaction) | Charm's convenience `tea.LogToFile("debug.log", "debug")` writes raw — it will capture bearer tokens, `CRUNCHYROLL_ETP_RT`, `client_id`, `private_key` if the existing `Debug("\n%s", body)` calls (api/episode.go, api/manifest.go) route to it. (PITFALLS.md Pitfall 5, Security Mistakes.) | Route slog through a `ReplaceAttr`-redacting handler wrapping a `lumberjack` writer; keep raw body dumps behind a separate `--debug-raw` flag. |
| Go-native H.264/H.265 encoder libs (e.g. `x264`/`x265` cgo bindings, `gen2brain/avif`, `codec2`) | No production-grade pure-Go H.264/H.265 encoder exists; cgo bindings break the clean cross-compile (Linux/macOS/Windows) story of a stdlib-FFmpeg CLI. We already require FFmpeg for mux — reuse that one dependency. | FFmpeg subprocess via `exec.CommandContext` (the existing mux.go:21 `ffmpegCommand` var for testability). |
| Hardware-accelerated encode (NVENC/AMF/QSV) as the default | NVIDIA-only / Intel-only / platform-dependent — breaks the cross-platform binary guarantee and FFmpeg-build-consistency. Different presets produce different-looking output on different machines (non-reproducible bug reports). | Software `libx264`/`libx265` for all v1.1 presets. Research HW-accel as a separate future opt-in flag, never the default. |
| A separate "transcoder library" (e.g. gst, handbrake-cli) | Adds a 2nd external binary dependency to a tool that currently requires only FFmpeg. `--compress` is a download-adjacent step, not a general library transcoder (FEATURES.md Anti-Feature). | FFmpeg only — it already does the mux, doing the compress in the same dependency is the minimal surface. |
| `go-flags` / `pflag` / `cobra` for the new `--tui`/`--compress`/`--log-*` flags | PROJECT.md Key Decision "Keep Go stdlib for CLI // Good". The 4 new flags are plain `flag.String`/`flag.Bool`, resolvable through the existing `resolveString` precedence chain (main.go:226). A framework migration risks the backward-compat constraint. | stdlib `flag` (unchanged). |
| `testify` for the new `internal/{tui,compress,nfo,dirorganizer,logging}` tests | PROJECT.md Key Decision D-03: "Table-driven stdlib tests only // Good". testify is an explicit rejection. | stdlib `testing` table-driven tests; `teatest` (charm.land) for TUI integration since it's in-ecosystem. |

## Stack Patterns by Variant

**If `--tui` and stdout is a TTY (interactive run):**
- Boot `internal/tui`: `tea.NewProgram(model, ...)` owns stdout.
- `output.Global` is swapped to a `bridge{p *tea.Program}` implementing `output.Reporter` — each `Info/Warn/Progress` call becomes a `p.Send(msg)`, never a raw `fmt.Fprintf`.
- `slog` default handler writes to a `lumberjack` file (NOT stdout — stdout is the TUI's), at Info level. Use `slog.NewTextHandler(lj, &slog.HandlerOptions{Level: lvl, ReplaceAttr: redact})`.
- Lip Gloss v2 styles render the model's `View()`; downsampling is automatic because Bubble Tea v2 manages I/O. Do NOT import `compat`.
- Gate: `term.IsTerminal(os.Stdout.Fd()) && *tui && !*json && !*quiet`. Otherwise fall through to the v1.0 sinks byte-identically.

**If `--json` or stdout is piped / `--quiet` (CI, scripted, non-interactive):**
- Do **NOT** start Bubble Tea. `output.Global` stays `jsonOutput`/`quietOutput`/`humanOutput` (v1.0 unchanged).
- `slog` can write to **stderr** (TextHandler) AND a `lumberjack` file — two handlers via a `slog.MultiHandler` or a fan-out `io.MultiWriter`. stdout stays clean NDJSON.
- No Lip Gloss, no bubbles — the dependency is compiled in but unused in this path (small binary cost only).

**If `--compress` is set (any output mode):**
- `mux.MergeEverything` runs first, unchanged (`-c copy`, writes temp MKV) — the v1.0 path verbatim.
- `internal/compress.Compress` runs second: `exec.CommandContext(ctx, "ffmpeg", "-i", tempMKV, "-c:v", codec, "-crf", crf, "-preset", preset, "-tune", "animation", "-pix_fmt", "yuv420p", "-c:a", "copy", "-c:s", "copy", <re-applied metadata>, finalMKV)`.
- Write to a **distinct temp path**, `os.Rename` to final only on success; on failure the v1.0 remuxed MKV survives (PITFALLS.md Recovery Strategies). Clean the temp remux only after the rename.
- FFmpeg progress (`-progress pipe:2` or stderr `frame=`) feeds the same `output.Reporter`/bridge seam, so the TUI/JSON/human sinks all show a compress-phase bar without compress-specific plumbing.

**If missing audio/subtitle track (graceful handling, any output mode):**
- Convert the two hard `return error` sites (episode.go:97, episode.go:143) into warn+skip for **non-primary** tracks; `all audio missing` stays a hard error.
- Each skip emits a `slog.WarnContext(ctx, "track missing", "locale", lc, "kind", "audio", "episode", n)` — component-scoped via `WithGroup`.
- TUI shows a per-episode missing-track badge in the list panel; `--json` emits a `{"type":"warn","missing":"audio","locale":"pt-BR"}` event; season summary lists the gaps.

**If writing folder metadata (NFO + images):**
- Depends on the organized-output structure (Series/Season NN/) landing first — NFO targets that layout.
- `tvshow.nfo` at `<out>/<Series>/`; per-episode `<filename>.nfo` next to each `.mkv`; `poster.jpg`/`backdrop.jpg` at series root.
- Fix the **latent `season_number=EpisodeNumber` bug** (mux.go:87) to `SeasonNumber` BEFORE mirroring it into NFO — otherwise folder metadata surfaces a v1.0 mux bug as a visible season mismatch in Plex/Jellyfin (PITFALLS.md Pitfall 6).
- `encoding/xml` marshals structs; images fetched via the existing `api.Client` transport, **non-fatal** on 404 (never fail a 30-min download over a poster).

## Research Flags for Phase Planning

These are areas where research gives the *direction* but not the *exact value* — they need a hands-on spike in their phase, per PITFALLS.md Phase-Specific Warnings:

| Phase Topic | What needs spike-time validation | Why research can't settle it |
|-------------|----------------------------------|------------------------------|
| **Compression** | Exact CRF/preset matrix on 3 real Crunchyroll episodes (1080p action, 1080p slice-of-life, 720p legacy) measuring source-size vs transcoded-size vs wall-clock vs A/B dark-scene quality. | PITFALLS.md Pitfall 1 requires a **measured size-reduction gate** (≥20% smaller than remux on ≥1 episode) before a preset ships. Research says "CRF 24, libx265, slow, animation" is the right *starting point*, but double-compression of already-Crunched source can flip the result — only a bench on real input settles it. |
| **TUI** | Pin the exact Bubble Tea v2 API surface (e.g. `tea.View` field names, `progress.WithColors` signature, `teatest` v2 usage) against a minimal spike before broad implementation. | v2 is new (Feb 2026); few non-Charm code examples exist yet. ARCHITECTURE.md already flags "v1→v2 migration / API specifics need a phase-level spike." |
| **Folders/NFO** | Validate that one real Jellyfin AND one real Kodi scan correctly reads the generated `tvshow.nfo` + `Season 01/` layout + `<uniqueid type="crunchyroll">`. | NFO scanner precedence varies by media server; the schema is community-documented, not a published standard. |
| **Logging redaction** | Run a live `--debug` auth flow and `grep` the log for token-like substrings. | Unit tests can't prove redaction is complete (they test the rules you thought of); an obtuse manual grep is the PITFALLS.md-prescribed verification. |

## Sources

- **charmbracelet/bubbletea GitHub releases** (https://github.com/charmbracelet/bubbletea/releases) — verified v2.0.8 is latest (2026-07-03); v2.0.0 shipped 2026-02-24; import path `charm.land/bubbletea/v2`; `View() tea.View` and declarative view struct fields confirmed from the v2.0.0 release notes. — **HIGH** (official release page, read directly 2026-07-10)
- **charmbracelet/bubbles GitHub releases** (https://github.com/charmbracelet/bubbles/releases) — verified v2.1.1 is latest (2026-07-04); v2.0.0 2026-02-24; import `charm.land/bubbles/v2` with sub-packages; getter/setter + functional options + `DefaultKeyMap()` function change confirmed from v2.0.0 notes. — **HIGH**
- **charmbracelet/lipgloss GitHub releases** (https://github.com/charmbracelet/lipgloss/releases) — verified v2.0.5 is latest (2026-07-03); import `charm.land/lipgloss/v2`; deterministic/pure styles, `LightDark()`, `compat` for non-Bubble-Tea only, `tea.RequestBackgroundColor`+`tea.BackgroundColorMsg` pairing confirmed from v2.0.0 notes. — **HIGH**
- **natefinch/lumberjack GitHub** (https://github.com/natefinch/lumberjack) — v2.2.1 latest (2023-02-06); `gopkg.in/natefinch/lumberjack.v2` import; `io.WriteCloser` confirmed from README; 5.5k stars, 67 commits (stable/complete). — **HIGH**
- **Go stdlib `log/slog`** — present in `go 1.25.0` (go.mod line 3); `TextHandler`/`JSONHandler`/`LevelVar`/`WithGroup`/`ReplaceAttr` per pkg.go.dev/log/slog. — **HIGH** (stdlib, unambiguous)
- **Go stdlib `encoding/xml`** — present since Go 1.0; `xml.MarshalIndent`, `xml.Encoder` handle entity escaping for NFO. — **HIGH**
- **Existing repo `go.mod`, `internal/output/output.go`, `internal/mux/mux.go`, `internal/download/episode.go`, `main.go`** — integration seams (singleton, `exec.CommandContext` FFmpeg, `-c copy` mux, `output.Global` call sites, `checkFFmpeg`, `resolveString` precedence, `term.IsTerminal` gate) verified by direct source read. — **HIGH**
- **Cross-referenced research files** (`.planning/research/FEATURES.md`, `ARCHITECTURE.md`, `PITFALLS.md`) — alignment on preset table (copy/balanced/space/best), post-mux Option B compression, dual-stream slog/output, `encoding/xml` NFO, the `season_number` latent bug, Bubble Tea v2 stdio-ownership pitfall. These are the authoritative siblings STACK.md must not contradict. — **HIGH**

---
*Stack research for: Go CLI anime downloader v1.1 (compression + Bubble Tea TUI + structured logging + organized output + folder metadata)*
*Researched: 2026-07-10*