---
gsd_state_version: 1.0
milestone: v1.1
milestone_name: Storage, CLI & Error Handling
current_phase: 07
current_phase_name: Organized Output + Folder Metadata
status: executing
stopped_at: Completed 07-04-PLAN.md
last_updated: "2026-07-11T14:05:57.825Z"
last_activity: 2026-07-11
progress:
  total_phases: 5
  completed_phases: 2
  total_plans: 11
  completed_plans: 11
  percent: 40
---

# Project State

## Current Milestone

🚧 **v1.1 Storage, CLI & Error Handling** — Phases 6-10. Roadmap created 2026-07-10.
Prior v1.0 Improvement & Optimization Pass shipped 2026-07-10 (Phases 1-5, all complete). See `.planning/milestones/` for archives.

## Planning Artifacts

| Document | Status | Description |
|----------|--------|-------------|
| PROJECT.md | ✓ Updated | v1.1 milestone goal + active requirements set |
| ROADMAP.md | ✓ Updated | v1.0 archived; v1.1 Phases 6-10 appended |
| REQUIREMENTS.md | ✓ Updated | 27 v1.1 requirements defined + traceability filled (27/27 mapped) |
| research/SUMMARY.md | ✓ Created | v1.1 research (HIGH confidence), 6-phase structure backing roadmap |
| STATE.md | ✓ Updated | This file |

## Completed Plans

| Plan | Summary | Date | Commits |
|------|---------|------|---------|
| 01-01 | [01-01-SUMMARY.md](./phases/01-foundation-error-handling-http-memory/01-01-SUMMARY.md) | 2026-07-08 | 3c42c9e, 9f63958, b41ac01 |
| 01-02 | [01-02-SUMMARY.md](./phases/01-foundation-error-handling-http-memory/01-02-SUMMARY.md) | 2026-07-08 | 188e483, c5ba1f9, fbc9c77 |
| 01-03 | [01-03-SUMMARY.md](./phases/01-foundation-error-handling-http-memory/01-03-SUMMARY.md) | 2026-07-08 | 4b30b24, fe177b0, 075f497 |
| 01-04 | [01-04-SUMMARY.md](./phases/01-foundation-error-handling-http-memory/01-04-SUMMARY.md) | 2026-07-08 | c1eb2e3, c9e6bb6, 1cbfc74, c9d7c72 |
| 01-05 | [01-05-SUMMARY.md](./phases/01-foundation-error-handling-http-memory/01-05-SUMMARY.md) | 2026-07-08 | 37f2d82, b38357d, da69621 |
| 02-01 | [02-01-SUMMARY.md](./phases/02-performance-caching-parallelism/02-01-SUMMARY.md) | 2026-07-09 | 01460aa, c90f3e8, 8dd033b |
| 02-02 | [02-02-SUMMARY.md](./phases/02-performance-caching-parallelism/02-02-SUMMARY.md) | 2026-07-09 | 27924d8, 0e99a85, 82bdeda |
| 02-03 | [02-03-SUMMARY.md](./phases/02-performance-caching-parallelism/02-03-SUMMARY.md) | 2026-07-09 | b883bb9, c584d85, 8aff1b2, 935bf90, 1aded7a |
| 03-01 | [03-01-SUMMARY.md](./phases/03-usability-configuration-validation/03-01-SUMMARY.md) | 2026-07-09 | fb3dab4, 5df156d, 7386e9e |
| 03-02 | [03-02-SUMMARY.md](./phases/03-usability-configuration-validation/03-02-SUMMARY.md) | 2026-07-09 | af7bc71, 7d23a74, 65fad22 |
| 03-03 | [03-03-SUMMARY.md](./phases/03-usability-configuration-validation/03-03-SUMMARY.md) | 2026-07-09 | 25eabd1, 5c63824, 6a3f0b8 |
| 03-04 | [03-04-SUMMARY.md](./phases/03-usability-configuration-validation/03-04-SUMMARY.md) | 2026-07-09 | a0ee350, a4e757b, 27c0cd0 |
| 03-05 | [03-05-SUMMARY.md](./phases/03-usability-configuration-validation/03-05-SUMMARY.md) | 2026-07-09 | 78a5197 |
| 04-03 | [04-03-SUMMARY.md](./phases/04-user-experience-progress-output/04-03-SUMMARY.md) | 2026-07-10 | 07ee40c, e09321b, a09d866 |
| 04-01 | [04-01-SUMMARY.md](./phases/04-user-experience-progress-output/04-01-SUMMARY.md) | 2026-07-10 | 072d075, 751965d |
| 04-02 | [04-02-SUMMARY.md](./phases/04-user-experience-progress-output/04-02-SUMMARY.md) | 2026-07-10 | 3ed61e9, 993bd37, da8fb26 |
| 05-01 | [05-01-SUMMARY.md](./phases/05-testing-quality/05-01-SUMMARY.md) | 2026-07-09 | 4647915, 2b47065, 5982bd3, cf34a31 |
| 05-02 | [05-02-SUMMARY.md](./phases/05-testing-quality/05-02-SUMMARY.md) | 2026-07-09 | d4147ba, 12832c6, cc8d3e8 |

## Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260708-001 | Atomic commits for all modifications | 2026-07-08 | 402a4e3 | [260708-001-atomic-commits-refactor](./quick/260708-001-atomic-commits-refactor/) |
| 260709-uq1 | Add .env file support with CRUNCHYROLL_ETP_RT, WIDEVINE_CLIENT_ID_PATH, WIDEVINE_DEVICE_PATH, WIDEVINE_PRIVATE_KEY_PATH, XDG_CONFIG_HOME, OUTPUT_DIR | 2026-07-10 | 46167ce | [260709-uq1-add-env-file-support-with-crunchyroll-et](./quick/260709-uq1-add-env-file-support-with-crunchyroll-et/) |
| 260709-uw6 | Move config JSON from XDG directory to project root (./config.json) | 2026-07-10 | 7bc96eb | [260709-uw6-move-config-json-from-xdg-directory-to-p](./quick/260709-uw6-move-config-json-from-xdg-directory-to-p/) |
| 260709-v3w | Fix speed display unit — Bps() used newest instead of oldest sample timestamp | 2026-07-10 | 0167928 | [260709-v3w-fix-speed-display-unit-bps-uses-wrong-ol](./quick/260709-v3w-fix-speed-display-unit-bps-uses-wrong-ol/) |

Last activity: 2026-07-11

## Next Action

v1.1 roadmap ready. Next: `/gsd-plan-phase 6` (Track Hardening + Structured Logging).

## Project Reference

See: .planning/PROJECT.md (updated 2026-07-10)

**Core value:** Download any anime episode or full season from Crunchyroll into a single playable MKV file
**Current focus:** Phase 07 — Organized Output + Folder Metadata

## Current Position

Phase: 07 (Organized Output + Folder Metadata) — COMPLETE
Plan: 4 of 4
Status: Complete
Last activity: 2026-07-11 — Phase 07 complete

Progress: [████░░░░░░] 40% (2/5 v1.1 phases)

## Session

**Last session:** 2026-07-11T14:05:25.731Z
**Stopped at:** Completed 07-04-PLAN.md
**Resume file:** None

## Performance Metrics

| Phase | Plan | Duration | Notes |
|-------|------|----------|-------|
| Phase 06 P02 | 15min | 2 tasks | 6 files |
| Phase 06 P04 | 8min | 3 tasks | 3 files |
| Phase 06 P06 | 22min | 3 tasks | 5 files |
| Phase 06 P07 | 3 min | 3 tasks | 3 files |
| Phase 07 P01 | 1 min | 1 tasks | 2 files |
| Phase 07 P02 | 1 min | 2 tasks | 2 files |
| Phase 07 P04 | 7 min | 2 tasks | 7 files |

## Decisions

- [Phase 06]: Used a discardHandler for the diag pre-Init and fallback no-op logger so only the production path constructs slog.TextHandler.
- [Phase 06]: Kept diagnostic PII redaction centralized in redactAttr via HandlerOptions.ReplaceAttr while handlerWrapper preserves the handler seam.
- [Phase ?]: [Phase 06]: Kept missing-track skip surfacing on output.Global.Warn only, preserving the existing NDJSON warn contract.
- [Phase 06]: [Phase 06 P07] resolveLangs consults no env vars - no CRUNCHYROLL_AUDIO_LANG/SUBS_LANG decision exists in Phase 06, unlike the scalar resolveString path.
- [Phase 06]: [Phase 06 P07] Nil config slice => default fallback; explicit empty config array => empty slice to ERR-03 hard-error guard. Nil-vs-empty distinction preserved and tested.
- [Phase ?]: D-05 fix: season_number mux metadata arg (mux.go:108) now sources SeasonNumber, not EpisodeNumber — proven by regression test with distinct Season=2/Episode=7
- [Phase ?]: D-04: drop [{quality}] from on-disk MKV filename; *videoQuality param retained (07-02)
- [Phase 07 P04]: Artwork URL extraction uses direct poster_url/backdrop_url fields from SeriesInfo, with EpisodeMetadata fallback for single-episode metadata.
- [Phase 07 P04]: FetchArtwork allowlist is HTTPS only for .crunchyroll.com, .vmdcdn.com, and .akamaized.net; rejected URLs return ErrArtworkNotFound before network access.
