# Requirements: AnimeHeaven (Crunchyroll Downloader)

**Defined:** 2026-07-10
**Core Value:** Download any anime episode or full season from Crunchyroll into a single playable MKV file with chosen audio and subtitle tracks.

## v1.1 Requirements

Requirements for milestone v1.1 (Storage, CLI & Error Handling). Each maps to a roadmap phase.

### Error Handling

- [x] **ERR-01**: User can download an episode when a requested non-primary audio language is missing (warn + skip that track, not abort)
- [x] **ERR-02**: User can download an episode when a requested non-primary subtitle language is missing (warn + skip that track, not abort)
- [x] **ERR-03**: User gets a hard error when ALL audio tracks are missing (no silent empty MKV)
- [x] **ERR-04**: Muxer rejects empty or mismatched input tracks before invoking FFmpeg (os.Stat each input, reject empty) to prevent positional -map mislabeling dubs

### Logging

- [x] **LOG-01**: User can configure structured log output via `--log-level` (Debug/Info/Warn/Error) and `--log-file` (destination path) flags
- [x] **LOG-02**: Diagnostic logs are scoped per subsystem (download/drm/media/mux/api) via slog WithGroup so a failed-run log dump reads with component grouping
- [x] **LOG-03**: Log files are rotated automatically (lumberjack, 5-10MB max, 5 backups) to avoid unbounded disk growth on long download runs
- [x] **LOG-04**: Structured logs redact PII (bearer tokens, cookies, etp_rt, client_id, private_key) via ReplaceAttr wrapper — no secrets written to disk
- [x] **LOG-05**: Default log level emits strategic events only (episode start/finish, FFmpeg summary, token refresh, season failure) — not per-segment noise

### Output Organization

- [ ] **OUT-01**: Downloads are organized into `Series Title (Year)/Season 01/Series Title S01E01 - Title.mkv` folder structure (Jellyfin/Kodi convention)
- [ ] **OUT-02**: Re-running a series download skips already-downloaded episodes using the new nested folder layout (resumability preserved)
- [x] **OUT-03**: The latent `season_number=EpisodeNumber` bug in mux.go is fixed to use SeasonNumber before metadata is mirrored into NFO

### Metadata

- [ ] **META-01**: User gets a `tvshow.nfo` at the series root so Jellyfin/Kodi/Emby display correct title/plot without relying on online scraper matching
- [ ] **META-02**: User gets a per-episode `.nfo` alongside each `.mkv` with `<uniqueid type="crunchyroll">` for stable re-scrapes even when titles drift
- [ ] **META-03**: User gets `poster.jpg`/`backdrop.jpg` artwork fetched from Crunchyroll API (non-fatal on 404, download still succeeds)

### Compression

