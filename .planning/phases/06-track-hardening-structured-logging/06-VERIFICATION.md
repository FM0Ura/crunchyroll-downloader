---
phase: 06-track-hardening-structured-logging
verified: 2026-07-10T21:29:53Z
status: passed
score: 19/19 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 18/19
  gaps_closed:
    - "Config.AudioLang / Config.SubsLang array fields feed runtime primary-element-first language selection"
  gaps_remaining: []
  regressions: []
---

# Phase 6: Track Hardening + Structured Logging Verification Report

**Phase Goal:** Users get graceful missing-track handling so partial tracks no longer abort downloads, plus a configurable structured diagnostic log with automatic rotation and PII redaction
**Verified:** 2026-07-10T21:29:53Z
**Status:** passed
**Re-verification:** Yes — after gap closure (plan 06-07)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|---|---|---|
| 1 | Non-primary requested audio/subtitle missing warns and skips, not aborts | VERIFIED | `internal/download/episode.go` has secondary audio/subtitle `output.Global.Warn` + `continue`; `internal/download/episode_test.go` covers `Skipping en-US audio` and partial subtitle warning. Regression check: download package tests still pass. |
| 2 | All/no audio tracks hard-error before empty MKV | VERIFIED | `internal/download/episode.go` returns `no audio tracks available`; `TestEpisodeReturnsErrorWhenAudioLangsEmptyBeforeMux` covers the guard. |
| 3 | Muxer rejects empty/missing inputs before FFmpeg | VERIFIED | `internal/mux/mux.go` runs `os.Stat` before args and `ffmpegCommand`; mux tests cover empty video, empty audio, and missing input with sentinel FFmpeg not invoked. |
| 4 | `--log-level` / `--log-file` configure structured diagnostic logs with subsystem strategic events | VERIFIED | `main.go` declares flags, resolves env/config/default precedence, calls `diag.Init` before `checkFFmpeg`; strategic events exist in download, mux, api with nil guards. |
| 5 | Log file rotates and redacts PII keys | VERIFIED | `internal/diag/diag.go` uses lumberjack MaxSize 10, MaxBackups 5; `redactAttr` redacts token/cookie/etp_rt/client_id/private_key/authorization; end-to-end redaction test passes. |
| 6 | Config accepts language arrays and legacy single strings | VERIFIED | `Config.UnmarshalJSON` accepts arrays and legacy strings; config tests cover arrays, legacy strings, explicit empty slices. |
| 7 | Config log fields exist and merge/write correctly | VERIFIED | `LogLevel`/`LogFile` fields, Merge branches, and `WriteSkeleton` `log_level: info` verified in code and tests. |
| 8 | Diagnostic logger core initializes no-op default, rotation, groups, level parsing | VERIFIED | `diag.Global` defaults to discard handler; `Init` creates five group loggers; `ParseLevel` and Init tests pass. |
| 9 | Main startup ordering initializes diagnostics after config and before FFmpeg preflight | VERIFIED | `main.go`: `config.Load` then `resolvedLogLevel/resolvedLogFile`, `diag.Init`, then `checkFFmpeg`. |
| 10 | Primary audio/subtitle missing hard-errors | VERIFIED | Audio and subtitle primary branches present; primary audio has tests. |
| 11 | Subtitle download failure remains hard error | VERIFIED | `downloading subtitles for %s: %w` preserved in `internal/download/episode.go`. |
| 12 | Partial episode emits separate warn after success | VERIFIED | `downloaded partially` warning exists after success line; test asserts partial marker. |
| 13 | NDJSON warn contract unchanged for skips | VERIFIED | Skip paths use `output.Global.Warn`; `internal/output/ndjson.go` was not modified by Phase 06. |
| 14 | Strategic slog producer calls avoid raw secret attrs | VERIFIED | `internal/api/client.go` logs token refresh with status only; grep found no token/authorization/cookie/etp_rt/client_id/private_key attrs in modified producers. |
| 15 | Config.AudioLang / Config.SubsLang array fields feed runtime primary-element-first language selection | VERIFIED | **Previously FAILED — closed by plan 06-07.** `main.go` now defines `resolveLangs(explicitFlags, flagName, flagVal, configVal, defaultVal)` implementing explicit-CLI-flag > config-array > default precedence (lines 50-66). Runtime construction at lines 346-347 calls `resolveLangs(explicitFlags, "audio-lang", *audioLang, cfg.AudioLang, []string{"ja-JP"})` and `resolveLangs(explicitFlags, "subs-lang", *subtitlesLang, cfg.SubsLang, []string{"en-US"})`. `TestResolveLangs` (10 table cases) passes: `go test ./ -run TestResolveLangs -count=1` → ok crunchyroll-downloader 0.003s. |

