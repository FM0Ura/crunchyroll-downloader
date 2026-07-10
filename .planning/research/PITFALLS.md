# Pitfalls Research

**Domain:** Go CLI media-downloader — adding FFmpeg compression, Bubble Tea TUI, structured logging, folder metadata to an existing v1.0 tool (5,525 LOC, `output.Global` singleton, `-c copy` mux pipeline, concurrent segment workers)
**Researched:** 2026-07-10
**Confidence:** HIGH (codebase-grounded; Bubble Tea facts verified against charmbracelet/bubbletea v2.0.8 README and upgrade guide)

## How to read this file

This research is scoped to **adding** the v1.1 features to the *existing* codebase, not greenfield design. The most dangerous pitfalls are integration ones — where the new feature collides with v1.0 invariants the team already relies on (the `output.Global` singleton pattern, the `exec.CommandContext` FFmpeg invocation with `-c copy` + dynamic `-map` args, the `errgroup` worker pool). These are flagged **[INTEGRATION]**.

---

## Critical Pitfalls

### Pitfall 1: Re-encoding already-compressed Crunchyroll streams causes visible quality loss AND larger files **[INTEGRATION]**

**What goes wrong:**
Crunchyroll delivers H.264/H.265 streams already encoded at bitrate ceilings set by their CDN. The v1.0 mux path (`internal/mux/mux.go:50`) uses `-c:v copy -c:a copy -c:s copy` — zero loss, near-instant. v1.1 "compression" is chartered to *reduce storage footprint*. Naive fix: switch to `-c:v libx264 -crf 23` (or libx265 `-crf 28`). Result on already-compressed source: **double compression** — the decoder re-flattens DCT coefficients that are already quantized, then re-quantizes them. At CRF 23 you typically get *larger* files than the source (because libx264 adds bits trying to preserve the artifacts the source already has), and at CRF 28+ you get blocking, banding in dark anime scenes, and lost line-art sharpness. Anime is pathological for encoders — flat color regions + hard edges expose quantization grid instantly.

**Why it happens:**
"Compression shrinks files" is intuition from raw/lossless source. Crunchyroll source is neither. Developers reach for CRF because the FFmpeg wiki says it's the modern default, without measuring against the *actual* input.

**How to avoid:**
- Treat compression as an **opt-in, measured** feature, never a default replacement for `-c:v copy`. Keep remux (`-c copy`) as the default codec path; expose compression only behind an explicit `--compress` flag or TUI toggle.
- Before shipping any preset, run a codec-matrix bench on 3 real episodes (1080p action, 1080p slice-of-life, 720p legacy): source size vs. transcoded size, transcode wall-clock, and A/B side-by-side on a dark scene. Define a *size-reduction gate*: if a preset doesn't beat the source by ≥20% on a representative episode, it fails the milestone goal and is not shipped — that's the *point* of the feature.
- Prefer `-c:v libx265 -preset slow -crf 24-26` for storage savings on anime (HEVC is ~40% more efficient than AVC at equal quality); document that HEVC requires a player that supports it (VLC/mpv fine; QuickTime iFFmpeg-tagged MKVs sometimes not). Offer AVC fall back.
- Stream-copy by default; **never** re-encode audio (Crunchyroll AAC/EC-3 is already low-bitrate — a second pass burns CPU and risks sync drift).
- Cap encode threads (`-threads`) and run encode *after* mux, as a separate FFmpeg invocation reading the final MKV — so a compression failure can't corrupt the proven mux pipeline.

**Warning signs:**
- Transcoded file is *bigger* than the remuxed source for ≥1 test episode.
- Dark/night scenes show macroblocking or color banding in side-by-side.
- Encode takes longer than the download itself on a modern CPU.
- Issue reports: "first episode fine, this one looks worse than the streaming player."

**Phase to address:**
Compression research/spike phase (early) — the size-reduction gate must be defined *before* implementation so a failed preset can be cut without sunk-cost pressure.

---

### Pitfall 2: Bubble Tea v2 steals stdio while `output.Global` still owns stdout/stderr — corrupted TUI and lost progress **[INTEGRATION — highest blast radius]**

**What goes wrong:**
v1.0's `output.Global` (`internal/output/output.go:34`) is a process-wide singleton whose every method writes directly via `fmt.Fprintf(os.Stdout, ...)` / `os.Stderr` (lines 112–143). It's called from **8 packages** (`mux`, `download/season`, `download/episode`, `download/segment`, `api/client`, `api/episode`, `api/manifest`). Bubble Tea takes ownership of the terminal — its cell-based renderer repaints the screen on every `View()`. If *any* code path still calls `output.Global.Info(...)` while the TUI is running, those raw writes land in the middle of the renderer's frame buffer: interleaved garbage, broken progress lines,CursorPosition desync, and in alt-screen mode the messages vanish entirely. The charmbracelet README is explicit: *"You can't really log to stdout with Bubble Tea because your TUI is busy occupying that!"*

