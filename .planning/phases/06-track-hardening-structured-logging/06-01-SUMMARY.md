---
phase: 06-track-hardening-structured-logging
plan: 01
subsystem: config
tags: [go, config, json, logging, tests]
requires:
  - phase: 06-track-hardening-structured-logging
    provides: phase context and config migration requirements
provides:
  - Config language fields migrated to []string with legacy string compatibility
  - Config LogLevel and LogFile pointer fields for diagnostic logging precedence
  - WriteSkeleton array language defaults and log_level default
  - Table-driven tests for D-02 and D-19 config behavior
affects: [06-track-hardening-structured-logging, config, diagnostic-logging, track-selection]
tech-stack:
  added: []
  patterns:
    - custom JSON unmarshal for backward-compatible schema migration
    - nil slice as absent and empty slice as explicit empty config override
key-files:
  created:
    - .planning/phases/06-track-hardening-structured-logging/06-01-SUMMARY.md
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - main_test.go
key-decisions:
  - "Used Config.UnmarshalJSON with json.RawMessage to accept both array and legacy string language fields."
  - "Kept main.go out of scope per plan; updated only main_test.go helper assignments required by the new slice field type."
patterns-established:
  - "Language config fields use nil slice for absent, non-nil empty slice for explicit empty override."
  - "Legacy single-string language config emits a stderr migration warning and loads as a one-element slice."
requirements-completed: [LOG-01, ERR-01, ERR-02, ERR-03]
coverage:
  - id: D1
    description: "Config.AudioLang and Config.SubsLang are []string fields accepting array and legacy string JSON forms."
    requirement: ERR-01
    verification:
      - kind: unit
        ref: "go test ./internal/config/... -count=1"
        status: pass
      - kind: other
        ref: "rg field-shape acceptance probes for AudioLang/SubsLang"
        status: pass
    human_judgment: false
  - id: D2
    description: "Config.LogLevel and Config.LogFile pointer fields merge by explicit overlay semantics."
    requirement: LOG-01
    verification:
      - kind: unit
        ref: "go test ./internal/config/... -count=1"
        status: pass
      - kind: other
        ref: "rg field-shape and Merge branch acceptance probes for LogLevel/LogFile"
        status: pass
    human_judgment: false
  - id: D3
    description: "WriteSkeleton emits language arrays and the log_level info default."
    requirement: LOG-01
    verification:
      - kind: unit
        ref: "go test ./internal/config/... -count=1"
        status: pass
      - kind: other
        ref: "rg '\"log_level\": \"info\"' internal/config/config.go"
        status: pass
    human_judgment: false
duration: 17min
completed: 2026-07-10
status: complete
---

# Phase 06 Plan 01: Config Schema Migration Summary

**Config language arrays with legacy string migration and diagnostic log override fields**

## Performance

- **Duration:** 17 min
- **Started:** 2026-07-10T17:45:19Z
- **Completed:** 2026-07-10T18:02:38Z
- **Tasks:** 2 completed
- **Files modified:** 3

## Accomplishments

- Migrated `audio_lang` and `subs_lang` config fields to `[]string` with nil-vs-empty slice semantics.
- Added backward-compatible JSON loading for legacy single-string language values with migration warnings.
- Added `LogLevel` and `LogFile` pointer config fields plus Merge overlay handling.
- Updated skeleton config output to write language arrays and `"log_level": "info"`.
- Added config tests covering array load, legacy string load, explicit empty slices, log field merge, and skeleton output.

## Task Commits

1. **Task 1: Migrate Config struct and Merge/WriteSkeleton** - `46df2a3` (feat)
2. **Task 2: Extend config_test.go coverage** - `1333fad` (test)

**Plan metadata:** pending summary commit

## Files Created/Modified

- `internal/config/config.go` - Migrated language fields, added log config fields, custom unmarshal, Merge branches, and skeleton defaults.
- `internal/config/config_test.go` - Added table-driven coverage for D-02 array/legacy behavior and D-19 log config behavior.
- `main_test.go` - Updated `isAllNilConfig` test helpers to assign language slices instead of string pointers.
- `.planning/phases/06-track-hardening-structured-logging/06-01-SUMMARY.md` - Execution summary.

## Decisions Made

- Used `Config.UnmarshalJSON` with `json.RawMessage` to keep the public `Config` field types as plain `[]string` while accepting legacy string JSON.
- Kept production `main.go` unchanged per plan scope. The only root-level change was a test helper update required for the new `Config` type.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated root test helper for new Config language field types**
- **Found during:** Task 2 (config tests)
- **Issue:** `main_test.go` still assigned `*string` values to `Config.AudioLang` and `Config.SubsLang`, which would fail repository test compilation after the schema migration.
- **Fix:** Updated the `TestIsAllNilConfig` language setters to assign `[]string{"x"}`.
- **Files modified:** `main_test.go`
- **Verification:** `go test ./... -count=1` passed.
- **Committed in:** `1333fad`

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Scope remained limited to compile compatibility caused directly by the config schema migration.

## Issues Encountered

- Initial sandboxed Go test run could not access the Go build cache. Verification was rerun with approval using the same commands and passed.
- Existing planning files were dirty before execution; they were not staged or committed with this plan.

## Verification

- `go build ./...` - passed
- `go test ./internal/config/... -run 'TestLoad|TestMerge|TestWriteSkeleton' -count=1 -v` - passed
- `go test ./internal/config/... -count=1` - passed
- `go test ./... -count=1` - passed, 273 tests across 10 packages
- `go vet ./internal/config/...` - passed
- Field-shape and acceptance `rg` probes passed for `AudioLang`, `SubsLang`, `LogLevel`, `LogFile`, Merge branches, skeleton default, array-load tests, log tests, and slice assertions.

## Known Stubs

None.

## Threat Flags

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Plan 06-02 can build on the config foundation for diagnostic logger initialization and redaction. `Config.LogLevel` and `Config.LogFile` are available for the main-level precedence wiring planned in 06-03.

## Self-Check: PASSED

- Summary file created at `.planning/phases/06-track-hardening-structured-logging/06-01-SUMMARY.md`.
- Task commits exist: `46df2a3`, `1333fad`.
- Key modified files exist: `internal/config/config.go`, `internal/config/config_test.go`, `main_test.go`.

---
*Phase: 06-track-hardening-structured-logging*
*Completed: 2026-07-10*
