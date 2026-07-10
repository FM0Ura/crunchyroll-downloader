---
phase: 06-track-hardening-structured-logging
plan: 04
subsystem: download
tags: [missing-tracks, audio, subtitles, ndjson, tests]

requires:
  - phase: 06-track-hardening-structured-logging
    provides: "Phase 6 context decisions D-01..D-09 and existing output.Global warn contract"
provides:
  - "Primary audio/subtitle locales now hard-error when unavailable"
  - "Secondary missing audio/subtitle locales now warn and skip"
  - "Partial episode success now emits a separate warning listing skipped tracks"
  - "Episode tests cover primary errors, secondary skips, partial warnings, and empty audio guard"
affects: [download, output, testing]

tech-stack:
  added: []
  patterns:
    - "Unexported package-level seams around Episode external I/O boundaries for deterministic tests"
    - "Missing secondary tracks reuse output.Global.Warn and the existing NDJSON warn event"

key-files:
  created:
    - .planning/phases/06-track-hardening-structured-logging/06-04-SUMMARY.md
  modified:
    - internal/download/episode.go
    - internal/download/episode_test.go

key-decisions:
  - "Kept skip surfacing on output.Global.Warn only, preserving the existing NDJSON warn contract."
  - "Added narrow unexported Episode seams so tests can exercise successful partial downloads without real DRM, HTTP segment downloads, or FFmpeg."

patterns-established:
  - "Primary track is the first requested locale and remains protected by a hard error."
  - "Secondary missing tracks are accumulated as '<locale> dub/sub' and reported after the success line."

requirements-completed: [ERR-01, ERR-02, ERR-03]

coverage:
  - id: D1
    description: "Missing primary audio locale returns a hard error naming the primary locale."
    requirement: ERR-01
    verification:
      - kind: unit
        ref: "internal/download/episode_test.go#TestEpisodeReturnsErrorForUnavailableAudioLocale"
        status: pass
      - kind: unit
        ref: "internal/download/episode_test.go#TestEpisodeReturnsPrimaryAudioErrorBeforePostLoopGuard"
        status: pass
    human_judgment: false
  - id: D2
    description: "Missing secondary audio locale warns and skips while the episode completes."
    requirement: ERR-01
    verification:
      - kind: unit
        ref: "internal/download/episode_test.go#TestEpisodeSkipsUnavailableSecondaryAudioLocale"
        status: pass
    human_judgment: false
  - id: D3
    description: "Missing secondary subtitle locale warns and contributes to partial episode reporting while subtitle download failures remain hard errors."
    requirement: ERR-02
    verification:
      - kind: unit
        ref: "internal/download/episode_test.go#TestEpisodeWarnsWhenDownloadedPartially"
        status: pass
      - kind: other
        ref: "rg -n 'downloading subtitles for %s: %w' internal/download/episode.go"
        status: pass
    human_judgment: false
  - id: D4
    description: "Empty audio language list returns before muxing so no empty MKV is produced."
    requirement: ERR-03
    verification:
      - kind: unit
        ref: "internal/download/episode_test.go#TestEpisodeReturnsErrorWhenAudioLangsEmptyBeforeMux"
        status: pass
    human_judgment: false
  - id: D5
    description: "NDJSON contract is unchanged and skip events reuse the existing warn event."
    requirement: ERR-02
    verification:
      - kind: other
        ref: "git diff --name-only -- internal/output/ndjson.go"
        status: pass
    human_judgment: false

duration: 8min
completed: 2026-07-10
status: complete
---

# Phase 06 Plan 04: Missing Track Hardening Summary

**Primary-track protection with secondary-track skip warnings for audio and subtitles**

## Performance

- **Duration:** 8 min
- **Started:** 2026-07-10T18:21:04Z
- **Completed:** 2026-07-10T18:29:17Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments

- Missing primary audio and subtitle locales now return hard errors that name the primary locale.
- Missing secondary audio and subtitle locales now emit `output.Global.Warn`, skip the track, and allow the episode to continue.
- Successful partial episodes now emit a separate warning after the green success line listing skipped tracks.
- `episode_test.go` covers primary errors, secondary skips, partial warnings, and the empty-audio guard.

## Task Commits

1. **Task 1: Audio locale loop hardening** - `b51b6f6` (`feat`)
2. **Task 2: Subtitle loop and partial warning** - `8db7bad` (`feat`)
3. **Task 3: Missing-track tests and deterministic seams** - `2d8f767` (`test`)

**Plan metadata:** skipped (`commit_docs` disabled)

## Files Created/Modified

- `internal/download/episode.go` - Implements primary/secondary missing-track behavior, partial warning, empty-audio guard, and unexported seams for deterministic tests.
- `internal/download/episode_test.go` - Adds behavioral tests for primary audio errors, secondary audio skips, partial subtitle warnings, and empty audio guard.
- `.planning/phases/06-track-hardening-structured-logging/06-04-SUMMARY.md` - Execution summary.

## Decisions Made

- Reused `output.Global.Warn` for all skip and partial notifications so human and NDJSON output continue through the established warning path.
- Added unexported function seams in `download` instead of new fake client types, keeping runtime behavior unchanged while making success-path tests deterministic.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added deterministic Episode seams for success-path tests**
- **Found during:** Task 3 (episode_test.go missing-track cases)
- **Issue:** `download.Episode` directly called HTTP, DRM, media download, and mux boundaries, so successful secondary-skip and partial-episode tests would require real Widevine/FFmpeg/network work.
- **Fix:** Added unexported package-level function variables defaulting to the production functions, then used them from tests with `t.Cleanup` restoration.
- **Files modified:** `internal/download/episode.go`, `internal/download/episode_test.go`
- **Verification:** `go test ./internal/download/... -count=1` passes.
- **Committed in:** `2d8f767`

---

**Total deviations:** 1 auto-fixed (1 blocking).
**Impact on plan:** Testability seam only; production behavior remains delegated to the same API/media/DRM/mux functions by default.

## Issues Encountered

- Go commands needed escalation because the sandbox blocked writes to the Go build cache under `~/.cache/go-build`.
- The TDD-labelled test task was executed after production tasks as the plan ordered; no RED commit was possible because the implementation was already committed by Tasks 1 and 2.

## User Setup Required

None - no external service configuration required.

## Verification

- `go build ./internal/download/...` - pass
- `go test ./internal/download/... -count=1` - pass (38 tests)
- `go vet ./internal/download/...` - pass
- `git diff --name-only -- internal/output/ndjson.go` - empty, confirming the NDJSON contract was not modified

## Known Stubs

None.

## Threat Flags

None.

## Self-Check: PASSED

- Summary file created at `.planning/phases/06-track-hardening-structured-logging/06-04-SUMMARY.md`.
- Task commits found: `b51b6f6`, `8db7bad`, `2d8f767`.
- Key modified files exist: `internal/download/episode.go`, `internal/download/episode_test.go`.

## Next Phase Readiness

Plan 06-04 is complete. Phase 6 can continue with the remaining hardening/logging plans.

---
*Phase: 06-track-hardening-structured-logging*
*Completed: 2026-07-10*