There's also a v2-specific trap: **Bubble Tea v2's `View()` returns `tea.View`, not a `string`** (see `tea.NewView(s)` in the v2.0.8 tutorial). Anyone copying from v1 tutorials/StackOverflow will write `func (m model) View() string` and get a compile error or — worse — a `interface{}` assertion panic depending on the build. `UPGRADE_GUIDE_V2.md` is the required reading.

**Why it happens:**
The TUI is added as a new layer *on top of* the existing output sink. Nobody threads `output.Global` out of the 8 call sites — they assume Bubble Tea "just renders" and the old prints become harmless. They are not harmless; they are corrupting.

**How to avoid:**
- **First refactor of the TUI phase: replace the `output.Global` sink with a `tea.Program`-aware emitter.** Keep the `Outputter` interface (`Info/Warn/Error/Debug/Progress`) but implement a `bubbleteaOutput` that converts each call into a `tea.Msg` (`progressTickMsg`, `infoMsg`, `errorMsg`) sent via `p.Send(...)`. Then the `View()` renders from accumulated model state — single owner of stdio.
- When the program is *not* a TUI (piped output, `--json`, `--quiet`, `CI=true`), fall back to the existing humanOutput/jsonOutput/quietOutput unchanged. Gate on `term.IsTerminal` + a `--no-tui` flag. This preserves the JSON mode that v1.0 already tests.
- Send **all** diagnostic output to a file via `tea.LogToFile("debug.log", "debug")` (the Charm-endorsed pattern), or to the slog sink (see Pitfall 5). Never `fmt.Println` from inside the running program.
- Decide alt-screen vs inline up front. Alt-screen (`tea.WithAltScreen`) wipes scrollback — fine for a full interactive download-manager view, hostile for a one-shot download where users expect output in their shell history. For a downloader, prefer **inline** mode so completed lines scroll away normally; use alt-screen only for the season-progress dashboard if at all.
- Hook SIGINT/SIGTERM through Bubble Tea's `tea.QuitMsg` path, not a separate `signal.NotifyContext`. v1.0 already has a graceful-signal cleanup contract (`internal/download/episode.go:108` cleans stream + temp files) — that contract must run on `tea.Quit`, not be bypassed. Bubble Tea installs its own signal handler; letting two handlers fight over cleanup is a recipe for orphaned temp files and leaked `DELETE /playback/v1/token/` calls.

**Warning signs:**
- Progress bar "flickers" or splits into two halves.
- `[Episode n/m]` informational lines appear on top of, not above, the progress bar.
- `--json` output now contains stray ANSI/bracketed text (the renderer is leaking into the JSON stream).
- Temp files (`*.part`, decrypted segments) linger after Ctrl-C where v1.0 cleaned them.
- Tests that exercise `output.Global` start racing under `-race` because the program and the test both install signal handlers.

**Phase to address:**
TUI phase — but the `output.Global` → message-bridge refactor is the **very first task**, not "polish at the end." If it's deferred, every subsequent TUI subtask fights corrupted output.

---

### Pitfall 3: Bubble Tea's `Update` loop blocks on download I/O — frozen UI, dropped keypresses **[INTEGRATION]**

**What goes wrong:**
The Elm Architecture demands `Update(msg) (Model, Cmd)` return fast. v1.0's download path does work synchronously inside `download.Episode` and friends (segment fetch loop with `errgroup`, FFmpeg `cmd.Run()`). If the team wires downloads directly into `Update` (`case startDownloadMsg: download.Episode(...); return m, nil`), the entire event loop stalls for the *whole download*: no keypresses processed, no progress repaints, no Ctrl-C. The renderer stops ticking; users report "it's frozen but the file is growing."

**Why it happens:**
The natural-looking translation of v1.0's top-level `download.Run(ctx)` into a Bubble Tea `Cmd` hides that `Cmd`s are expected to be `func() tea.Msg` and run on a worker — but only if you actually *return a `Cmd`*. A common mistake is doing the work inline in `Update` "to keep it simple," which runs it on the render goroutine.

**How to avoid:**
- All long work is a `tea.Cmd` (`func() tea.Msg`) that Bubble Tea runs off the render goroutine. For the segment/worker pool, emit a `progressMsg` via `p.Send` from inside the errgroup (same place `output.Global.Progress` is called today — `internal/media/segment.go:198`). That call site becomes the single integration seam: swap `output.Global.Progress(...)` for `program.Send(progressMsg{...})`.
- Throttle progress sends. The segment loop fires progress on every segment; a 1080p episode is hundreds of segments. Sending a `tea.Msg` per segment saturates the msg channel and the renderer. Debounce to ~10–15 fps (e.g. emit only if ≥67ms since last send, or use `tea.Tick` / `tea.Every`). The existing `SpeedTracker` already does rolling windows — reuse it as the throttle gate.
- Keep the **cancellation contract**: pass a `context.Context` derived from the model's "quit" state into every `cmd.Run()` / `http.Request`. Bubble Tea quitting must cancel the ctx that FFmpeg sees (`exec.CommandContext`), the errgroup, and the HTTP client — same chain v1.0 uses today, just rooted in the TUI lifecycle instead of `signal.NotifyContext`.

