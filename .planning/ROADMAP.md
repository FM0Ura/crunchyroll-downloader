# Roadmap: AnimeHeaven

**Created:** 2026-07-08
**Scope:** Performance, usability, UX, and code quality improvements to the Crunchyroll downloader.

## Milestones

- ✅ **v1.0 Improvement & Optimization Pass** — Phases 1-5 (shipped 2026-07-10)
- 🚧 **v1.1 Storage, CLI & Error Handling** — Phases 6-10 (in progress)

## Phases

<details>
<summary>✅ v1.0 Improvement & Optimization Pass (Phases 1-5) — SHIPPED 2026-07-10</summary>

- [x] Phase 1: Foundation (5/5 plans) — completed 2026-07-08
- [x] Phase 2: Performance (3/3 plans) — completed 2026-07-09
- [x] Phase 3: Usability (5/5 plans) — completed 2026-07-09
- [x] Phase 4: UX (3/3 plans) — completed 2026-07-10
- [x] Phase 5: Testing (2/2 plans) — completed 2026-07-10

</details>

### 🚧 v1.1 Storage, CLI & Error Handling (In Progress)

**Milestone Goal:** Reduce storage footprint via opt-in compression, modernize the CLI with a Bubble Tea TUI, harden download error handling for missing tracks, organize output into Jellyfin-friendly folders with NFO metadata, and add structured strategic logging for diagnostics.

- [ ] **Phase 6: Track Hardening + Structured Logging** - Graceful missing-track handling and configurable slog diagnostics with PII redaction
- [ ] **Phase 7: Organized Output + Folder Metadata** - Jellyfin folder layout, NFO metadata, artwork, and the season_number bug fix
- [ ] **Phase 8: Compression Presets** - Intent-named opt-in post-mux re-encode presets gated on a >=20% size-reduction spike
- [ ] **Phase 9: Output Reporter Seam** - Mechanical interface refactor decoupling progress producers from renderers (unblocks TUI)
- [ ] **Phase 10: Bubble Tea TUI** - Interactive TUI with live progress, episode checklist, and track picker

## Phase Details

### Phase 6: Track Hardening + Structured Logging
**Goal**: Users get graceful missing-track handling so partial tracks no longer abort downloads, plus a configurable structured diagnostic log with automatic rotation and PII redaction
**Depends on**: Nothing (first v1.1 phase; builds on the shipped v1.0 pipeline)
**Requirements**: ERR-01, ERR-02, ERR-03, ERR-04, LOG-01, LOG-02, LOG-03, LOG-04, LOG-05
**Success Criteria** (what must be TRUE):
  1. User can download an episode where a requested non-primary audio or subtitle language is missing and gets a warn + skip of that track, not an abort
  2. User gets a clear hard error (no silent empty MKV) when ALL audio tracks are missing
  3. Muxer rejects empty or mismatched input tracks before invoking FFmpeg, so an episode's dubs never get mislabeled by positional -map indices
  4. User can set `--log-level` and `--log-file` and observe structured diagnostic logs scoped per subsystem (download/drm/media/mux/api) with strategic events only (episode start/finish, FFmpeg summary, token refresh, season failure)
  5. User's log file auto-rotates (5-10MB, 3 backups) and never leaks bearer tokens, cookies, etp_rt, client_id, or private_key
**Plans**: TBD

### Phase 7: Organized Output + Folder Metadata
**Goal**: Users get Jellyfin/Kodi-friendly organized download folders with correct NFO metadata and fetched artwork
**Depends on**: Phase 6 (uses the new logger for non-fatal artwork-fetch warnings)
**Requirements**: OUT-01, OUT-02, OUT-03, META-01, META-02, META-03
**Success Criteria** (what must be TRUE):
  1. User's downloads land in a `Series Title (Year)/Season 01/Series Title S01E01 - Title.mkv` folder structure
  2. Re-running a series download skips already-downloaded episodes in the nested layout (resumability preserved)
  3. MKV and NFO metadata use the correct season number — the latent v1.0 `season_number=EpisodeNumber` bug is fixed
  4. User gets a `tvshow.nfo` at the series root and a per-episode `.nfo` with `<uniqueid type="crunchyroll">`, both readable by a real Jellyfin and a real Kodi scan
  5. User gets `poster.jpg`/`backdrop.jpg` when available; a 404 on artwork does not fail the download
