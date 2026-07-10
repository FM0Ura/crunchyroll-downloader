---
phase: 06-track-hardening-structured-logging
plan: 03
subsystem: logging
tags: [go, cli, config, slog, diagnostics, tests]
requires:
  - phase: 06-track-hardening-structured-logging
    provides: Config.LogLevel/LogFile fields and internal/diag logger core
provides:
  - CLI flags for diagnostic log level and log file
  - Config/env/flag/default precedence wiring for diagnostic logging
  - diag.Init startup ordering after config load and before FFmpeg preflight
  - resolveString tests for log-level and log-file precedence
affects: [06-track-hardening-structured-logging, main, diagnostic-logging]
tech-stack:
  added: []
  patterns:
    - resolveString precedence reused for diagnostic logging configuration
    - diag.ParseLevel remains the canonical log-level parser
key-files:
  created:
    - .planning/phases/06-track-hardening-structured-logging/06-03-SUMMARY.md
  modified:
    - main.go
    - main_test.go
key-decisions:
  - "Initialized diagnostics after config-backed log setting resolution and before checkFFmpeg so preflight failures can be recorded."
  - "Extended the existing TestResolveString instead of adding a duplicate because main_test.go already existed from prior work."
patterns-established:
  - "Diagnostic log settings use explicit flag > CRUNCHYROLL_LOG_* env > config > default precedence."
requirements-completed: [LOG-01]
coverage:
  - id: D1
    description: "--log-level and --log-file are resolved through flag/env/config/default precedence and passed to diag.Init."
    requirement: LOG-01
    verification:
      - kind: unit
        ref: "go test ./ -run TestResolveString -count=1"
        status: pass
      - kind: other
        ref: "go build ./..."
        status: pass
      - kind: other
        ref: "go vet ./..."
        status: pass
    human_judgment: false
  - id: D2
    description: "diag.Init(diag.ParseLevel(...), ...) runs after config.Load and before checkFFmpeg."
    requirement: LOG-01
    verification:
      - kind: other
        ref: "source-order probe: config.Load line 299, diag.Init line 319, checkFFmpeg call line 322"
        status: pass
    human_judgment: false
duration: 4min
completed: 2026-07-10
status: complete
---

# Phase 06 Plan 03: Main Diagnostic Logging Wiring Summary

**Diagnostic logging CLI/config precedence wired into startup before FFmpeg preflight**

## Performance

- **Duration:** 4 min
- **Started:** 2026-07-10T18:40:33Z
- **Completed:** 2026-07-10T18:44:03Z
- **Tasks:** 2 completed
- **Files modified:** 2

## Accomplishments

- Added `--log-level` and `--log-file` flags in `main.go`.
- Resolved diagnostic log settings via `resolveString` with `CRUNCHYROLL_LOG_LEVEL` and `CRUNCHYROLL_LOG_FILE` support.
- Called `diag.Init(diag.ParseLevel(resolvedLogLevel), resolvedLogFile)` after config load and before `checkFFmpeg()`.
- Extended `isAllNilConfig` so log-only config files are not treated as fresh configs.
- Updated `TestResolveString` to cover log-level and log-file precedence with env-var cleanup.

## Task Commits

Each task was committed atomically:

1. **Task 1: Declare log flags and wire diag.Init** - `4bddcef` (feat)
2. **Task 2: Cover diagnostic resolveString precedence** - `ef2d1eb` (test)

**Plan metadata:** skipped (commit_docs disabled in `.planning/config.json`)

## Files Created/Modified

- `main.go` - Added diagnostic log flags, config/env/default resolution, canonical `diag.ParseLevel` use, init ordering, and fresh-config detection for log fields.
- `main_test.go` - Reworked the existing `TestResolveString` table to cover `log-level` and `log-file` precedence with `t.Cleanup` env restoration.
- `.planning/phases/06-track-hardening-structured-logging/06-03-SUMMARY.md` - Execution summary.

## Decisions Made

- Used the existing `resolveString` helper directly; no local `parseLogLevel` wrapper was added.
- Adapted the existing `TestResolveString` rather than adding another test function with the same name.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Adapted existing main_test.go instead of creating a new file**
- **Found during:** Task 2
- **Issue:** `main_test.go` already existed and already contained `TestResolveString`, so creating a new file or duplicate test would not compile cleanly.
- **Fix:** Replaced the existing generic output-dir cases with log-level/log-file cases required by this plan.
- **Files modified:** `main_test.go`
- **Verification:** `go test ./ -run TestResolveString -count=1` passed; `rg -c 'TestResolveString' main_test.go` returned `1`.
- **Committed in:** `ef2d1eb`

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** No scope expansion; the adjustment preserved the single required `TestResolveString` and matched existing test structure.

## Issues Encountered

- Sandboxed Go build/test/vet could not reliably access the Go build cache under the user home directory. Verification was rerun with approval and passed.
- Planning files were already dirty before this execution. They were left untouched except for creating this plan's summary.

## Verification

- `go build ./...` - passed
- `go vet ./...` - passed
- `go test ./ -run TestResolveString -count=1` - passed
- Acceptance probes passed for log flag declarations, env var names, single `diag.Init`, single `diag.ParseLevel`, no `parseLogLevel`, no log-format/JSON handler addition, and diag import.
- Source order verified: `config.Load(cfgPath)` at line 299, `diag.Init(...)` at line 319, `checkFFmpeg()` call at line 322.

## Known Stubs

None.

## Threat Flags

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Diagnostic logging now starts from user/configurable settings before FFmpeg preflight. Downstream instrumentation plans can rely on initialized `diag` subsystem loggers.

## Self-Check: PASSED

- Summary file created at `.planning/phases/06-track-hardening-structured-logging/06-03-SUMMARY.md`.
- Task commits exist: `4bddcef`, `ef2d1eb`.
- Key modified files exist: `main.go`, `main_test.go`.

---
*Phase: 06-track-hardening-structured-logging*
*Completed: 2026-07-10*
