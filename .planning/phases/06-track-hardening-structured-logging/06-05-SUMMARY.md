---
phase: 06-track-hardening-structured-logging
plan: 05
subsystem: mux
tags: [go, ffmpeg, mux, validation, err-04]
requires:
  - phase: 06-track-hardening-structured-logging
    provides: missing-track handling context and ERR-04 mux validation decisions
provides:
  - MergeEverything input stat validation before ffmpeg args are assembled
  - Empty and missing mux input rejection tests that assert ffmpeg is not invoked
affects: [mux, download-error-handling, ffmpeg]
tech-stack:
  added: []
  patterns:
    - size-only os.Stat validation at the single ffmpeg invocation boundary
    - sentinel ffmpegCommand tests for pre-exec validation paths
key-files:
  created:
    - .planning/phases/06-track-hardening-structured-logging/06-05-SUMMARY.md
  modified:
    - internal/mux/mux.go
    - internal/mux/mux_test.go
key-decisions:
  - "Kept ERR-04 validation inside mux.MergeEverything so every caller is protected before ffmpeg args are assembled."
  - "Updated the cleanup-warning test to use the documented empty-string path tolerance instead of a missing file, because missing files are now hard errors."
patterns-established:
  - "Mux input validation skips empty-string paths, wraps os.Stat failures, and rejects zero-byte files before ffmpegCommand."
requirements-completed: [ERR-04]
coverage:
  - id: D1
    description: "MergeEverything validates video, audio, and subtitle input file paths before building ffmpeg args."
    requirement: ERR-04
    verification:
      - kind: unit
        ref: "go test ./internal/mux/... -count=1 -v -run TestMergeEverythingRejects"
        status: pass
      - kind: other
        ref: "go build ./internal/mux/... && go vet ./internal/mux/..."
        status: pass
    human_judgment: false
  - id: D2
    description: "Zero-byte and missing mux inputs return hard errors without invoking ffmpegCommand."
    requirement: ERR-04
    verification:
      - kind: unit
        ref: "internal/mux/mux_test.go#TestMergeEverythingRejectsEmptyVideo"
        status: pass
      - kind: unit
        ref: "internal/mux/mux_test.go#TestMergeEverythingRejectsEmptyAudioTrack"
        status: pass
      - kind: unit
        ref: "internal/mux/mux_test.go#TestMergeEverythingRejectsMissingInput"
        status: pass
    human_judgment: false
duration: 4min
completed: 2026-07-10
status: complete
---

# Phase 06 Plan 05: Mux Input Validation Summary

**FFmpeg mux input validation now rejects empty or missing tracks before positional map args can be built.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-07-10T18:33:04Z
- **Completed:** 2026-07-10T18:36:47Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Added pre-arg-build `os.Stat` validation in `MergeEverything` for video, audio, and subtitle inputs.
- Added hard errors for missing files and zero-byte mux inputs while preserving empty-string path tolerance.
- Added rejection tests proving invalid inputs return before `ffmpegCommand` is invoked.

## Task Commits

1. **Task 1: Insert os.Stat size-only validation in MergeEverything** - `8a64ab2` (fix)
2. **Task 2: mux_test.go empty-input rejection cases with sentinel ffmpeg** - `9d8011e` (test)

**Plan metadata:** skipped (`commit_docs` disabled)

## Files Created/Modified

- `.planning/phases/06-track-hardening-structured-logging/06-05-SUMMARY.md` - Execution summary and verification evidence.
- `internal/mux/mux.go` - Validates mux inputs before ffmpeg args and command creation.
- `internal/mux/mux_test.go` - Adds empty-video, empty-audio, and missing-input rejection tests.

## Decisions Made

- Kept validation in `MergeEverything` rather than callers so ERR-04 is enforced at the single ffmpeg invocation boundary.
- Preserved the existing cleanup-warning coverage by switching that test to the allowed empty-string path case.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated stale cleanup-warning test after missing paths became hard errors**
- **Found during:** Task 2 (mux rejection tests)
- **Issue:** `TestMergeEverythingWarnsButSucceedsWhenCleanupFails` intentionally passed a missing video path, but the new D-10/D-12 validation correctly rejects missing files before ffmpeg.
- **Fix:** Changed that test to use the explicitly tolerated empty-string path so it still covers cleanup warning behavior without contradicting ERR-04.
- **Files modified:** `internal/mux/mux_test.go`
- **Verification:** `go test ./internal/mux/...` passed.
- **Committed in:** `9d8011e`

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** The adjustment aligned an existing test with the new missing-input hard-error contract. No scope expansion.

## Issues Encountered

- Go build/test/vet needed normal Go cache access outside the restricted sandbox.
- Task 2 was marked `tdd="true"`, but Task 1 had already implemented the behavior by plan order. The rejection tests therefore passed when written instead of producing a true RED phase.

## TDD Gate Compliance

- RED commit: not present. The plan ordered implementation before the `tdd="true"` test task, so the feature existed before the rejection tests were written.
- GREEN commit: not applicable as a separate implementation commit for Task 2; implementation was committed as Task 1 (`8a64ab2`).

## Verification

- `go build ./internal/mux/...` - pass
- `go test ./internal/mux/...` - pass (`12 passed in 1 packages`)
- `go vet ./internal/mux/...` - pass
- `go test ./internal/mux/... -count=1 -v -run TestMergeEverythingRejects` - pass (`3 passed in 1 packages`)
- Source checks passed for `os.Stat(path)`, D-11 empty-input error, D-10 stat-error wrapping, no `diag.` references, and no `log/slog` or `internal/diag` test imports.

## Known Stubs

None.

## Threat Flags

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

ERR-04 mux hardening is complete. Plan 06-06 can add strategic ffmpeg diagnostic logging without changing this validation contract.

## Self-Check: PASSED

- Summary file exists at `.planning/phases/06-track-hardening-structured-logging/06-05-SUMMARY.md`.
- Task commits found: `8a64ab2`, `9d8011e`.
- No tracked file deletions were introduced.

---
*Phase: 06-track-hardening-structured-logging*
*Completed: 2026-07-10*
