# Architecture Research

**Domain:** Go CLI media downloader — compression pass, Bubble Tea TUI, structured logging integration into existing internal/ package structure
**Researched:** 2026-07-10
**Confidence:** MEDIUM (Bubble Tea v2 / Bubbles v2 / slog verified against current docs; FFmpeg compression trade-offs from official ffmpeg docs + domain knowledge, anime-specific CRF tuning needs phase-specific validation)

## Standard Architecture

### Existing System Overview (v1.0 baseline)

The codebase is a Go CLI with `internal/` packages and two singletons (`config` pattern + `output.Global`). The data flow is linear and synchronous per-episode:

```
main.go (flag.Parse → output.Init → config.Load → resolve precedence)
   │
   ▼
processURL → download.Episode / download.Season
   │
   ▼ (per episode)
download.Episode ──┬── media.DownloadParts (video)   ──► Disk-backed temp .mp4
                  ├── media.DownloadParts (audio×N)  ──► Disk-backed temp .mp4/mp3   [errgroup parallel for N>1]
                  ├── media.DownloadSubs (subs)       ──► Disk-backed temp .ass
                  │
                  ▼  (all decrypted via widevine.DecryptMP4Auto)
              mux.MergeEverything ──► single .mkv
                  │   ffmpeg -c:v copy -c:a copy -c:s copy  (STREAMCOPY — fast, lossless)
                  │   + metadata:g / metadata:s:a:N / metadata:s:N tags
                  │   + disposition flags
                  ▼
              os.Remove(tempFiles)
                  │
                  ▼
              output.Global.Info("Download finished! Output file: %s")
```

Throughout, `output.Global` (a singleton `Outputter` interface with Info/Warn/Error/Debug/Progress) is called directly from every package. `output.RecordBytes` / `output.SpeedBps` / `output.ETASeconds` provide the rolling speed tracker.

### v1.1 Target System Overview (with new features overlaid)