**Warning signs:**
- UI repaints freeze the moment a download starts, then "catches up" in a burst when it ends.
- Ctrl-C during download has no effect until the current operation finishes.
- `go test -race` reports a data race on the model after concurrent progress sends.
- Progress bar advances in chunks every second rather than smoothly.

**Phase to address:**
TUI phase — design the message/command boundary before "make it pretty."

---

### Pitfall 4: Missing / mismatched audio & subtitle tracks silently produce broken or empty MKVs **[INTEGRATION]**

**What goes wrong:**
v1.0 builds FFmpeg args by *counting* inputs (`-map 0:v:0`, then `-map {1+i}:a:0`, then `-map {1+len(audio)+j}`) — `internal/mux/mux.go:42–48`. The mapping is positional and assumes the slice the caller passed equals the files actually written. The milestone char's "graceful error handling for missing audio/subtitle tracks" implies a track *can* be absent. Failure modes:

1. **Requested locale not in manifest** → empty adaptation set. v1.0 already has an "Empty adaptation set guard" (Validated), but if a missing track is downgraded to a *warning* rather than an error, the slice passed to `MergeEverything` shrinks, `-map` indices still resolve — but the file the user gets is *missing a dub/category theyselected* with no obvious in-file indication.
2. **Subtitle fetch returns 0 bytes / 404 for one locale** → a `.srt`/`.ass` temp file is created empty; `-map` of an empty stream makes FFmpeg emit an empty subtitle track that some players (mpv) silently ignore and others (VLC) render as a blank line at the bottom for the whole episode.
3. **`-map` index off-by-one** if any audio download failed mid-season and the caller "compacted" the slice — track language metadata (`-metadata:s:a:%d language=...`) ends up stamped onto the wrong stream (e.g. the Japanese track labeled Portuguese). Metadata/language mislabeling is the #1 silent-data-corruption bug in multi-dub muxers.

**Why it happens:**
The existing muxer trusts its caller. v1.0's caller only ever fed it complete, pre-validated slices. The moment "missing track = soft skip" is introduced, the contract "slice length == input count == file count" becomes something that must be enforced, not assumed.

**How to avoid:**
- Make the muxer **verify** its inputs: before launching FFmpeg, `os.Stat` each `MediaTrack.File` and require `size > 0`; return a typed error (`ErrEmptyTrack{Locale, Kind}`) otherwise. Never let FFmpeg be the thing that discovers an empty input — its error is unreadable.
- Pass tracks as **`[]MediaTrack` carrying their intended locale**, and have the muxer assert that every requested locale is represented in the slice *before* building `-map` indices. If a locale is genuinely optional (user said "download these if available"), separate the "required" set from the "best-effort" set at the call site and surface a structured warning per missing best-effort track, not a silent gap.
- Encode the mapping **by track identity, not position**: build the args in a single pass and keep a parallel `[]trackMeta` so the `language=`/`title=`/`disposition=` args always index in lockstep with `-map`. A helper that appends input+map+metadata together per track eliminates the off-by-one class.
- On delete, surface the missing ones in the season summary (`internal/download/season.go:78` already prints "X of Y failed") — extend it to "missing audio: [ja-JP], missing subs: [pt-BR]" as structured warnings.
- Test the partial-mux path explicitly: an episode where one of three audio locales is unavailable must produce a valid MKV with the other two, correctly labeled, and a user-visible warning listing the gap.

**Warning signs:**
- Players show a subtitle track that's blank / can't be turned off.
- Track language dropdown lists "und" (undetermined) for some dubs.
- User reports "I picked Japanese but it defaulted to English" — metadata got stamped on the wrong stream downtown.
- `ffprobe` on output shows fewer streams than the user requested with no warning in the run log.

**Phase to address:**
Track-handling-hardening phase — must land *before* compression, because a broken partial-mux produced by v1.0 logic then *re-encoded* by the compression path bakes the breakage into a transcoded file that can't be trivially remuxed to fix.

---

### Pitfall 5: `slog` default JSON handler + per-segment logging = perf tax and PII leakage **[INTEGRATION]**

**What goes wrong:**
v1.1 charters "structured strategic logging for bug/error identification." Reach implementation: replace `output.Global.Debug` with `slog.Info(...)` using the default JSON handler writing to a file, at Info level, from the hot segment loop. Two flavors of pain:

