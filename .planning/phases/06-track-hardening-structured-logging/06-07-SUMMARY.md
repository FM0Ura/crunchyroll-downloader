---
phase: 06-track-hardening-structured-logging
plan: 07
subsystem: config
tags: [go, config, language-selection, tdd, gap-closure]

# Dependency graph
requires:
  - phase: 06-track-hardening-structured-logging (plans 06-01, 06-03, 06-04, 06-05, 06-06)
    provides: config D-02 array schema, missing-track skip handling, mux validation, diag logging
provides:
  - resolveLangs helper wiring Config.AudioLang/Config.SubsLang into runtime download selection with flag/config/default precedence
  - Table-driven TestResolveLangs proving D-01/D-02 primary-element-first ordering and ERR-03 empty-list hard-error paths
  - ERR-04 requirement metadata corrected to Complete in REQUIREMENTS.md
affects: [future-series-downloads, config-driven-track-selection, verifier-phase-06-closure]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Language-list resolver with explicit-flag > config-array > default precedence (mirrors resolveString pattern for slice fields)"

key-files:
  created: []
  modified:
    - main.go
    - main_test.go
    - .planning/REQUIREMENTS.md

key-decisions:
  - "resolveLangs consults no environment variables — no CRUNCHYROLL_AUDIO_LANG/CRUNCHYROLL_SUBS_LANG decision exists in Phase 06, unlike the scalar resolveString path."
  - "Nil config slice => default fallback; explicit empty config array []string{} => empty slice flows to existing ERR-03 hard-error guard. The nil-vs-empty distinction is preserved and tested."

patterns-established:
  - "Slice-field precedence resolver (resolveLangs) parallels the scalar resolveString resolver, enabling config array consumption without CLI-flag-only behavior."

requirements-completed: [ERR-01, ERR-02, ERR-04]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "Config audio_lang/subs_lang arrays feed runtime language selection when CLI flags are not explicit (closes Phase 06 verifier failed truth)"
    requirement: ERR-01
    verification:
      - kind: unit
        ref: "main_test.go#TestResolveLangs (audio non-explicit uses config array in order)"
        status: pass
      - kind: unit
        ref: "rtk go test ./ -run TestResolveLangs -count=1"
        status: pass
    human_judgment: false
  - id: D2
    description: "Config subtitle array feeds runtime selection preserving first-element-primary order (D-01)"
    requirement: ERR-02
    verification:
      - kind: unit
        ref: "main_test.go#TestResolveLangs (subs non-explicit uses config array, pt-BR primary)"
        status: pass
      - kind: unit
        ref: "rtk go test ./internal/download -count=1"
        status: pass
    human_judgment: false
  - id: D3
    description: "Explicit CLI language flags override config arrays; comma-separated flag parsing stays backward compatible"
    verification:
      - kind: unit
        ref: "main_test.go#TestResolveLangs (audio/subs explicit flag wins over config)"
        status: pass
      - kind: unit
        ref: "main_test.go#TestParseLangs"
        status: pass
    human_judgment: false
  - id: D4
    description: "ERR-04 requirement metadata marked Complete in REQUIREMENTS.md (checkbox + traceability), reflecting already-verified mux validation from 06-05"
    requirement: ERR-04
    verification:
      - kind: unit
        ref: "rtk go test ./internal/mux -count=1"
        status: pass
      - kind: other
        ref: "grep 'ERR-04' .planning/REQUIREMENTS.md shows '- [x]' and 'Complete'"
        status: pass
    human_judgment: false

# Metrics
duration: 3 min
completed: 2026-07-10
status: complete
---

# Phase 06 Plan 07: Config Language Array Wiring Summary

**resolveLangs helper in main.go wires Config.AudioLang/Config.SubsLang into runtime download selection with explicit-flag > config-array > default precedence, closing the Phase 06 verifier failed truth**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-07-10T21:24:25Z
- **Completed:** 2026-07-10T21:26:36Z
- **Tasks:** 3
- **Files modified:** 3 (main.go, main_test.go, .planning/REQUIREMENTS.md)

## Accomplishments
- Added `resolveLangs(explicitFlags, flagName, flagVal, configVal, defaultVal)` in main.go implementing D-01/D-02 precedence: explicit CLI flag > config array > default, preserving slice order so index 0 stays the protected primary track.
- Replaced `parseLangs(*audioLang)` / `parseLangs(*subtitlesLang)` runtime construction with `resolveLangs(...)` calls that pass `cfg.AudioLang` / `cfg.SubsLang`, so config language arrays now flow into `processURL` and `download.Episode`.
- Added table-driven `TestResolveLangs` covering audio/subs flag names, explicit-flag override, config-array order preservation (2- and 3-element), nil-config default fallback, and explicit-empty paths that retain the ERR-03 empty-audio hard-error guard.
- Corrected stale ERR-04 metadata in `.planning/REQUIREMENTS.md`: Error Handling checkbox now `- [x]` and traceability row reads `Complete`, matching the already-verified mux validation from plan 06-05.

## Task Commits

Each task was committed atomically (TDD RED/GREEN for Tasks 1-2):

1. **Task 1: Add tests for config-backed language selection** (RED) - `64746cf` (test)
2. **Task 2: Wire config language arrays into main runtime selection** (GREEN) - `290d62a` (feat)
3. **Task 3: Correct stale ERR-04 requirement metadata** - no commit (commit_docs disabled; `.planning/REQUIREMENTS.md` edit left in working tree per project convention, consistent with prior Phase 06 ERR-01/02/03 complete marks)

_Plan metadata commit: skipped (commit_docs disabled in .planning/config.json)_

## Files Created/Modified
- `main.go` - Added `resolveLangs` helper and rewired runtime `audioLangs`/`subsLangs` construction to consume `cfg.AudioLang`/`cfg.SubsLang`.
- `main_test.go` - Added `TestResolveLangs` with 10 table cases covering flag/config/default precedence and order preservation.
- `.planning/REQUIREMENTS.md` - ERR-04 checkbox marked complete and traceability table status changed from Pending to Complete.

## Decisions Made
- `resolveLangs` consults no environment variables — no `CRUNCHYROLL_AUDIO_LANG`/`CRUNCHYROLL_SUBS_LANG` decision exists in Phase 06, unlike the scalar `resolveString` path. This keeps the language-list resolver minimal and aligned with the Phase 06 decision set.
- Nil config slice (absent from file) falls back to defaults; explicit empty config array (`[]string{}`) flows as an empty slice to the existing ERR-03 `len(audioLangs) == 0` hard-error guard in `processURL`. The nil-vs-empty distinction is preserved and explicitly tested so empty MKV production stays impossible.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Authentication Gates

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- The Phase 06 verifier failed truth (`Config.AudioLang`/`Config.SubsLang` feed runtime selection) is now closed; config language arrays affect downloads without requiring CLI flags.
- ERR-04 metadata warning is closed; all four ERR-* requirements now show Complete in REQUIREMENTS.md.
- Phase 06 is fully closable on re-verification. No completed Phase 06 plans or summaries were rewritten.

## Self-Check: PASSED

- `06-07-SUMMARY.md` exists on disk: FOUND
- RED commit `64746cf` present in git log: FOUND
- GREEN commit `290d62a` present in git log: FOUND
- ERR-04 metadata: 2 matching lines (checkbox `[x]` + traceability `Complete`)

---
*Phase: 06-track-hardening-structured-logging*
*Completed: 2026-07-10*