```
main.go (flag.Parse → output.Init → config.Load → resolve precedence → logger.Init)
   │                       ┌──────────────────────────────────┐
   │                       │  internal/logging (NEW)           │
   │                       │  slog.TextHandler → stderr       │
   │                       │  slog.JSONHandler (debug/--json)  │
   │                       │  LevelVar (dynamic --verbose)     │
   │                       └───────────────┬──────────────────┘
   ▼                                       │ logger attrs: run_id, episode_id
processURL                                                  │
   │                                                        │
   ▼                                                        ▼
download.Episode/Season ──┬── media.DownloadParts ──► ProgressReporter (NEW iface)
                         │                         replaces direct output.Global.Progress
                         │                              │
                         ▼                              │ progress events
                     compress.Compress (NEW) ◄──────────┘
                         │   optional post-mux re-encode pass
                         ▼
                     mux.MergeEverything  (modified: emits progress to reporter)
                         │
                         ▼
                     nfo.Write (NEW) + dirorganizer.Organize (NEW)
                         │   ──► Series/Season N/episodes + .nfo + poster
                         ▼
                     output.Global / tui.Model (NEW, coexisting via reporter impl)

  ┌─────────────────────────────────────────────────────────────┐
  │  internal/tui (NEW) — Bubble Tea v2 program                  │
  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────┐     │
  │  │ tea.Model│  │ progress │  │ spinner  │  │ viewport    │     │
  │  │ (state)  │  │ (bubbles)│  │ (bubbles)│  │ (log tail)  │     │
  │  └────┬─────┘  └────┬─────┘  └────┬─────┘  └──────┬──────┘     │
  │       │             │             │               │            │
  │       └── p.Send(progressMsg) ◄──── workers (goroutines) ──────┘ │
  └─────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Existing / New | Implementation Notes |
|-----------|-----------------|---------------|------------------------|
| `internal/config` | Pointer-field Config singleton, precedence resolution | Existing (modified) | Add `Compression *CompressionCfg`, `LogLevel *string`, `OutputStructure *string` pointer fields |
| `internal/output` | `Outputter` interface singleton + speed tracker | Existing (modified) | Keep; add a `Reporter` adapter so TUI and legacy modes share one progress contract |
| `internal/logging` | Structured slog setup, dynamic level, stderr sink | **NEW** | Thin wrapper: `logging.Init(mode, level)` configures `slog.SetDefault` |
| `internal/mux` | FFmpeg mux invocation, metadata tags, cleanup | Existing (modified) | Accept a `ProgressReporter`; stream progress|stderr parsing optional |
| `internal/compress` | Post-mux FFmpeg re-encode pass (CRF) | **NEW** | `compress.Compress(ctx, inMKV, outMKV, cfg) error` |
| `internal/tui` | Bubble Tea v2 program, model, view, message routing | **NEW** | Hosts `*tea.Program`, exposes `Send`-based progress bridge |
| `internal/download` | Episode/Season orchestration | Existing (modified) | Calls compress + nfo + organize after mux |
| `internal/nfo` | NFO metadata XML writer (anime folder) | **NEW** | Emits `tvshow.nfo` / `episode.nfo` |
| `internal/dirorganizer` | Output directory structure (Series/Season N/) | **NEW** | Replaces flat `Series/` + filename pattern in `download.Episode` |
| `internal/media` | Segment download, decryption, speed progress | Existing (minor) | Progress already funnels through `output.Global` — unchanged |
| `main.go` | Flag parsing, bootstrap, mode selection | Existing (modified) | Boots TUI vs legacy based on `--tui` flag + TTY detection |

## Recommended Project Structure

```
internal/
├── api/            # existing — Crunchyroll API client
├── config/         # existing (modified) — add Compression/LogLevel/OutputStructure pointer fields
├── download/       # existing (modified) — calls compress+nfo+organize after mux
├── drm/            # existing — Widevine CDM
├── locale/         # existing — language mappings
├── media/          # existing — segment download/decrypt/mux tracks
├── mux/            # existing (modified) — accept ProgressReporter; optional -progress parsing
├── output/         # existing (modified) — Outputter + new Reporter interface
├── logging/        # NEW — slog setup, dynamic level, stderr sink
├── compress/       # NEW — FFmpeg re-encode pass (CRF), codec presets
├── tui/            # NEW — Bubble Tea v2 model, view, progress bridge
├── nfo/            # NEW — NFO XML metadata writer
└── dirorganizer/   # NEW — Series/Season N/ output structure + poster
```

### Structure Rationale

- **`logging/` separate from `output/`:** Logging (structured machine-readable events for debugging) is a *different concern* from output (human/JSON progress display). Mixing them recreates the current god-singleton problem. `logging` owns slog handlers, `output` owns the user-facing renderer. They share nothing except both writing to stderr (logging) vs stdout/human-styled (output).
- **`compress/` separate from `mux/`:** Muxing (streamcopy + metadata append) and compression (transcoding with CRF) are distinct FFmpeg operations with different failure modes, timescales, and tunables. Keeping them separate lets users disable compression (the v1.0 default path) without touching mux code, and lets compression be tested/researched independently.
- **`tui/` as a peer of `output/`, not a replacement:** The TUI is *one rendering strategy* for the same progress events. The `Reporter` interface in `output/` lets Bubble Tea and the legacy human/json/quiet sinks consume the same progress stream. This preserves `--json` and `--quiet` as first-class modes (PROJECT.md constraint: backward compatibility).
- **`nfo/` + `dirorganizer/` isolated:** Metadata injection and folder layout are pure filesystem operations with no download/decrypt dependency. Isolating them makes them unit-testable with table-driven stdlib tests (matching the existing D-03 "no testify" decision).

## Architectural Patterns

### Pattern 1: Progress Reporter Interface (decouple producers from renderers)

**What:** Define a small interface in `internal/output` that all progress producers satisfy. Each output mode (human, JSON, quiet, TUI) implements it. Producers (`media.DownloadParts`, `mux.MergeEverything`, `compress.Compress`) call the reporter, never `output.Global` directly.

**When to use:** Whenever the same operation must render to multiple, swappable sinks (terminal text, NDJSON, TUI, quiet). This is exactly the v1.1 situation.

**Trade-offs:** One extra interface + indirection vs. the current direct `output.Global.X()` calls. The cost is small (the interface is ~5 methods mirroring the existing `Outputter`) and the payoff is that the TUI becomes a drop-in renderer instead of a fork of the download logic.

**Example:**
```go
// internal/output/reporter.go (NEW)
package output

// Reporter is the progress contract every producer satisfies.
// output.Global already almost implements it; tuiReporter and the existing
// humanOutput/jsonOutput/quietOutput will too.
type Reporter interface {
    Info(format string, args ...any)
    Warn(format string, args ...any)
    Error(format string, args ...any)
    Debug(format string, args ...any)
    Progress(format string, args ...any)
    // Structured progress for TUI/JSON consumers (adds to Progress).
    SegmentProgress(label string, done, total int, bps float64, etaSecs int)
}