**Score:** 19/19 truths verified (0 present, behavior-unverified)

### Deferred Items

None — no items deferred to later milestone phases.

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/config/config.go` | Config arrays, log fields, Merge, skeleton | VERIFIED | Substantive implementation present and tested. Run consumed via `cfg.AudioLang`/`cfg.SubsLang`. |
| `internal/diag/diag.go` | Global, Init, ParseLevel, subsystem loggers, lumberjack | VERIFIED | Uses TextHandler + lumberjack and initializes five groups. Package tests pass. |
| `internal/diag/redact.go` | Handler wrapper and key redaction | VERIFIED | ReplaceAttr redacts exactly six keys; `device_id` excluded. |
| `main.go` | Log flags, precedence, diag.Init ordering, language resolver | VERIFIED | Logging wiring verified; `resolveLangs` now consumes `cfg.AudioLang`/`cfg.SubsLang` at runtime call site (lines 346-347). |
| `internal/download/episode.go` | Track hardening and strategic events | VERIFIED | Primary/secondary split, partial warning, episode events present. |
| `internal/download/season.go` | Season failure event | VERIFIED | Emits `season_failure` before returning `SeasonError`. |
| `internal/mux/mux.go` | Input validation and ffmpeg event | VERIFIED | Validation before args/FFmpeg; success path logs `ffmpeg finished`. |
| `internal/api/client.go` | Token refresh event | VERIFIED | Logs `token refreshed` after successful refresh without secret attrs. |
| `main_test.go` | TestResolveLangs precedence/primary/order | VERIFIED | 10 table cases cover flag/config/default precedence, order preservation, nil/empty config, ERR-03 empty path. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `Config.LogLevel` / `Config.LogFile` | `main.go` | `resolveString(... cfg.LogLevel/cfg.LogFile ...)` | WIRED | Verified in source and `TestResolveString`. |
| `diag.Init` | subsystem producers | package globals `diag.DownloadLogger`, `diag.MuxLogger`, `diag.ApiLogger` | WIRED | Producers nil-check and call initialized globals. |
| `output.Global.Warn` skip paths | NDJSON warn event | existing output implementation | WIRED | No new NDJSON event shape added. |
| `MergeEverything` validation | FFmpeg invocation | validation before `args :=` and before `ffmpegCommand` | WIRED | Sentinel tests prove FFmpeg not invoked on invalid input. |
| `Config.AudioLang` / `Config.SubsLang` | runtime language selection | `resolveLangs(...)` -> `audioLangs`/`subsLangs` -> `processURL`/`download.Episode` | WIRED | **Previously NOT_WIRED — now closed.** `main.go` lines 346-347 pass `cfg.AudioLang`/`cfg.SubsLang` through `resolveLangs`; first element of returned slice is the protected primary track. |

### Data-Flow Trace

| Artifact | Data Variable | Source | Produces Real Data | Status |
|---|---|---|---|---|
| `main.go` | `resolvedLogLevel`, `resolvedLogFile` | CLI/env/config/default via `resolveString` | Yes | FLOWING |
| `main.go` | `audioLangs`, `subsLangs` | `resolveLangs(explicitFlags, "audio-lang"/"subs-lang", flagVal, cfg.AudioLang/cfg.SubsLang, default)` | Yes | FLOWING — config arrays now flow when CLI flag is not explicit. |
| `internal/download/episode.go` | `skippedTracks` | missing secondary branches | Yes | FLOWING |
| `internal/diag/redact.go` | `redactKeys` | handler `ReplaceAttr` | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| Config-backed language resolver | `go test ./ -run TestResolveLangs -count=1` | ok crunchyroll-downloader 0.003s (10/10 subtests) | PASS |
| Config/diag/download/mux regression | `go test ./internal/config ./internal/diag ./internal/download ./internal/mux -count=1` | ok in all 4 packages | PASS |

### Probe Execution

No `scripts/.../probe-*.sh` files were present. Step 7c skipped.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ---------- | ----------- | ------ | -------- |
| ERR-01 | 06-04, 06-07 | Non-primary audio missing warns/skips, primary audio protected | SATISFIED | Skip code/tests in download; resolver preserves audio language order via `resolveLangs`. REQUIREMENTS.md checkbox `[x]`, traceability `Complete`. |
| ERR-02 | 06-04, 06-07 | Non-primary subtitle missing warns/skips, primary subtitle protected | SATISFIED | Skip code/tests in download; resolver preserves subtitle language order via `resolveLangs`. REQUIREMENTS.md checkbox `[x]`, traceability `Complete`. |
| ERR-03 | 06-04 | Hard error when ALL audio tracks missing (no silent empty MKV) | SATISFIED | Empty-audio guard and test; `TestResolveLangs` explicit-empty case preserves the empty-slice path. REQUIREMENTS.md checkbox `[x]`, traceability `Complete`. |
| ERR-04 | 06-05, 06-07 | Muxer rejects empty/mismatched inputs before FFmpeg | SATISFIED | Mux validation and tests pass. **Stale metadata corrected by 06-07 Task 3:** REQUIREMENTS.md line 15 checkbox now `- [x]`, line 103 traceability row now `Complete`. |
| LOG-01 | 06-01 | `--log-level` / `--log-file` configurable structured logs | SATISFIED | Flags, precedence, diag.Init, tests. REQUIREMENTS.md checkbox `[x]`, traceability `Complete`. |
| LOG-02 | 06-02 | Per-subsystem scoped slog groups | SATISFIED | Five grouped loggers; strategic producer calls use grouped subsystem loggers. REQUIREMENTS.md checkbox `[x]`, traceability `Complete`. |
| LOG-03 | 06-03 | Lumberjack rotation (5-10MB max, 5 backups) | SATISFIED | lumberjack direct dependency and rotation constants. REQUIREMENTS.md checkbox `[x]`, traceability `Complete`. |
| LOG-04 | 06-02 | PII redaction via ReplaceAttr wrapper | SATISFIED | ReplaceAttr redaction and end-to-end redaction test. REQUIREMENTS.md checkbox `[x]`, traceability `Complete`. |
| LOG-05 | 06-06 | Strategic events only at default log level | SATISFIED | Info-level events for episode start/finish, FFmpeg summary, token refresh, season failure; no per-segment diag Info calls. REQUIREMENTS.md checkbox `[x]`, traceability `Complete`. |

**Orphaned requirements:** None. All 9 phase requirement IDs (ERR-01..04, LOG-01..05) are declared across plans 06-01..06-07 and accounted for above.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |

No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers found in `main.go` or `main_test.go`. No blocker or warning anti-patterns in the files modified by plan 06-07.

### Human Verification Required

None. The phase is fully closable on automated checks.

### Gaps Summary

No gaps remain. The single previously-failed truth (`Config.AudioLang`/`Config.SubsLang` feed runtime selection) was closed by plan 06-07:
- `resolveLangs` helper added to `main.go` with explicit-flag > config-array > default precedence (lines 50-66).
- Runtime `audioLangs`/`subsLangs` construction rewired to consume `cfg.AudioLang`/`cfg.SubsLang` (lines 346-347).
- `TestResolveLangs` (10 table-driven subtests) passes, covering flag/config/default precedence, multi-element order preservation (D-01/D-02 primary-element-first), nil-config default fallback, and explicit-empty paths feeding the ERR-03 hard-error guard.
- ERR-04 stale-metadata warning closed: `REQUIREMENTS.md` checkbox and traceability row now read `Complete`.

All 9 phase requirements (ERR-01..04, LOG-01..05) are marked Complete in both the checkbox list and traceability table, and the underlying code/tests continue to pass.

---

_Verified: 2026-07-10T21:29:53Z_
_Verifier: the agent (gsd-verifier)_