- **Perf:** `slog`'s default JSON handler does a full `json.Marshal` of the record (with `time.RFC3339Nano`, attrs) on **every call**. The segment loop currently calls `output.Global.Progress` per segment (~hundreds/episode) *and* `Debug` on every API call. At Info level with per-segment events, a 13-episode season at 1080p emits tens of thousands of JSON records; marshaling + file write per record adds visible wall-clock on slow disks and, worse, contends with the same `os.Stdout` lock if log-to-stderr-with-TTY is chosen.
- **PII:** crash-logs now persist. The auth path (`internal/api/client.go:103` token refresh, `CRUNCHYROLL_ETP_RT` cookie, `CRUNCHYROLL_CLIENT_AUTH` Basic) and request bodies (currently dumped via `output.Global.Debug("\n%s", body)` in `api/episode.go` and `api/manifest.go`) will, if logged at Debug level to a file, write bearer tokens and the Basic-auth header to disk in cleartext. A user who shares `debug.log` in a bug report leaks their Crunchyroll account.

**Why it happens:**
"Structured logging" is treated as "turn on slog, ship." The defaults are Info+JSON+stdout; nobody audits what flows through Debug, and nobody writes a redaction layer because the existing `Debug("\n%s", body)` was *only* going to a TTY the user controlled.

**How to avoid:**
- Use `slog.LevelDebug` only when `--debug` (or `DEBUG=1`); default to `slog.LevelInfo` and emit *strategic* events (episode start/finish, FFmpeg invocation summary, token refresh, seasonal failure). NOT per-segment. The segment loop keeps using the in-process progress mechanism (`output.Global`/TUI), not the log file.
- Write logs to a fixed, configurable path (`~/.config/animeheaven/logs/animeheaven.log`) — NOT stdout (see Pitfall 2) and NOT cwd. Implement **size-capped rotation** (lumberjack-style: rotate at 5–10 MB, keep 3). Unbounded single-file logging in a "leave it running overnight for a season" tool will eventually fill a small disk.
- **Redact before log.** Wrap the slog handler in a `ReplaceAttr`/custom handler that scrubs: `Authorization`, `Cookie`, `etp_rt`, `client_id`, `private_key`, and any `*_token`. Treat the Bearer token as PII by default — log only `"token": "<redacted>"` and `token_len`, never the value. The existing body-dumps (`Debug("\n%s", body)`) must be gated behind `--debug-raw` (a separate, dangerous flag) or removed.
- Prefer `slog.NewTextHandler` over JSON for the local log file — it's ~2× faster to marshal, vastly easier to `tail -f`, and the "structured" goal is satisfied (key=value is structured). Reserve JSON for a future `--log-format=json` opt-in.
- Bench: log a 10k-episode-worth of records and measure the overhead. If logging adds >5% to total wall-clock, drop the level or batch.

**Warning signs:**
- `debug.log` contains the literal `Authorization: Bearer <jwt>` string.
- Log file grows unbounded over a multi-day season download.
- Total download time regresses vs v1.0 in a benchmark with `--debug`.
- A user files a bug with `debug.log` attached and the maintainer has to ask them to rotate their token.

**Phase to address:**
Logging phase — land **with** redaction and rotation, not after. PII in logs is a ship-blocker, not a polish item.

---

### Pitfall 6: Folder metadata injection writes to the wrong scope / fights the muxer **[INTEGRATION]**

**What goes wrong:**
v1.0 already injects *file-level* MKV metadata (`-metadata:g title=...`, `show=...`, `track=...`, `season_number=...` — `mux.go:83–87`). "Folder metadata injection" adds a layer on top — typically a `tvshow.nfo`/`series.nfo` (Kodi/Jellyfin/Plex convention) and/or `poster.jpg` next to the MKVs. Common mistakes:

- Writing `tvshow.nfo` at the *episode* folder — Plex reads it for the show root only; placing it per-season re-tag the show with the *first season's* metadata and breaks mixed-season libraries.
- Fetching poster URL but never downloading it, or downloading to `poster.jpg` then having FFmpeg's temp-file cleanup (the `warnRemove` loop, `mux.go:103–108`) delete it because it lives next to the temp mux files. v1.0's cleanup removes files in the mux input list; a stray cleanup helper that's "helpfully" tidying the output dir will eat the `.nfo`/`.jpg`.
- Metadata XML with unescaped `&`, `<`, quotes from the series title — Jellyfin/Plex will refuse to parse the NFO and silently fall back to filename heuristics, undoing the work.
- `season_number` in v1.0 is set to `EpisodeNumber` (look: `mux.go:87`) — that's a latent bug *already*. The number stamped as "season number" is actually the episode number. Adding folder-level `season=<n>` metadata surfaces this as a visible mismatch (Plex shows season 1 = "season 5 episodes"). Don't build the folder layer on top of a broken file layer.

**Why it happens:**
Folder metadata is treated as additive decoration. It's not — it competes with file metadata in the scanner's precedence chain, and it lives in a directory the existing cleanup code already touches.

