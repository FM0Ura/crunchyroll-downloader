---
phase: 06-track-hardening-structured-logging
plan: 02
subsystem: logging
tags: [go, slog, lumberjack, redaction, diagnostics]
requires:
  - phase: 06-track-hardening-structured-logging
    provides: config log fields and phase logging decisions
provides:
  - Diagnostic slog singleton with lumberjack rotation
  - Five grouped subsystem diagnostic loggers
  - Six-key PII redaction seam via slog ReplaceAttr
  - ParseLevel helper for log-level wiring
affects: [06-track-hardening-structured-logging, diagnostic-logging, api, download, mux, media, drm]
tech-stack:
  added:
    - gopkg.in/natefinch/lumberjack.v2 v2.2.1
  patterns:
    - no-op package default for diagnostic singleton before Init
    - slog TextHandler wrapped by handlerWrapper with ReplaceAttr redaction
key-files:
  created:
    - internal/diag/diag.go
    - internal/diag/redact.go
    - internal/diag/diag_test.go
    - .planning/phases/06-track-hardening-structured-logging/06-02-SUMMARY.md
  modified:
    - go.mod
    - go.sum
key-decisions:
  - "Used a tiny discardHandler for the pre-Init/fallback no-op logger so only the production path uses slog.TextHandler."
  - "Kept redaction centralized in redactAttr on HandlerOptions.ReplaceAttr; handlerWrapper delegates and preserves the handler seam."
patterns-established:
  - "Diagnostic subsystem loggers are exported package globals populated by diag.Init from Global.WithGroup."
  - "PII redaction is key-based at the handler layer; producers should log structured attrs without local redaction."
requirements-completed: [LOG-01, LOG-02, LOG-03, LOG-04, LOG-05]
coverage:
  - id: D1
    description: "diag.Init wires a slog TextHandler through lumberjack rotation with the D-15 constants and default log path behavior."
    requirement: LOG-01
    verification:
      - kind: unit
        ref: "go test ./internal/diag/... -count=1 -v"
        status: pass
      - kind: other
        ref: "go build ./..."
        status: pass
    human_judgment: false
  - id: D2
    description: "Download, DRM, media, mux, and API subsystem loggers are grouped children of diag.Global."
    requirement: LOG-02
    verification:
      - kind: unit
        ref: "internal/diag/diag_test.go#TestInitWiresSubsystemLoggers"
        status: pass
    human_judgment: false
  - id: D3
    description: "Diagnostic logs rotate through lumberjack.v2 pinned as a direct Go module dependency."
    requirement: LOG-03
    verification:
      - kind: other
        ref: "rg -n 'gopkg.in/natefinch/lumberjack.v2' go.mod"
        status: pass
    human_judgment: false
  - id: D4
    description: "token, cookie, etp_rt, client_id, private_key, and authorization attrs are redacted while device_id is not."
    requirement: LOG-04
    verification:
      - kind: unit
        ref: "internal/diag/diag_test.go#TestRedactKeys"
        status: pass
      - kind: unit
        ref: "internal/diag/diag_test.go#TestRedactionEndToEnd"
        status: pass
    human_judgment: false
  - id: D5
    description: "ParseLevel maps debug/info/warn/error case-insensitively and defaults unknown values to info."
    requirement: LOG-05
    verification:
      - kind: unit
        ref: "internal/diag/diag_test.go#TestParseLevel"
        status: pass
    human_judgment: false
duration: 15min
completed: 2026-07-10
status: complete
---

# Phase 06 Plan 02: Diagnostic Logging Core Summary

**slog diagnostic logger with lumberjack rotation, subsystem grouping, and handler-layer PII redaction**

## Performance

- **Duration:** 15 min
- **Started:** 2026-07-10T18:02:40Z
- **Completed:** 2026-07-10T18:17:00Z
- **Tasks:** 2 completed
- **Files modified:** 6

## Accomplishments

- Added `internal/diag` with a no-op default `Global`, `Init`, `ParseLevel`, and five exported subsystem loggers.
- Wired diagnostic output through `slog.TextHandler` and `lumberjack.Logger` using MaxSize 10 MB, MaxBackups 5, MaxAge 0, and no compression.
- Added handler-layer redaction for exactly the six D-21 keys while preserving `device_id` and URL/content-id visibility.
- Added TDD coverage for level parsing, key membership, Init logger wiring, and end-to-end redaction from log file output.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add lumberjack.v2 dep + create diag.go** - `0870630` (feat)
2. **Task 2 RED: Add failing diag redaction tests** - `40f7f1d` (test)
3. **Task 2 GREEN: Implement redact.go** - `41169cc` (feat)

**Plan metadata:** pending summary commit

## Files Created/Modified

- `internal/diag/diag.go` - Diagnostic logger singleton, Init wiring, rotation config, ParseLevel, and subsystem logger globals.
- `internal/diag/redact.go` - Six-key redaction set, ReplaceAttr callback, and slog handler wrapper delegation.
- `internal/diag/diag_test.go` - Table-driven tests and end-to-end file redaction coverage.
- `go.mod` - Added direct `gopkg.in/natefinch/lumberjack.v2 v2.2.1` requirement.
- `go.sum` - Added lumberjack module checksums.
- `.planning/phases/06-track-hardening-structured-logging/06-02-SUMMARY.md` - Execution summary.

## Decisions Made

- Used `discardHandler` for the default and MkdirAll-failure fallback so diagnostic logging never aborts the app and the production path remains the only `slog.TextHandler` construction.
- Kept the wrapper handler as a structural seam; redaction itself happens in `redactAttr` through `HandlerOptions.ReplaceAttr`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added explicit no-op default handler**
- **Found during:** Task 1 (`diag.go`)
- **Issue:** The plan's must-have truth required `diag.Global` to be initialized to a no-op default before `Init`, while the task prose also referenced nil-before-Init behavior and fallback via another `TextHandler`.
- **Fix:** Initialized `Global` with a small `discardHandler` and reused it for MkdirAll fallback, preserving non-aborting diagnostic behavior and keeping only one production `TextHandler`.
- **Files modified:** `internal/diag/diag.go`
- **Verification:** `go build ./...`, `go test ./internal/diag/... -count=1 -v`, and Task 1 `rg` probes passed.
- **Committed in:** `0870630`

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** The change aligned the implementation with the phase must-have no-op default and did not add user-visible scope.

## Issues Encountered

- Sandboxed network access blocked `go get`; reran the same Go module command with approval to fetch `lumberjack.v2`.
- Sandboxed Go test/build initially could not write to the Go build cache; verification was rerun with approval and passed.

## Verification

- `go build ./...` - passed
- `go test ./internal/diag/... -count=1 -v` - passed
- `go vet ./internal/diag/...` - passed
- `rg -n 'gopkg.in/natefinch/lumberjack.v2' go.mod` - passed
- Redaction test writes `token=supersecret`, asserts `[REDACTED]` appears, `device_id=uuid-123` remains, and `supersecret` is absent.

## Known Stubs

None.

## Threat Flags

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Plan 06-03 can now wire `diag.Init(diag.ParseLevel(...), ...)` from main-level log config precedence. Later instrumentation plans can use the exported subsystem loggers without adding producer-side redaction.

## Self-Check: PASSED

- Summary file created at `.planning/phases/06-track-hardening-structured-logging/06-02-SUMMARY.md`.
- Task commits exist: `0870630`, `40f7f1d`, `41169cc`.
- Key created files exist: `internal/diag/diag.go`, `internal/diag/redact.go`, `internal/diag/diag_test.go`.

---
*Phase: 06-track-hardening-structured-logging*
*Completed: 2026-07-10*