**Plans**: TBD

### Phase 8: Compression Presets
**Goal**: Users can opt into intent-named compression presets that measurably reduce storage without surprise, with a proven remux preserved on any compression failure
**Depends on**: Phase 7 (correct metadata + fixed season_number before any re-encode bakes a bug in)
**Requirements**: COMP-01, COMP-02, COMP-03, COMP-04, COMP-05, COMP-06, COMP-07
**Success Criteria** (what must be TRUE):
  1. User can choose compression by intent name via `--compress copy|balanced|space|best` (HandBrake-style, not raw codec/CRF flags), with anime-tuned encoding and HEVC presets producing ~40-50% smaller files
  2. User gets a clear fail-fast error before any download starts if their FFmpeg lacks the encoder a chosen preset needs
  3. A compression failure leaves the good remux MKV intact on disk (post-mux separate invocation, distinct temp, atomic rename)
  4. User sees live compression progress (frame/out_time parsed into the display) with no silent multi-minute stalls
  5. Each shipped preset beats source size by >=20% on >=1 real Crunchyroll episode, or it is cut before shipping
**Plans**: TBD

### Phase 9: Output Reporter Seam
**Goal**: Progress producers are decoupled from renderers via a Reporter interface so a future TUI can be injected as a pure renderer swap, with zero behavioral change to existing output
**Depends on**: Phase 6 (routing through the seam builds on the same output package touched by logging)
**Requirements**: TUI-05
**Success Criteria** (what must be TRUE):
  1. All progress producers across the codebase call a Reporter interface (Info/Warn/Error/Debug/Progress/SegmentProgress) rather than `output.Global` directly
  2. A new renderer (the future TUI bridge) can be injected behind the Reporter interface without touching producer code
  3. Existing `--json`/`--quiet`/piped output modes remain unchanged and their tests stay green (no regression to v1.0/v1.1 contracts)
**Plans**: TBD
**UI hint**: yes

### Phase 10: Bubble Tea TUI
**Goal**: Users running the tool interactively get a Bubble Tea TUI with live progress bars, an episode checklist, and a track picker, replacing scrolling `\r` lines — with non-interactive modes and clean shutdown preserved
**Depends on**: Phase 9 (Reporter seam) + Phase 6 (file logging frees stdout for the TUI)
**Requirements**: TUI-01, TUI-02, TUI-03, TUI-04
**Success Criteria** (what must be TRUE):
  1. User running the tool interactively sees live progress bars, an episode checklist, and a track picker instead of scrolling `\r` progress lines
  2. User can enable the TUI with `--tui`; it auto-enables only when stdout is a TTY and `--json`/`--quiet` are absent
  3. Non-interactive modes (`--json`/`--quiet`/piped) keep working unchanged with a stable NDJSON output contract
  4. Ctrl+C / SIGINT in the TUI triggers clean shutdown — temp files released, stream tokens deleted, no regression vs v1.0 cleanup behavior
**Plans**: TBD
**UI hint**: yes

## Progress

**Execution Order:**
Phases execute in numeric order continuing from v1.0: 6 → 7 → 8 → 9 → 10. (Phase 9 is mechanically independent of Phase 8 and may be planned in parallel; both must precede Phase 10.)

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation — Error Handling, HTTP, Memory | v1.0 | 5/5 | Complete | 2026-07-08 |
| 2. Performance — Caching & Parallelism | v1.0 | 3/3 | Complete | 2026-07-09 |
| 3. Usability — Configuration & Validation | v1.0 | 5/5 | Complete | 2026-07-09 |
| 4. UX — Progress & Output | v1.0 | 3/3 | Complete | 2026-07-10 |
| 5. Testing & Quality | v1.0 | 2/2 | Complete | 2026-07-10 |
| 6. Track Hardening + Structured Logging | v1.1 | 0/TBD | Not started | - |
| 7. Organized Output + Folder Metadata | v1.1 | 0/TBD | Not started | - |
| 8. Compression Presets | v1.1 | 0/TBD | Not started | - |
| 9. Output Reporter Seam | v1.1 | 0/TBD | Not started | - |
| 10. Bubble Tea TUI | v1.1 | 0/TBD | Not started | - |

---

*Roadmap created: 2026-07-08*
*Last updated: 2026-07-10 after v1.1 milestone roadmap creation*