// Global stays as the default Reporter; TUI swaps it on boot.
var Global Reporter = &humanOutput{}
```

### Pattern 2: Goroutine-to-TUI bridge via `p.Send(msg)`

**What:** Bubble Tea's Elm Architecture is single-threaded by design — `Update` runs on the program's event loop. Long-running downloads happen in goroutines. The integration bridge is `*tea.Program.Send(tea.Msg)`: worker goroutines emit progress messages, the program pends them, and `Update` mutates the model state. The `View()` then renders declaratively.

**When to use:** Any time existing concurrent download work (errgroup workers, segment download pool) must surface progress into a TUI. This is the only correct Bubble Tea v2 pattern for async→UI.

**Trade-offs:** Messages are queued and processed serially — high-frequency progress (per-segment) must be throttled or coalesced to avoid flooding the queue. The existing 1/sec throttle in `media/segment.go` (`lastProgressNanos`) already does this; reuse it. Avoid copying large payloads into messages (send counts/percentages, not byte buffers).

**Example:**
```go
// internal/tui/messages.go (NEW)
package tui

import tea "charm.land/bubbletea/v2"

type progressMsg struct {
    Episode   int
    Total     int
    Done      int
    SegTotal  int
    Bps       float64
    ETASecs   int
    Stage     string // "downloading" | "muxing" | "compressing"
}

// bridge implements output.Reporter, forwarding to the program.
type bridge struct{ p *tea.Program }

func (b *bridge) SegmentProgress(label string, done, total int, bps float64, eta int) {
    b.p.Send(progressMsg{Done: done, SegTotal: total, Bps: bps, ETASecs: eta, Stage: label})
}
// ...Info/Warn/Error/Progress/Debug forward similarly
```

### Pattern 3: Bubble Tea v2 Declarative View State (Elm Architecture)

**What:** The `tea.Model` is a plain struct holding *all* UI state. `Update(msg)` returns a *new* model (or mutated) plus optional `Cmd`. `View()` returns a `tea.View` struct (v2 change from string) and declares terminal features (alt-screen, mouse, cursor) as fields rather than side-effecting commands.

**When to use:** This is the Bubble Tea model — there is no alternative pattern within the framework. The architectural decision is *what lives in the model*: keep it a pure view of the download state, never let it drive downloads. Downloads stay in `download.Episode`; the model mirrors received messages.

**Trade-offs:** Model grows with every UI concern (current episode, season list, log tail). Split into sub-models (one per panel) with composite Update/View to avoid a monolith. This matches how Bubbles components (progress, spinner, viewport) are designed to embed.

**Example:**
```go
// internal/tui/model.go (NEW)
type model struct {
    episodes      []episodeRow
    currentIdx     int
    stage          string
    progress       progress.Model   // bubbles component
    spinner        spinner.Model     // bubbles component
    logViewport    viewport.Model   // bubbles component
    logLines       []string
    width, height  int
    quitting       bool
}