**How to avoid:**
- Define the folder layout contract first: `<out>/<Series Title>/Season NN/Season NN - Episode NN - <Title>.mkv`, `tvshow.nfo` only at `<out>/<Series Title>/`, `poster.jpg` at the same level, per-season `seasonNN.tbn` optional. Scan-able by Plex/Jellyfin/Kodi out of the box.
- Fix the existing `season_number` bug (it's `EpisodeNumber`, should be `SeasonNumber`) **before** mirroring it into the NFO. Phase the fix: file metadata correctness → folder metadata.
- Generate NFO with `encoding/xml` marshal, never `fmt.Sprintf`. Test with titles containing `&`, `<`, `"`, `'`, emoji, CJK.
- Make cleanup **exact-path**, never glob/wildcard in the output dir. The `warnRemove` loop should only ever touch the explicit temp inputs it was handed. Add a test that drops a `poster.jpg` and a `tvshow.nfo` in the output dir and asserts the post-download cleanup leaves them.
- Download artwork out-of-band (not blocking the FFmpeg path) and treat its failure as non-fatal — a missing poster must not fail a successful 30-minute download.

**Warning signs:**
- Plex/Jellyfin shows every episode as "Season 1, Episode N" regardless of actual season.
- `poster.jpg` or `tvshow.nfo` missing after a download that "succeeded."
- A series with `&` in the title (e.g. "Hidamari Sketch × Honeycomb") produces an NFO the scanner ignores.
- `git blame` shows the cleanup helper was edited to "also clean the output dir."

**Phase to address:**
Output-organization / metadata phase — AFTER track-hardening and BEFORE/AFTER compression (fix the latent `season_number` bug first; it's a one-line correctness fix that unblocks metadata trust).

---

### Pitfall 7: Bubble Tea tests can't exercise the real progress/stdio path — false-green suite **[INTEGRATION]**

**What goes wrong:**
v1.0 has a real test suite (9 packages, `-race`, CI). Adding Bubble Tea naively breaks it two ways:

- Tests that assert on `output.Global` (e.g. `output_test.go` checks `humanOutput.Info` writes `\033[K` + content) now pass *because the test still uses the old sink* — but production now uses the bubbletea bridge. The tests are green but no longer exercise the production path.
- Bubble Tea testing uses `tea.TestProgram` (v2) / `teatest` which runs the model in a headless virtual terminal. If the team *only* tests the model's `Update`/`View` in isolation, they miss the integration: a `progressMsg` sent from a real `errgroup` worker through `p.Send` into the model, asserting the rendered View shows "Downloaded N/M segments." Skipping that leaves the throttle/cancellation contract untested.

**Why it happens:**
Test-time wiring is invisible. The old tests didn't break, so nobody noticed they stopped being meaningful.

**How to avoid:**
- Keep the `Outputter` interface as the seam, add a `bubbleteaOutput` test-double (`*tea.Program` injected), and write at least one integration test that: spins a fake segment producer → sends `progressMsg`s through the real `Program` (using `teatest`) → asserts the final View string contains expected counts and that a `tea.QuitMsg` cancels the producer ctx with a bounded timeout.
- Delete or rewrite assertions that lock in v1.0's ANSI literal escapes if those literals are now renderer-controlled; otherwise the suite is testing a dead code path.
- Run CI **without** a TTY (`CI=true` forced) AND with a pty (`script -e -c` or `go test` under a pseudo-tty wrapper) so both the non-TUI fallback and the TUI path are exercised. A downloader that breaks under `cron`/`systemd` because the TUI hard-required a TTY is a regression.

**Warning signs:**
- Coverage % stays the same or rises but the new `internal/tui/` package has <30% coverage of its msg-handling paths.
- `--json` tests pass, `--tui` has no tests, and CI only runs the former.
- A bug ships that only reproduces when stdout is a real terminal.

**Phase to address:**
TUI phase (test design is a first-class deliverable, not a follow-up).

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Keep `output.Global` raw-write singleton and "just run Bubble Tea alongside it" | Saves rewiring 8 call sites for v1.1 launch | Corrupted TUI frames, intermittent CI races, eventually a forced rewrite once users hit it | **Never** — this is the root cause of Pitfall 2 |
| Default to transcoding instead of `-c copy` to "save space" | Smaller files in the happy case | Quality loss on already-compressed source, sometimes *larger* files; user trust loss | Only as explicit opt-in with a measured size-reduction gate |
| Per-segment `slog.Info` to file | "Complete" audit log for debugging | Disk fill, perf regression, PII leakage | Never at Info — max Debug behind `--debug`, debounced |
| Write NFO via `fmt.Sprintf` | Quick to ship | XML-injection bug on one `&`-titled series breaks the library scan | Only if you add an escaper in the same commit; otherwise use `encoding/xml` |
| `exec.Command("ffmpeg", "-y", ...)` overwrite everywhere | Avoids "file exists" prompts | Silent overwrite of a 30-min encode from a prior interrupted run — user loses the good file | Acceptable on temp inputs, **not** on the final output |
| Glob-cleanup the output directory "to be tidy" | Recovers disk from crashed runs | Deletes `poster.jpg`/`tvshow.nfo` (Pitfall 6); deletes a user's adjacent files | Never — cleanup must be exact-path on known temp inputs |
| Treat missing track as silent skip | Downloads "succeed" more often | User gets a dub/category they didn't pick, no warning; trust erosion | Only with a structured warning emitted per missing track and surfaced in the season summary |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Bubble Tea ↔ `output.Global` | Letting both write stdout concurrently | Bridge: `output.Global` becomes a `tea.Msg` sender under TUI; falls back to file/stderr sink behind `term.IsTerminal` gate |
| Compression ↔ `internal/mux` mux pipeline | Inserting `-c:v libx264` into the existing `MergeEverything` arg list so a transcode failure corrupts the proven mux | Run remux first (`-c copy`, unchanged), then a **separate** `ffmpeg -i out.mkv -c:v libx265 ... compressed.mkv` invocation so failures are isolated and the good mux survives |
| Compression ↔ cleanup (`warnRemove`) | Re-encode in place and let `warnRemove(mux.go:102)` delete the same path you're writing | Read the muxed MKV as input, write to a distinct temp path for the compressed output; only on success move into place and delete the remux |
| `slog` ↔ `api/client` token path | Logging the full HTTP response/request at Debug | `ReplaceAttr` redaction wrapping the handler; bearer/cookie/Basic-auth values become `<redacted>`; raw body-dump behind a separate `--debug-raw` flag only |
| Bubble Tea ↔ signal handling (v1.0's `signal.NotifyContext` for SIGINT/SIGTERM) | Installing your own `signal.Notify` after Bubble Tea installs its own | Derive ctx for downloads/FFmpeg from the model quit event; let Bubble Tea own the signal → `tea.QuitMsg` → your cleanup runs in the `tea.Quit` handler, mirroring v1.0's `episode.go:108` cleanup order |
| Folder metadata ↔ cleanup | Cleanup helper globs the output dir | Exact-path cleanup of the explicit temp inputs only; test asserts `poster.jpg`/`tvshow.nfo` survive |
| Track selection ↔ manifest-empty adaptation set | Soft-skip a missing locale → slice shrinks → `-map` indices and `-metadata:s:a:N` language tags go out of sync | Verify each `MediaTrack.File` non-empty before building args; build input+map+metadata per-track in one pass; surface missing best-effort tracks as structured warnings |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Per-segment `tea.Msg` send | Renderer saturates, UI freezes in bursts | Throttle to 10–15 fps using existing `SpeedTracker` window; bundle segment events | Any 1080p episode (hundreds of segments) |
| `slog` JSON handler per-segment at Info | Wall-clock regression, disk lock contention | Default Info emits only episode/season events; per-segment stays in the in-process progress path | ≥1 full season, multi-GB log files |
| libx264/x265 single-threaded by default in naive args | Transcode = 2× download time on multi-core box | `-preset` + `-threads 0` (auto) + measure; consider software vs hardware (NVENC/VAAPI) only as opt-in due to cross-platform variance (Linux/macOS/Windows constraint) | Any 1080p encode without thread flags |
| Re-encoding audio (AAC/EC-3 double pass) | CPU burn, possible A/V drift on long episodes | Always `-c:a copy` for compression path; only re-encode audio if a size audit says it matters AND the multi-dub count is ≥4 | Multi-dub episodes with 4+ audio tracks |
| In-process speed tracker under TUI | Racy reads from renderer + workers | Keep the `sync.Mutex`-guarded `RecordBytes`/`SpeedBps` API; messages snapshot a copy, never share the tracker pointer | As soon as TUI displays live speed/ETA |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Logging bearer/refresh tokens, `client_id`, `private_key`, cookies at Debug | Cleartext account credentials on disk; leak if user shares `debug.log` | `ReplaceAttr` redaction wrapper as part of the handler, not bolted on later; raw body dump only behind explicit `--debug-raw` |
| Persisting `CRUNCHYROLL_ETP_RT` / `CRUNCHYROLL_CLIENT_AUTH` into the log file via ambient-env capture | Token lives in plaintext log even after rotate | Never log env var values; log only their presence (`"etp_rt": "set"`) |
| NFO embeds the user's account/email (if scraped from API) | PII in a file users commonly sync to media servers / cloud | Strip user-account fields from NFO; keep only show/episode metadata |
| Compressed re-encode drops Widevine-related stdev/encryption metadata | Re-encode reads decrypted stream — must never write decrypted intermediate to a stable, predictable path | Keep decrypted segments in `os.CreateTemp` (v1.0 pattern) and shred-on-success; do not hardcode names |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Alt-screen TUI wipes scrollback after a download | User loses the history of which episodes finished in a batch | Inline mode by default; alt-screen only for an explicit interactive mode flag |
| Compression runs with no progress feedback for minutes | "Is it hung?" force-quits → corrupt partial encode → no output file | Pipe FFmpeg stderr progress (`-progress pipe:2`) into the same throttle/View path so encode has its own progress line after download |
| Missing dub not surfaced | User watches the wrong dub track before noticing; blames the tool | Warn loudly at episode-completion summary AND in the TUI View list per missing track |
| `poster.jpg` download failure fails the whole episode | 30-min download "fails" because artwork 404'd | Artwork out-of-band, non-fatal; log a warning, ship the MKV |
| Default logs to stdout broke piping (`animeheaven … > out.txt`) | V1.0 quiet/json honor this; v1.1 must not regress | TUI only when `term.IsTerminal(stdout) && term.IsTerminal(stdin)` and no `--json`/`--quiet`; else fallback to v1.0 sinks |

## "Looks Done But Isn't" Checklist

- [ ] **Compression:** Quality actually beats source at the chosen CRF — verify A/B side-by-side on a dark anime scene (not just file size)
- [ ] **Compression:** Size-reduction gate met on ≥1 representative episode (≥20% smaller than remux); if not, preset is cut, not shipped
- [ ] **Compression:** Compressed path does NOT re-encode audio (`-c:a copy` verified)
- [ ] **TUI:** `output.Global` no longer writes stdout while a `tea.Program` is running — verified by `-race` integration test sending `progressMsg` through `Program`
- [ ] **TUI:** SIGINT during download cleans temp files + sends `DELETE /playback/v1/token/` (v1.0 contract preserved)
- [ ] **TUI:** Non-TTY fallback (`CI=true`, `--json`, `--quiet`, piped stdout) produces byte-identical output to v1.0
- [ ] **TUI:** Tests cover the `tea.Msg` → `View` path with `teatest`; not just isolated `Update` unit tests
- [ ] **Logging:** `debug.log` does NOT contain bearer/cookie/client_id/private_key even at `--debug` — grep test asserts redaction
- [ ] **Logging:** Log file rotates (lumberjack-style) and lives under `~/.config/animeheaven/logs/`, not cwd, not stdout
- [ ] **Logging:** Per-segment events do not reach `slog` at Info — episode/season/FFmpeg-summary granularity only
- [ ] **Missing tracks:** Episode with one absent dub produces valid MKV with remaining tracks correctly language-labeled + user-visible warning listing the gap
- [ ] **Missing tracks:** `ffprobe` on output shows stream count == user-selected track count (or fewer, with a logged warning)
- [ ] **Folder metadata:** `season_number` is `SeasonNumber` (currently `EpisodeNumber` — latent bug fixed), mirrored into NFO
- [ ] **Folder metadata:** `tvshow.nfo` lands only at `<out>/<Series>/`, `poster.jpg` survives cleanup, NFO parses with `&`/`<`/CJK titles
- [ ] **Cleanup:** No glob/wildcard deletes in output dir; `poster.jpg`/`tvshow.nfo`/user files survive a download run (regression test exists)
- [ ] **Compression+cleanup:** Compressed output written to a distinct temp path, moved into place only on success; remuxed intermediate cleaned only after the move

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| `output.Global` × Bubble Tea stdio collision | HIGH | Force-major refactor: convert the 8 call sites to the `tea.Msg` bridge (Pitfall 2/7). No incremental patch survives — raw `fmt.Fprintf` and a renderer cannot coexist. Re-run the full `-race` suite under a pty. |
| Shipped a transcoding default that loses quality | MEDIUM | Revert default to `-c copy`; demote compression to `--compress`; re-bench CRF/preset against representative episodes with the size-reduction gate. Already-transcoded user files: document a `--revert` remux-from-source (requires re-download) or accept the loss. |
| Bearer token leaked into `debug.log` | HIGH | Rotate the user's Crunchyroll token; add `ReplaceAttr` redaction; scrub existing logs (provide a `animeheaven logs purge`); test with a grep-literal-token CI rule so it can't regress. |
| Metadata language mislabels dubs (off-by-one `-map`) | MEDIUM | Fix the per-track input+map+metadata helper; for already-mislabeled MKVs, `ffmpeg -i in.mkv -c copy -metadata:s:a:N language=XX out.mkv` to relabel without re-encode. Add the `ffprobe`-stream-count assertion to the suite. |
| `season_number = EpisodeNumber` latent bug surfaced by folder metadata | LOW | One-line fix (`mux.go:87`: `info.EpisodeMetadata.EpisodeNumber` → `SeasonNumber`); relabel existing MKVs with a `-c copy` remux if needed. Catch before NFO generation. |
| Cleanup deleted `poster.jpg`/`tvshow.nfo` | LOW | Switch `warnRemove` to exact-path-only; add the survival regression test; redownload artwork for affected series (non-fatal path). |
| Compression failure corrupted final output path | MEDIUM | Isolate: write compressed output to a disjoint temp, atomically `os.Rename` on success; on failure, fall back to the existing remuxed MKV (which must still exist — never overwrite-in-place). |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| 1 — Re-encode quality/size double-compression | Compression spike (early) | Size-reduction gate met on ≥1 episode; A/B visual check on dark scene; `-c:a copy` asserted in args |
| 2 — Bubble Tea × `output.Global` stdio collision | TUI phase — *first task* | `-race` test: `Program` running, `progressMsg` sent; no raw stdout writes; non-TTY fallback byte-identical to v1.0 |
| 3 — `Update` loop blocks on download I/O | TUI phase | Downloads run as `tea.Cmd`; Ctrl-C cancels ctx and cleans temp within bounded timeout; UI repaints ≥10 fps during download |
| 4 — Missing/mismatched tracks → broken MKVs | Track-hardening phase (before compression) | `ffprobe` stream count == selected count (or fewer + warning); empty-input rejected before FFmpeg; episode with absent dub produces valid MKV + warning |
| 5 — `slog` perf tax + PII leak | Logging phase | grep test: no bearer/cookie/client_id in logs at `--debug`; rotation rotates at threshold; default level is Info with no per-segment events; wall-clock bench ≤ v1.0 +5% |
| 6 — Folder metadata scope / cleanup conflict / latent `season_number` bug | Output-org / metadata phase (after track-hardening) | `season_number == SeasonNumber`; `tvshow.nfo` only at series root; `poster.jpg` survives cleanup; NFO parses with `&`/`<`/CJK |
| 7 — TUI test suite false-green | TUI phase (tests as deliverable) | `teatest` integration covering msg→View; CI runs both pty and headless; coverage of `internal/tui/` msg handlers ≥70% |

**Phase ordering rationale:**
1. **Track-hardening** first — Pitfall 4 underpins everything; a broken partial-mux that later gets *re-encoded* (Pitfall 1) bakes the breakage into a transcoded file that's expensive to undo.
2. **Output-org / metadata** next — fixes the latent `season_number` bug (Pitfall 6) so both file and folder metadata become trustworthy before more layers land on it; cheap, unblocks user-facing correctness.
3. **Logging** in parallel — Pitfall 5 has no upstream dependency and the redaction wrapper is needed from day one *regardless* of which feature is being debugged; ship early, gate on `--debug`.
4. **Compression spike** before Compression build — Pitfall 1 requires measured gates; a 1-2 day spike de-risks the preset choices and can cut losers before implementation cost.
5. **TUI** — highest blast radius (Pitfalls 2, 3, 7); depends on track-hardening and logging being stable because its tests need a clean Outputter/Logger seam. The `output.Global` → `tea.Msg` bridge must be the first TUI task, not the last.

**Research flags for phases:**
- Compression phase: needs a hands-on CRF/preset matrix spike on real Crunchyroll episodes — research alone won't settle it; data will.
- TUI phase: needs a real-terminal (pty) CI leg; standard `go test` under no-TTY exercises only the fallback path.
- Logging phase: needs an obtuse redaction-review (manual grep for token-like substrings in a `--debug` log from a live auth flow); not solvable by unit tests alone.

## Sources

- `internal/output/output.go` (v1.0 source — singleton, `fmt.Fprintf` to stdout/stderr, `Outputter` interface, `Init`/mode switch) — MEDIUM→HIGH (primary codebase truth)
- `internal/mux/mux.go` (v1.0 source — `-c copy` mux, positional `-map` `1+i`/`1+len(audio)+j`, `-metadata:s:a:%d language=...`, `warnRemove`, `season_number=EpisodeNumber` latent bug) — HIGH
- `internal/download/{episode,season,segment}.go`, `internal/api/{client,episode,manifest}.go` (v1.0 call sites of `output.Global`) — HIGH
- charmbracelet/bubbletea README + `UPGRADE_GUIDE_V2.md` (v2.0.8, 2026-07-03): "You can't really log to stdout with Bubble Tea," `tea.LogToFile`, `View()` returns `tea.View` (v2 breaking change), Delve headless debug requirement — HIGH (official)
- Go standard library `log/slog` (Go 1.21+): default JSON handler per-record marshal cost, `ReplaceAttr` for redaction, `LevelDebug`/`LevelInfo` — HIGH (stdlib docs)
- FFmpeg `libx264`/`libx265` encoding wiki: CRF semantics, double-compression on already-quantized source (domain-intro knowledge; verify via CRF matrix spike) — MEDIUM
- Kodi/Jellyfin/Plex NFO scanner precedence & `tvshow.nfo`/`seasonNN.tbn` placement conventions — MEDIUM (community wiki; cross-check against one scanner's docs before implementation)

---
*Pitfalls research for: Go CLI anime downloader v1.1 (compression, Bubble Tea TUI, structured logging, folder metadata)*
*Researched: 2026-07-10*