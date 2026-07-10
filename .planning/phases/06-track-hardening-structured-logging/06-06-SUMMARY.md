---
phase: 06-track-hardening-structured-logging
plan: 06
subsystem: logging
tags: [go, slog, diagnostics, download, mux, api]
requires:
  - phase: 06-track-hardening-structured-logging
    provides: "Grouped diag subsystem loggers, missing-track hardening, and mux input validation"
provides:
  - "Strategic download events for episode start, episode finish, skipped tracks, and season failure"
  - "Strategic mux event for successful ffmpeg completion"
  - "Strategic API event for recovered 401 token refresh"
affects: [download, mux, api, diagnostic-logging]
tech-stack:
  added: []
  patterns:
    - "Subsystem diagnostic producer calls are nil-check guarded"
    - "Producer events attach status/context attrs but avoid secret values"
key-files:
  created:
    - .planning/phases/06-track-hardening-structured-logging/06-06-SUMMARY.md
  modified:
    - internal/download/episode.go
    - internal/download/season.go
    - internal/mux/mux.go
    - internal/api/client.go
key-decisions:
  - "Kept all diagnostic events additive beside existing output.Global messages so the user-facing output plane remains unchanged."
  - "Logged token refresh as a status-only recovery event and did not attach token, authorization, cookie, or etp_rt values."
patterns-established:
  - "D-18 strategic events use Info except skipped-track diagnostics, which use Warn beside the user-facing skip warning."
requirements-completed: [LOG-02, LOG-04, LOG-05]
coverage:
  - id: D1
    description: "Download flow emits episode_start, episode_finish, skipped track, and season_failure events through diag.DownloadLogger."
    requirement: LOG-02
    verification:
      - kind: other
        ref: "go build ./internal/download/... && go test ./internal/download/... && go vet ./internal/download/..."
        status: pass
      - kind: other
        ref: "rg probes for episode_start, episode_finish, skipped track, season_failure, and nil guards"
        status: pass
    human_judgment: false
  - id: D2
    description: "Mux flow emits ffmpeg finished after successful cmd.Run and before temporary-file cleanup."
    requirement: LOG-05
    verification:
      - kind: other
        ref: "go build ./internal/mux/... && go test ./internal/mux/... && go vet ./internal/mux/..."
        status: pass
      - kind: other
        ref: "source-order probe: cmd.Run line 115, ffmpeg finished line 123, warnRemove line 126"
        status: pass
    human_judgment: false
  - id: D3
    description: "API 401 refresh recovery emits token refreshed without logging raw token or authorization values."
    requirement: LOG-04
    verification:
      - kind: other
        ref: "go build ./internal/api/... && go test ./internal/api/... && go vet ./internal/api/..."
        status: pass
      - kind: other
        ref: "rg probes for token refreshed, nil guard, no token/authorization attr, and auth.go untouched"
        status: pass
    human_judgment: false
duration: 22min
completed: 2026-07-10
status: complete
---

# Phase 06 Plan 06: Strategic Diagnostic Events Summary

**Strategic slog events now mark episode, mux, season, and token-refresh milestones through grouped subsystem loggers**

## Performance

- **Duration:** 22 min
- **Started:** 2026-07-10T18:32:30Z
- **Completed:** 2026-07-10T18:54:35Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments

- Added nil-guarded `diag.DownloadLogger` events for `episode_start`, `episode_finish`, skipped audio/subtitle tracks, and season failures.
- Added nil-guarded `diag.MuxLogger.Info("ffmpeg finished", "stderr", stderr.String())` on the successful ffmpeg path before cleanup.
- Added nil-guarded `diag.ApiLogger.Info("token refreshed", "status", "401-recovered")` after successful 401 token refresh without attaching secret values.

## Task Commits

1. **Task 1: download package strategic events** - `7dbf3e6` (feat)
2. **Task 2: mux package ffmpeg-finished Info event** - `192f7f4` (feat)
3. **Task 3: api package token-refresh strategic Info event** - `ea8aa4b` (feat)

**Plan metadata:** skipped (`commit_docs` disabled in `.planning/config.json`)

## Files Created/Modified

- `internal/download/episode.go` - Added download diagnostic events for episode start/finish and skipped audio/subtitle tracks.
- `internal/download/season.go` - Added season failure diagnostic event before returning `SeasonError`.
- `internal/mux/mux.go` - Added ffmpeg success diagnostic event after `cmd.Run()` returns nil.
- `internal/api/client.go` - Added token-refresh recovery diagnostic event after `c.fetchAccessToken` succeeds.
- `.planning/phases/06-track-hardening-structured-logging/06-06-SUMMARY.md` - Execution summary and verification evidence.

## Decisions Made

- Kept all slog producer calls nil-check guarded because subsystem logger globals are populated by `diag.Init`.
- Reused existing `output.Global` lines exactly and added diagnostic events beside them, preserving D-13 output-plane separation.
- Avoided producer-side secret attrs entirely in the API event; redaction remains a second line of defense from 06-02.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- The managed sandbox blocked Go build-cache access during initial download-package build/test, so the same Go commands were rerun outside the sandbox and passed.
- The managed sandbox blocked `httptest` from binding a localhost port in API tests, so `go test ./internal/api/...` was rerun outside the sandbox and passed.
- The managed sandbox keeps `.git` read-only, so scoped task commits were made outside the sandbox with normal hooks.

## Verification

- `go build ./internal/download/...` - pass
- `go test ./internal/download/...` - pass (38 tests)
- `go vet ./internal/download/...` - pass
- `go build ./internal/mux/...` - pass
- `go test ./internal/mux/...` - pass (12 tests)
- `go vet ./internal/mux/...` - pass
- `go build ./internal/api/...` - pass
- `go test ./internal/api/...` - pass (12 tests; rerun outside sandbox due localhost bind restriction)
- `go vet ./internal/api/...` - pass
- `go build ./...` - pass
- `go test ./internal/download/... ./internal/mux/... ./internal/api/...` - pass (62 tests)
- `go vet ./...` - pass
- Source probes confirmed 7 subsystem logger call sites and 7 matching nil guards.
- Source probes confirmed no token, authorization, cookie, etp_rt, client_id, or private_key attr is logged in modified files.

## Known Stubs

None. Stub scan findings were ordinary empty/nil guard branches and existing user-facing "not available" messages, not incomplete implementation placeholders.

## Threat Flags

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Phase 06 instrumentation is complete. The track-hardening behavior from 06-04/06-05 now has grouped strategic diagnostic events while preserving the existing user-facing output plane.

## Self-Check: PASSED

- Summary file created at `.planning/phases/06-track-hardening-structured-logging/06-06-SUMMARY.md`.
- Task commits found in `git log`: `7dbf3e6`, `192f7f4`, `ea8aa4b`.
- Key modified files exist: `internal/download/episode.go`, `internal/download/season.go`, `internal/mux/mux.go`, `internal/api/client.go`.
- No tracked file deletions were introduced by task commits.

---
*Phase: 06-track-hardening-structured-logging*
*Completed: 2026-07-10*