func (m model) View() tea.View {
    v := tea.NewView(renderBoard(m))
    v.AltScreen = true         // v2: declarative, not tea.WithAltScreen()
    v.MouseMode = tea.MouseModeNone
    return v
}
```

### Pattern 4: Two-Option Compression Pipeline (integrated vs post-mux)

**What:** Compression is breaking the current streamcopy mux into a transcode. Two viable placements:

- **Option A — Integrated mux+compress:** Modify `mux.MergeEverything` to accept a `*CompressionCfg`. When non-nil, replace `-c:v copy` with `-c:v libx264 -crf <N> -preset <P>` (and audio `-c:a` if re-encoding audio too). Single FFmpeg invocation, no temp file, but the mux path becomes slow and lossy.
- **Option B — Post-mux re-encode pass (RECOMMENDED):** Keep `mux.MergeEverything` exactly as-is (proven, fast streamcopy to a temp/compressed MKV), then call `compress.Compress` as a *separate* FFmpeg invocation that reads the MKV and writes the final file with re-encoded video, copied audio/subs, and re-applied metadata. The temp is deleted on success.

**When to use Option B:** Now. It is incremental, keeps the v1.0 mux regression-free, isolates compression failure from mux failure, and is trivially toggleable (`--compress` off by default keeps exact v1.0 behavior — satisfying the backward-compat constraint).

**Trade-offs:** Option B does one extra I/O pass over the muxed file and re-applies metadata. The cost is ~1× file read + 1× write plus the encode, which is dominated by the encode anyway. Option A saves that pass but couples two failure surfaces and makes the "compress off" path a code branch inside mux rather than a skipped call.

**Example (Option B):**
```go
// internal/compress/compress.go (NEW)
func Compress(ctx context.Context, in, out string, cfg CompressionCfg, info *api.EpisodeInfo) error {
    args := []string{"-i", in,
        "-c:v", cfg.Codec,          // libx264 | libx265
        "-crf", strconv.Itoa(cfg.CRF),
        "-preset", cfg.Preset,       // slow|medium|fast
        "-pix_fmt", "yuv420p",
        "-c:a", "copy", "-c:s", "copy",
    }
    // re-apply the same metadata:g / disposition flags mux already set
    args = appendMetadata(args, info)
    args = append(args, out)
    cmd := exec.CommandContext(ctx, "ffmpeg", args...)
    var stderr bytes.Buffer
    cmd.Stderr = &stderr
    if err := cmd.Run(); err != nil {
        os.Remove(out)
        return fmt.Errorf("compress ffmpeg failed: %w: %s", err, stderr.String())
    }
    return nil
}
```

### Pattern 5: slog on stderr, output on stdout (dual stream)

**What:** Use Go stdlib `log/slog` for structured *diagnostic* logging and keep `output/` for *user-facing* progress. Logging writes to **stderr** always; the human/JSON output writes to **stdout** (and the TUI occupies stdout when active). This separation means `--json` NDJSON on stdout stays parseable while logs go to a sibling stream/file, and piping `2>logs.txt` captures diagnostics without polluting machine-readable output.

**When to use:** Any CLI that has both a machine-readable output mode and a need for debug logs. Critical here because `--json` already uses stdout and the TUI will take it over — logs must not fight for the same FD.

**Trade-offs:** Two writers means log lines and progress lines can interleave visually in a TTY that merges streams. Acceptable because (a) logs are level-gated and off by default, (b) `2>` redirection cleanly separates them in scripts, (c) the TUI log panel can *also* tail a log file or show slog records via a custom handler that fans out to both stderr and the viewport.

**Example:**
```go
// internal/logging/logging.go (NEW)
package logging

func Init(mode Mode, levelStr string) *slog.LevelVar {
    lvl := new(slog.LevelVar)
    lvl.Set(parseLevel(levelStr))         // Info default
    var h slog.Handler
    if mode == ModeJSON {
        h = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
    } else {
        h = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
    }
    slog.SetDefault(slog.New(h))
    return lvl
}
// callers: slog.InfoContext(ctx, "episode start", "episode", n, "series", title)
```

## Data Flow

### Request Flow (v1.1 single episode, compression + TUI on)

```
User: animeheaven --url ... --tui --compress
   ↓
main.go: flag.Parse → output.Init(ModeTUI) → logging.Init(Info) → config.Load
   ↓
tui.Run(model)  returns *tea.Program p, sets output.Global = tui.bridge{p}
   ↓                                  ↓ (model.Init returns a Cmd that kicks download.Episode)
download.Episode(ctx, ...)                  │
   ├── media.DownloadParts (workers)  ──► bridge.SegmentProgress ──► p.Send(progressMsg{Stage:"downloading"})
   │                                          └─► model.Update ─► model.View (progress bar)
   ├── mux.MergeEverything (streamcopy) ──► bridge.Progress ──► p.Send(progressMsg{Stage:"muxing"})
   ├── compress.Compress (transcode)    ──► bridge.Progress ──► p.Send(progressMsg{Stage:"compressing"})
   ├── nfo.Write                         ──► slog.Info("nfo written")
   └── dirorganizer.Organize             ──► slog.Info("organized")
   ↓
p.Run() returns when download.Episode finishes (Cmd) or user quits (tea.Quit)
```

### State Management (TUI)

```
tea.Model (single, in program event loop)
    ↑ (p.Send)
    │ progressMsg / stageMsg / errMsg / keyMsg
    │
[workers]──► bridge ──► p ──► Update(msg) ─► mutated model ─► View() ─► tea.View
    │
    └─ slog ──► stderr / log file (independent of model)
