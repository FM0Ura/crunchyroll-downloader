---
phase: 07-organized-output-folder-metadata
plan: 01
subsystem: mux
tags: [ffmpeg, mkv-metadata, season-number, regression-test]

# Dependency graph
requires:
  - phase: 06-track-hardening-structured-logging
    provides: diag.MuxLogger slog plane used by MergeEverything (unchanged here)
provides:
  - "Correct MKV container metadata: season_number arg now equals api.EpisodeMetadata.SeasonNumber (D-05/OUT-03), unblocking NFO emission in Plan 03"
affects: [07-03 (NFO metadata), 07-02 (organized output), 07-04 (artwork)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "File-based subprocess arg-capture seam (GO_HELPER_ARGS_FILE) for asserting FFmpeg metadata args without a real ffmpeg binary"

key-files:
  created: []
  modified:
    - "internal/mux/mux.go (line 108: season_number arg source EpisodeNumber→SeasonNumber)"
    - "internal/mux/mux_test.go (TestMergeEverythingSetsCorrectSeasonNumber + restoreFFmpegCommandWithArgCapture + TestHelperProcess arg-capture extension)"

key-decisions:
  - "D-05 one-line fix: season_number arg now sources info.EpisodeMetadata.SeasonNumber instead of EpisodeNumber; SeasonNumber is already populated in both single-episode and season flows"
  - "Used a file-based arg-capture seam (GO_HELPER_ARGS_FILE) rather than sharing positional maps because the existing TestHelperProcess subprocess can only exchange state via env vars and the filesystem"
  - "Kept testEpisodeInfo() (Season=1, Episode=1) unchanged so all pre-existing tests pass byte-for-byte; the regression test builds its own distinct fixture (Season=2, Episode=7)"

patterns-established:
  - "GO_HELPER_ARGS_FILE env var: when set, TestHelperProcess writes the ffmpeg trailing args (one per line) to that path, enabling regression tests to assert on assembled -metadata:g values without a real ffmpeg"

requirements-completed: [OUT-03]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "MKV container metadata season_number equals SeasonNumber, not EpisodeNumber (OUT-03 / Pitfall 6 latent v1.0 bug fix)"
    requirement: "OUT-03"
    verification:
      - kind: unit
        ref: "internal/mux/mux_test.go#TestMergeEverythingSetsCorrectSeasonNumber"
        status: pass
      - kind: automated
        ref: "go test ./internal/mux/... -count=1 (full package green; -race green)"
        status: pass
    human_judgment: false

# Metrics
duration: 1 min
completed: 2026-07-10
status: complete
---

# Phase 7 Plan 1: season_number Metadata Bug Fix Summary

**One-line correctness fix at internal/mux/mux.go:108 sourcing SeasonNumber (not EpisodeNumber) into the FFmpeg `-metadata:g season_number=` arg, proven by a regression test with distinct Season=2/Episode=7 values**

## Performance

- **Duration:** 1 min
- **Started:** 2026-07-10T22:47:30Z
- **Completed:** 2026-07-10T22:49:11Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments

- Fixed the latent v1.0 `season_number=EpisodeNumber` bug at `internal/mux/mux.go:108` (D-05 / OUT-03 / Pitfalls Pitfall 6) — the MKV container metadata now stamps the actual season number, so Jellyfin/Kodi will show the correct season once Phase 7 emits NFO mirroring the file metadata.
- Added `TestMergeEverythingSetsCorrectSeasonNumber`, a regression test using a file-based arg-capture seam (`GO_HELPER_ARGS_FILE`) that asserts the captured FFmpeg args contain `season_number=2` (SeasonNumber) and NOT `season_number=7` (EpisodeNumber). Distinct Season=2 / Episode=7 values make the bug detectable.
- Extended the existing `TestHelperProcess` subprocess helper to write the ffmpeg trailing args to `GO_HELPER_ARGS_FILE` when set, and added a `restoreFFmpegCommandWithArgCapture` helper that wires the env var — a non-invasive extension of the established subprocess seam (no new process, no testify, stdlib-only per v1.0 D-03).
- Verified the test FAILS on the pre-fix line (proof of bug-detection) and PASSES after the one-line fix; full `internal/mux` package green under `-race`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Fix season_number bug at internal/mux/mux.go:108 and add regression test** - `d96a2c7` (fix)

_Plan metadata commit will follow this SUMMARY._

## Files Created/Modified

- `internal/mux/mux.go` - Line 108: `season_number=` metadata arg now sources `info.EpisodeMetadata.SeasonNumber` (was `EpisodeNumber`). Sibling `track=` (line 107) still uses EpisodeNumber; title (line 105) already used SeasonNumber correctly; stderr/cleanup block (lines 113-121) untouched.
- `internal/mux/mux_test.go` - Added `TestMergeEverythingSetsCorrectSeasonNumber` (distinct Season=2/Episode=7 fixture, asserts captured args), `restoreFFmpegCommandWithArgCapture` helper, and the `GO_HELPER_ARGS_FILE` arg-capture branch in `TestHelperProcess`. Existing `testEpisodeInfo()` (Season=1/Episode=1) left unchanged so all pre-existing tests stay byte-for-byte green.

## Decisions Made

- **D-05 fix location confirmed:** The plan and 07-CONTEXT.md both pin the bug to `internal/mux/mux.go:108`. The fix is exactly the one-line field swap the plan specified; `fmt.Sprintf("%v", ...)` wrapper and surrounding args unchanged.
- **Arg-capture seam via filesystem, not positional-map sharing:** The existing `TestHelperProcess` runs in a child `os.Args[0]` subprocess and can only exchange state with the parent through env vars and the filesystem. There was no pre-existing args-capture helper to reuse, so a minimal file-based capture (`GO_HELPER_ARGS_FILE`) was added — stdlib-only, opt-in (unset → existing behavior unchanged).
- **Distinct fixture kept local to the new test:** Rather than mutating the shared `testEpisodeInfo()` helper (which would couple every existing test to distinct numbers and risk masking other bugs), the regression test builds its own `*api.EpisodeInfo` with SeasonNumber=2 / EpisodeNumber=7. The shared helper stays at 1/1 so the seven pre-existing tests pass unchanged.
- **No TDD RED gate required:** `gsd-tools query task.is-behavior-adding` returned `false` for this plan (no `tdd="true"` frontmatter on the task), so the MVP+TDD gate did not trip. The fix and regression test were committed together; the test was independently verified to FAIL on the pre-fix line and PASS after the fix.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- OUT-03 delivered: MKV container metadata `season_number` now equals `SeasonNumber`. The file-metadata correctness layer is trustworthy before any folder-metadata / NFO layer lands on top (Pitfalls Pitfall 6: "fix the latent `season_number` bug before mirroring it into the NFO").
- Plumbing unblocked for Plan 03 (NFO emission): it can mirror the correct season number into the per-episode `.nfo` without re-introducing the v1.0 bug.
- No new functions, types, fields, or package paths introduced; `MergeEverything` signature unchanged — purely an internal behavior change.

## TDD Gate Compliance

- Plan type: `execute` (not `tdd`).
- `task.is-behavior-adding` predicate: `false` (task frontmatter has no `tdd="true"`). MVP+TDD gate did not trip; no separate RED test commit was required.
- Regression test was nevertheless verified to FAIL on the pre-fix line (season_number=7) and PASS after the one-line fix (season_number=2), providing equivalent bug-detection proof.

---
*Phase: 07-organized-output-folder-metadata*
*Completed: 2026-07-10*