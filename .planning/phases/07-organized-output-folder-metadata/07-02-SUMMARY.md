---
phase: 07-organized-output-folder-metadata
plan: 02
subsystem: download
tags: [output-layout, jellyfin, kodi, nested-folders, resumability, season-folder]

# Dependency graph
requires:
  - phase: 06-track-hardening-structured-logging
    provides: diag.DownloadLogger slog plane reused for the unchanged skip info line
  - phase: 07-organized-output-folder-metadata (plan 01)
    provides: Correct SeasonNumber in api.EpisodeMetadata feeding the new Season %02d folder name
provides:
  - "Nested Jellyfin/Kodi-friendly output: Series Title/Season NN/Series Title S01E01 - Title.mkv (OUT-01, D-01/D-03/D-04)"
  - "Resumability on the deeper nested path — os.Stat skip keys off the deep .mkv (OUT-02, D-02)"
  - "Uniform nested layout for single-episode downloads mirroring season downloads (D-03)"
affects: [07-03 (NFO placement at series root + per-episode .nfo beside the deep .mkv), 07-04 (artwork at series root), 08 (compression writes into the nested path)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Season %02d zero-padded subfolder via fmt.Sprintf from an int (no user input in the segment — path-traversal-safe)"
    - "Deep-path resumability: os.Stat on the fully-joined Series Title/Season NN/file.mkv path"
    - "Seam-merge-counter test pattern: package-level episodeMerge override + t.Cleanup restore, asserts invocation count"

key-files:
  created: []
  modified:
    - "internal/download/episode.go (output-path block lines 71-86: Season %02d subfolder mkdir + dropped [%s] quality bracket + outputFile joins under seasonPath)"
    - "internal/download/episode_test.go (TestOutputDirCreatesSeriesSubfolderInOutputDir updated for Season 01 nesting; + TestEpisodeOutputSkipsAlreadyDownloadedInNestedLayout; + TestEpisodeSingleEpisodeMirrorsSeasonLayout)"

key-decisions:
  - "D-01 honored: NO parenthetical year in the series root — path is Series Title/Season NN/... (overrides OUT-01's literal (Year) string)"
  - "D-02 honored: os.Stat skip check reuses the SAME deeper outputFile variable keys resumability off Series Title/Season NN/file.mkv"
  - "D-03 honored: single-episode flow (totalEpisodes=1) produces the SAME Series Title/Season NN/ nesting as a season episode — proven by TestEpisodeSingleEpisodeMirrorsSeasonLayout"
  - "D-04 honored: *.mkv filename drops the [{quality}] bracket; *videoQuality still drives media.GetVideoBaseUrl upstream and is logged via diag.DownloadLogger.Info(episode_start), it just no longer appears on disk"
  - "Kept the Episode function signature unchanged (incl. videoQuality *string) — no public API change; *videoQuality still required for media selection"

patterns-established:
  - "Season-folder mkdir with a distinct 'creating season directory: %w' error wrap, parallel to the existing 'creating output directory: %w' wrap"
  - "New download tests assert on the captured outputFile passed into the episodeMerge seam (via a closure-captured var), not on filesystem layout alone — proves the produced path shape without a real ffmpeg"

requirements-completed: [OUT-01, OUT-02]

# Coverage metadata (#1602) — one entry per shipped deliverable.
coverage:
  - id: D1
    description: "Downloads land in the nested Series Title/Season NN/Series Title S01E01 - Title.mkv layout with no (Year) and no [{quality}] bracket (OUT-01 / D-01 / D-03 / D-04)"
    requirement: "OUT-01"
    verification:
      - kind: unit
        ref: "internal/download/episode_test.go#TestOutputDirCreatesSeriesSubfolderInOutputDir"
        status: pass
      - kind: unit
        ref: "internal/download/episode_test.go#TestEpisodeSingleEpisodeMirrorsSeasonLayout"
        status: pass
      - kind: automated
        ref: "go build ./internal/download/... + go vet ./internal/download/... (exit 0)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Re-running an episode download skips because the deep Series Title/Season NN/.mkv already exists — resumability keys off the nested deep path (OUT-02 / D-02)"
    requirement: "OUT-02"
    verification:
      - kind: unit
        ref: "internal/download/episode_test.go#TestEpisodeOutputSkipsAlreadyDownloadedInNestedLayout"
        status: pass
      - kind: automated
        ref: "go test ./internal/download/... -count=1 (full package green)"
        status: pass
    human_judgment: false

# Metrics
duration: 1 min
completed: 2026-07-10
status: complete
---

# Phase 7 Plan 2: Organized Nested Output Layout Summary

**Nested Jellyfin/Kodi-friendly `Series Title/Season NN/Series Title S01E01 - Title.mkv` output (no year, no quality bracket) with resumability preserved on the deeper path, for both single-episode and season flows**

## Performance

- **Duration:** 1 min
- **Started:** 2026-07-10T22:50:50Z
- **Completed:** 2026-07-10T22:52:32Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Restructured the output-path construction in `internal/download/episode.go` (lines 71-86) so every download — single-episode OR season — lands under `Series Title/Season NN/Series Title S01E01 - Title.mkv`, matching Jellyfin/Kodi scan convention (zero-padded `Season %02d` folder generated from `info.EpisodeMetadata.SeasonNumber`).
- Dropped the `[{quality}]` bracket from the on-disk `.mkv` filename (D-04). The `*videoQuality` parameter stays in the `Episode` signature because it still drives `media.GetVideoBaseUrl` upstream and is logged via `diag.DownloadLogger.Info("episode_start", ...)`; it just no longer appears in the filename.
- Preserved D-02 resumability: the `os.Stat(outputFile)` skip check now keys off the deeper `Series Title/Season NN/file.mkv` path — same skip idiom, deeper path — with the `...skipped (already downloaded)` info line text unchanged.
- Kept `--output-dir` semantics intact: the `outputBase = filepath.Join(outputDir, cleanSeriesTitle)` computation is unchanged, so the series root still nests under `outputDir` when set.
- Updated `TestOutputDirCreatesSeriesSubfolderInOutputDir` to assert BOTH the series-root AND the nested `Season 01` subfolder are created inside `outputDir`.
- Added `TestEpisodeOutputSkipsAlreadyDownloadedInNestedLayout`: pre-creates the deep `.mkv` at `Series Title/Season 01/...`, asserts `Episode` returns nil, the mux seam is NOT invoked (counter stays 0), and the `...skipped (already downloaded)` info line fires — proving D-02 resumability on the deeper path.
- Added `TestEpisodeSingleEpisodeMirrorsSeasonLayout`: invokes `Episode` with `totalEpisodes=1` (single-episode flow) and asserts the captured outputFile contains the `Series Title/Season 01/` segment with no quality bracket — proving D-03 uniform nesting.

## Task Commits

Each task was committed atomically:

1. **Task 1: Build the nested `Series Title/Season NN/` output path and drop the `[{quality}]` filename bracket** - `a55496b` (feat)
2. **Task 2: Update path tests for the nested layout + resumability on the deep path** - `8bc00a0` (test)

_Plan metadata commit will follow this SUMMARY._

## Files Created/Modified

- `internal/download/episode.go` - Output-path block (lines 71-86): after the existing `os.MkdirAll(outputBase, ...)` success, introduces `seasonDir := fmt.Sprintf("Season %02d", info.EpisodeMetadata.SeasonNumber)` + `seasonPath := filepath.Join(outputBase, seasonDir)` + `os.MkdirAll(seasonPath, 0777)` with a `creating season directory: %w` error wrap; the `outputFile` now joins under `seasonPath` with the format `"%s S%02dE%02d - %s.mkv"` (no `[%s]` quality bracket, no `*videoQuality` arg). The `os.Stat(outputFile)` skip check text is unchanged. The `Episode` function signature is unchanged.
- `internal/download/episode_test.go` - `TestOutputDirCreatesSeriesSubfolderInOutputDir` now asserts both `Series Title/` and `Series Title/Season 01/` exist under `outputDir`. Added `TestEpisodeOutputSkipsAlreadyDownloadedInNestedLayout` (pre-creates deep `.mkv`, asserts mux seam counter == 0 + skip info line) and `TestEpisodeSingleEpisodeMirrorsSeasonLayout` (totalEpisodes=1, asserts captured outputFile contains `Season 01` segment and no `[` bracket). Added `fmt` import. No testify; reuses `restoreEpisodeTestSeams` + closure-captured seam-counter pattern.

## Decisions Made

- **No separate TDD RED commit required:** `gsd-tools query task.is-behavior-adding` returned `is_behavior_adding: false` for this plan (no `tdd="true"` frontmatter on the tasks; `<behavior>` block absent). MVP+TDD gate did not trip. The plan is `type: execute` (not `type: tdd`), so the plan's own task ordering (Task 1 = source, Task 2 = tests) was followed rather than a RED→GREEN cycle. The new tests were nevertheless verified to PASS against the implemented code, and the full `internal/download` suite stays green.
- **Season folder name uses `%02d` from an int:** The `Season %02d` segment is generated via `fmt.Sprintf` from `info.EpisodeMetadata.SeasonNumber` (an `int`), so it cannot carry user-controlled path separators or `..` — the trust-boundary mitigation in the plan's `<threat_model>` holds without new code (T-07-02-T mitigated by the existing `sanitizeFilename` + int-derived segment).
- **No `(Year)` introduced:** D-01 explicitly overrides OUT-01's literal `Series Title (Year)/` string. The series root is the sanitized Series Title only; no year literal anywhere in the path construction (verified by grep).
- **`*videoQuality` parameter retained:** Removing it from the signature would break `media.GetVideoBaseUrl` media selection and the `episode_start` slog event; D-04 only removes the bracket from the on-disk filename, not the parameter.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- OUT-01 delivered: downloads land in `Series Title/Season 01/Series Title S01E01 - Title.mkv` (no `(Year)` per D-01; no `[{quality}]` per D-04).
- OUT-02 delivered: re-running an episode download skips because the deep `.mkv` already exists (proven by `TestEpisodeOutputSkipsAlreadyDownloadedInNestedLayout`).
- D-03 delivered: single-episode downloads produce the SAME nested tree as season downloads (proven by `TestEpisodeSingleEpisodeMirrorsSeasonLayout`).
- Unblocks Plan 03 (NFO placement): the per-episode `.nfo` can be written beside the deep `.mkv` (`strings.TrimSuffix(outputFile, ".mkv") + ".nfo"`) and the `tvshow.nfo` at the `outputBase` series root, both inside the now-established nested layout.
- No new exported functions, types, struct fields, or package paths introduced; the `Episode` signature is unchanged — purely an internal output-path behavior change.

## TDD Gate Compliance

- Plan type: `execute` (not `tdd`).
- `task.is-behavior-adding` predicate: `false` (task frontmatter has no `tdd="true"`; no `<behavior>` block). MVP+TDD gate did not trip; no separate RED test commit was required.
- The plan's own structure provides test coverage as Task 2, committed after the implementation (Task 1). All tests pass against the implemented code; the full `internal/download` package is green.

## Self-Check: PASSED

- `internal/download/episode.go` — FOUND on disk
- `internal/download/episode_test.go` — FOUND on disk
- `.planning/phases/07-organized-output-folder-metadata/07-02-SUMMARY.md` — FOUND on disk
- Commit `a55496b` (feat, Task 1) — FOUND in git log
- Commit `8bc00a0` (test, Task 2) — FOUND in git log
- `go build ./internal/download/...` — exit 0
- `go vet ./internal/download/...` — exit 0
- `go test ./internal/download/... -count=1` — PASS (full package green)
- All plan-level `<verification>` checks satisfied