```

### Key Data Flows

1. **Progress flow:** worker goroutine → `output.Global.SegmentProgress` → (if TUI) `tea.Program.Send(progressMsg)` → `Update` mutates `model.progress` / `model.stage` → `View` re-renders the progress bar. Throttled to 1/sec to match existing `lastProgressNanos` logic and avoid queue flooding.
2. **Log flow:** any package → `slog.InfoContext(ctx, msg, attrs...)` → stderr TextHandler/JSONHandler. Bounded by `LevelVar` set from `--verbose`. Never touches the TUI buffer directly; the TUI log panel can tail the same log file if a file handler is added.
3. **Compression flow:** `download.Episode` → `mux.MergeEverything` (unchanged, writes temp MKV) → `compress.Compress` (reads temp, writes final, re-applies metadata) → `os.Remove(temp)` → `nfo.Write` + `dirorganizer.Organize`.

## Scaling Considerations

This is a single-user CLI, not a service — "scaling" means download volume, not user count.

| Scale (download volume) | Architecture Adjustments |
|------------------------|--------------------------|
| Single episode (~1GB) | Current pipeline + optional post-mux compress is fine; no change needed |
| Full season (~24GB) | TUI model must handle many episodes (season list panel, viewport pagination); compress pass becomes the wall-clock bottleneck — stream encode progress via `-progress pipe:1` parsing |
| Batch of many series (>100GB) | Need download queue state (already a v1.1 active requirement); compress should be skippable per-episode; consider a resume marker so a failed compress doesn't re-download |

### Scaling Priorities

1. **First bottleneck: wall-clock time when compress is on.** Transcoding a 24-episode season at CRF 18/slow is hours. Mitigation: default `--compress` OFF (preserving v1.0 fast path), expose CRF/preset/codec as flags, and research hardware acceleration (NVENC/AMF/QSV) as a follow-up — note it breaks cross-platform/FFmpeg-consistency guarantees.
2. **Second bottleneck: TUI message queue under heavy parallel downloads.** Per-segment progress from N workers floods `p.Send`. Mitigation: coalesce in the bridge (send at most 1 msg/sec/episode, like the existing throttle) and carry only scalar counts, never payloads.

## Anti-Patterns

### Anti-Pattern 1: Making the TUI the driver of downloads

**What people do:** Put download orchestration inside `tea.Model.Update` / `Init` so the model "does work".
**Why it's wrong:** `Update` runs on the serial event loop; blocking it stalls the UI. Mixing orchestration with rendering breaks the Elm separation and makes the download logic untestable without a terminal.
**Do this instead:** Keep `download.Episode` exactly where it is. The model only *mirrors* state from messages. Kick the download off via a `tea.Cmd` that calls `download.Episode` and emits a completion message; report intermediate progress via `p.Send`.

### Anti-Pattern 2: Replacing `output.Global` entirely with slog

**What people do:** "We have slog now, let's route all the Info/Warn/Progress calls through slog and delete output.Global."
**Why it's wrong:** `output.Progress` is a *rendering* call (carriage-return redraw in human mode, NDJSON event in JSON mode, progress bar in TUI). slog is for *records*. Routing progress through slog forces every consumer to parse a log line to draw a bar, and conflates user output with diagnostics.
**Do this instead:** Keep `output/` for progress/human output; add `logging/` for slog. Two singletons, two responsibilities. The `Reporter` interface (Pattern 1) is the clean seam.

### Anti-Pattern 3: Integrated mux+compress when compression is optional

**What people do:** Add `if cfg.Compress != nil { args = ...transcode... } else { args = ...copy... }` inside `mux.MergeEverything`.
**Why it's wrong:** Makes the moth-tested v1.0 mux path a branch inside a bigger function — every existing mux test now also covers the transcode branch, and a compression bug can regress muxing.
**Do this instead:** Post-mux `compress.Compress` (Option B). Mux stays streamcopy-only; compression is an additive, separable call. `--compress` off = identical to v1.0 behavior, verifiable by test diff = nil.

### Anti-Pattern 4: Letting Bubble Tea own stdout while `--json` also uses stdout

**What people do:** Boot the TUI unconditionally (alt-screen on stdout) and still emit NDJSON.
**Why it's wrong:** The alt-screen TUI repaints stdout; interleaved NDJSON corrupts both the TUI and any JSON consumer parsing stdout.
**Do this instead:** Mutually exclusive modes via `main.go`: `--json` → NDJSON to stdout (no TUI, no alt-screen); `--quiet` → nothing to stdout, errors/minimal to stderr; default human or `--tui` → TUI on stdout. Mode selected once at boot. `--tui` and `--json` together is a usage error (exit with message).

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| FFmpeg (mux) | `exec.CommandContext` streamcopy — unchanged | Already wrapped via `ffmpegCommand` var for testability; keep |
| FFmpeg (compress) | `exec.CommandContext` transcode, second invocation | Parse `-progress pipe:1` or stderr `frame=` lines for TUI percent; option vary |
| Charm Bubble Tea v2 | `charm.land/bubbletea/v2` + `charm.land/lipgloss/v2` + `charm.land/bubbles/v2` | v2 import paths; vanity domain. Pin versions in go.mod |
| Go stdlib `log/slog` | `slog.SetDefault` once in `logging.Init` | go1.26.5 stdlib; no external dep |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `download` ↔ `mux` | Direct call (existing) | Add compress call after mux returns |
| `download` ↔ `compress` | Direct call (new) | Compress is conditional on cfg |
| download/media/mux/compress ↔ `output.Reporter` | Interface call (new seam) | Replaces direct `output.Global.X`; TUI injects bridge impl |
| `output` ↔ `tui` | `tui` implements `output.Reporter`; swaps `output.Global` at boot | Bridge holds `*tea.Program`; thread-safe `Send` |
| `logging` ↔ all packages | `slog.XxxContext(ctx, ...)` global default | No pointer plumbing; uses context for attrs where available |
| `nfo` / `dirorganizer` ↔ `download` | Direct call after compress | Pure functions, no singletons, table-testable |
| `config` ↔ `compress`/`tui`/`logging` | Pointer fields in `config.Config` | Same precedence (CLI > env > config > default) as existing fields |

## Build Order Recommendation (for roadmap)

The features have a dependency chain that dictates build order:

1. **`internal/logging` (slog) first** — no behavioral change, zero risk, unblocks everything by giving later phases a diagnostics channel. Pure additive.
2. **`internal/output.Reporter` interface** — refactor existing `output.Global` calls to go through `Reporter` without changing behavior. Mechanical, test-guarded (existing tests must stay green). Enables the TUI without touching producers.
3. **`internal/compress` (post-mux, default OFF)** — isolated new package, toggleable, default preserves v1.0. Can be developed/tested with the existing mux output as input. Phase-specific research needed on CRF/preset/codec choice.
4. **`internal/nfo` + `internal/dirorganizer`** — pure filesystem ops, no dependency on compress or TUI. Parallelizable with item 3.
5. **`internal/tui` (Bubble Tea v2) last** — depends on the `Reporter` interface (item 2) to wire progress. Most novel surface; benefits from the logger (item 1) for debugging. Bring Bubbles `progress`/`spinner`/`viewport` in together.

**Rationale:** Items 1–2 are refactors that de-risk 3–5. Item 3 (compress) is the highest-uncertainty (CRF tuning, encode time, hardware accel) so research it in its own phase before building. Item 5 is the biggest new surface; do it once progress already flows through the seam, so the TUI is purely a renderer swap.

## Sources

- Bubble Tea v2 README + tutorial * [pkg.go.dev](https://pkg.go.dev/github.com/charmbracelet/bubbletea) — Elm Architecture, `View() tea.View`, `tea.KeyPressMsg`, ProgramOptions — **MEDIUM** (official, current Jul 2026)
- Bubble Tea v2 Upgrade Guide * [github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md](https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md) — declarative View fields, import path `charm.land/bubbletea/v2`, removed options/commands — **MEDIUM** (official)
- Bubbles v2 README * [github.com/charmbracelet/bubbles](https://github.com/charmbracelet/bubbles) — progress/spinner/viewport/list components, v2.1.1 Jul 2026 — **MEDIUM** (official)
- Go `log/slog` package docs * [pkg.go.dev/log/slog](https://pkg.go.dev/log/slog) (go1.26.5) — TextHandler/JSONHandler/LevelVar/DiscardHandler/MultiHandler/LogValuer — **MEDIUM** (official stdlib)
- FFmpeg `ffmpeg` documentation * [ffmpeg.org/ffmpeg.html](https://ffmpeg.org/ffmpeg.html) — streamcopy vs transcoding distinction, stream specifiers, `-c copy` vs `-c:v libx264 -crf` — **MEDIUM** (official, but anime-specific CRF/preset tuning needs phase validation: refer to encode.moe/community fansub guides when available)
- Existing repo: `internal/output/output.go`, `internal/mux/mux.go`, `internal/media/segment.go`, `main.go` — integration seams verified against actual code — **HIGH** (read directly)

---
*Architecture research for: Go CLI media downloader compression + Bubble Tea TUI + structured logging integration*
*Researched: 2026-07-10*