- [ ] **COMP-01**: User can choose compression by intent-named preset (`--compress copy|balanced|space|best`) — HandBrake-style, not raw codec/CRF flags
- [ ] **COMP-02**: Compression presets apply anime-tuned encoding (`-tune animation`) for flat color regions + line art quality
- [ ] **COMP-03**: User can opt into HEVC/H.265 (`space`/`best` presets) for ~40-50% file size reduction vs AVC source
- [ ] **COMP-04**: Tool probes FFmpeg encoder availability (libx265/libx264) at startup and fails fast with a clear error if a preset's encoder is missing
- [ ] **COMP-05**: Compression runs as a post-mux separate FFmpeg invocation (distinct temp, os.Rename on success) so a compress failure leaves the good remux intact
- [ ] **COMP-06**: User sees compression progress (FFmpeg frame=/out_time_ms parsed into progress display) — no silent multi-minute stalls
- [ ] **COMP-07**: Each shipped preset passes a >=20% size-reduction gate measured on real Crunchyroll episodes (preset is cut if it doesn't beat source by >=20% on >=1 episode)

### CLI/TUI

- [ ] **TUI-01**: User gets an interactive Bubble Tea TUI with live progress bars, episode checklist, and track picker (replaces scrolling `\r` lines)
- [ ] **TUI-02**: User can enable TUI via `--tui` flag; TUI auto-enables only when stdout is a TTY and `--json`/`--quiet` are absent
- [ ] **TUI-03**: Non-interactive modes (`--json`/`--quiet`/piped) remain unchanged and TUI-disableable; NDJSON output contract stays stable
- [ ] **TUI-04**: Ctrl+C/SIGINT in TUI triggers tea.Quit -> ctx cancel -> existing cleanup (temp files released, stream tokens deleted) with no regression
- [ ] **TUI-05**: Output Reporter interface in `output/` decouples progress producers from renderers, enabling the TUI to inject a bridge implementation as a pure renderer swap

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Compression (advanced)

- **COMP-08**: User can choose 10-bit encoding (`yuv420p10le`) presets for anime banding reduction (deferred — playback compat risk)
- **COMP-09**: User can override CRF directly via `--crf` advanced flag (deferred — presets cover common cases first)
- **COMP-10**: User can opt into HW-accelerated encoding (NVENC/QSV) for faster compression (deferred — breaks cross-platform consistency)

### Metadata (advanced)

- **META-04**: User gets season-specific poster artwork (`season01-poster.jpg`) per season folder
- **META-05**: Tool enriches NFO with online TVDB/anidb metadata matching (deferred — uniqueid approach avoids scraper mismatch without this)

### Logging (advanced)

- **LOG-06**: User can switch default log format to JSON via `--log-format=json` (deferred — TextHandler default sufficient for v1.1)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Default-on re-encoding/compression | Re-encoding is lossy, slow, CPU-heavy; surprising users with smaller-but-lossy files breaks trust. Copy remains default. |
| 2-pass bitrate-target encoding | CRF is better for anime (constant quality, variable bitrate); 2-pass targets file size at quality cost |
| Audio re-encoding | Crunchyroll AAC/EC-3 is already low-bitrate; second pass burns CPU and risks audio sync drift |
| "Download lower quality to compress" | Conflates two separate features; compression works on whatever quality user selected |
| Full GUI or web interface | CLI-only by design (v1.0 decision, unchanged) |
| Online TVDB/anidb NFO enrichment | Scope too broad; `<uniqueid type="crunchyroll">` solves scraper mismatch without external API dependency |
| Batch transcode of pre-existing MKVs | This is a download tool, not a transcoder; compression applies to fresh downloads only |
| Replacing Outputter with slog | Different planes — slog is diagnostic, Outputter is user-facing; both coexist |
| Auto-delete original MKV after compress | Data loss risk; user keeps both until they choose to clean up |
| Raw `--ffmpeg-args` escape hatch | Bypasses presets, produces unpredictable results, breaks support contract |
| HW accel (NVENC/QSV) as default | Breaks cross-platform consistency (Linux/macOS/Windows); deferred to v2 opt-in |
| testify test framework | D-03 decision from v1.0: table-driven stdlib tests only |
| go-flags/pflag/cobra CLI framework | Keeping stdlib flag parsing (v1.0 decision); Bubble Tea adds TUI, not flag parsing |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| ERR-01 | Phase 6 | Complete |
| ERR-02 | Phase 6 | Complete |
| ERR-03 | Phase 6 | Complete |
| ERR-04 | Phase 6 | Complete |
| LOG-01 | Phase 6 | Complete |
| LOG-02 | Phase 6 | Complete |
| LOG-03 | Phase 6 | Complete |
| LOG-04 | Phase 6 | Complete |
| LOG-05 | Phase 6 | Complete |
| OUT-01 | Phase 7 | Pending |
| OUT-02 | Phase 7 | Pending |
| OUT-03 | Phase 7 | Complete |
| META-01 | Phase 7 | Pending |
| META-02 | Phase 7 | Pending |
| META-03 | Phase 7 | Pending |
| COMP-01 | Phase 8 | Pending |
| COMP-02 | Phase 8 | Pending |
| COMP-03 | Phase 8 | Pending |
| COMP-04 | Phase 8 | Pending |
| COMP-05 | Phase 8 | Pending |
| COMP-06 | Phase 8 | Pending |
| COMP-07 | Phase 8 | Pending |
| TUI-05 | Phase 9 | Pending |
| TUI-01 | Phase 10 | Pending |
| TUI-02 | Phase 10 | Pending |
| TUI-03 | Phase 10 | Pending |
| TUI-04 | Phase 10 | Pending |

**Coverage:**

- v1.1 requirements: 27 total
- Mapped to phases: 27 ✓
- Unmapped: 0

---
*Requirements defined: 2026-07-10*
*Last updated: 2026-07-10 after v1.1 milestone roadmap creation (traceability filled)*
