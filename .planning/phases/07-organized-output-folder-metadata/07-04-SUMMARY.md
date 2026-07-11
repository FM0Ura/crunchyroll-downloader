---
phase: 07-organized-output-folder-metadata
plan: 04
subsystem: metadata
tags: [artwork, poster, backdrop, cms-api, ssrf, crunchyroll]

requires:
  - phase: 07-03
    provides: "SeriesInfo/GetSeriesInfo and series-root tvshow.nfo seams reused for artwork"
provides:
  - "Bearer-authenticated FetchArtwork helper with HTTPS/CDN host allowlist"
  - "Series-root poster.jpg and backdrop.jpg writes for season and single-episode flows"
  - "Non-fatal artwork failures with output.Global.Warn and diag.ApiLogger.Warn"
affects: [phase-07-verification, jellyfin-kodi-artwork, future-cdn-host-maintenance]

tech-stack:
  added: []
  patterns: [bearer-authenticated-binary-fetch, ssrf-host-allowlist, series-root-stat-guard, non-fatal-dual-channel-warn]

key-files:
  created:
    - internal/api/artwork.go
    - internal/api/artwork_test.go
  modified:
    - internal/api/types.go
    - internal/download/season.go
    - internal/download/season_test.go
    - internal/download/episode.go
    - internal/download/episode_test.go

key-decisions:
  - "Artwork URL extraction uses direct poster_url/backdrop_url fields decoded from SeriesInfo, with EpisodeMetadata poster_url/backdrop_url as the single-episode fallback."
  - "No dedicated images endpoint was added; Plan 03's GetSeriesInfo CMS response is the default source."
  - "FetchArtwork accepts HTTPS URLs only for .crunchyroll.com, .vmdcdn.com, and .akamaized.net host families; rejected URLs return ErrArtworkNotFound before any HTTP call."
  - "Live code uses diag.ApiLogger, not the plan text's diag.APILogger spelling."

patterns-established:
  - "Pattern: API helpers return sentinel/typed errors and keep user logging in download callers."
  - "Pattern: artwork files are static basenames joined to the sanitized series root and skipped with os.Stat when present."

requirements-completed: [META-03]

coverage:
  - id: D1
    description: "FetchArtwork writes artwork bytes on 200, returns ErrArtworkNotFound on 404, and rejects non-HTTPS or unknown hosts before network access."
    requirement: META-03
    verification:
      - kind: unit
        ref: internal/api/artwork_test.go#TestFetchArtworkWritesFileOn200
        status: pass
      - kind: unit
        ref: internal/api/artwork_test.go#TestFetchArtworkReturnsSentinelOn404
        status: pass
      - kind: unit
        ref: internal/api/artwork_test.go#TestFetchArtworkRejectsUnknownHost
        status: pass
      - kind: other
        ref: go test ./internal/api/... -count=1
        status: pass
    human_judgment: false
  - id: D2
    description: "Season flow writes poster.jpg and backdrop.jpg at the series root and treats 404 artwork failures as non-fatal."
    requirement: META-03
    verification:
      - kind: unit
        ref: internal/download/season_test.go#TestRunSeasonWritesArtworkAtSeriesRoot
        status: pass
      - kind: unit
        ref: internal/download/season_test.go#TestRunSeasonWritesArtworkNonFatalOn404
        status: pass
      - kind: other
        ref: go test ./internal/download/... -count=1
        status: pass
    human_judgment: false
  - id: D3
    description: "Single-episode flow mirrors season artwork writes at outputBase/series root and remains successful when artwork fetch returns 404."
    requirement: META-03
    verification:
      - kind: unit
        ref: internal/download/episode_test.go#TestEpisodeWritesArtworkOnSingleEpisodeFlow
        status: pass
      - kind: other
        ref: go test ./internal/download/... -count=1
        status: pass
    human_judgment: false
  - id: D4
    description: "Real Crunchyroll artwork host coverage beyond the known CDN families."
    requirement: META-03
    verification: []
    human_judgment: true
    rationale: "Requires a live CMS response from Crunchyroll; the helper is intentionally fail-closed so new legitimate CDN hosts can be appended when observed."

duration: 7 min
completed: 2026-07-11
status: complete
---

# Phase 7 Plan 4: Series Root Artwork Summary

**Bearer-authenticated poster/backdrop fetching with SSRF allowlist and non-fatal series-root writes**

## Performance

- **Duration:** 7 min
- **Started:** 2026-07-11T13:57:11Z
- **Completed:** 2026-07-11T14:04:10Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments

- Added `(*api.Client).FetchArtwork` with `ErrArtworkNotFound`, HTTPS-only URL parsing, and a Crunchyroll CDN host allowlist before request creation.
- Wired `poster.jpg` and `backdrop.jpg` writes to the series root for both season and single-episode flows, with per-file `os.Stat` guards.
- Kept artwork best-effort: 404, unknown hosts, 5xx, transport, and write errors warn through `output.Global.Warn` plus `diag.ApiLogger.Warn("artwork_fetch_failed", ...)` and do not abort downloads.
- Added regression coverage for 200, 404, 500, SSRF short-circuit, season-root placement, season 404 non-fatal behavior, and single-episode 404 non-fatal behavior.

## Task Commits

1. **Task 1: Build FetchArtwork helper with 404 sentinel + SSRF gate** - `5e861f1` (feat)
2. **Task 2: Wire series-root poster.jpg + backdrop.jpg** - `7cef9ec` (feat)

**Plan metadata:** pending final docs commit

## Files Created/Modified

- `internal/api/artwork.go` - FetchArtwork helper, ErrArtworkNotFound sentinel, and CDN allowlist gate.
- `internal/api/artwork_test.go` - FetchArtwork 200/404/500/unknown-host tests.
- `internal/api/types.go` - Added `poster_url`/`backdrop_url` decode fields on `SeriesInfo` and `EpisodeMetadata`.
- `internal/download/season.go` - Series-root artwork write helper, season pre-loop wiring, non-fatal warnings.
- `internal/download/season_test.go` - Season root placement and 404 non-fatal artwork tests.
- `internal/download/episode.go` - Single-episode artwork seam and series-root wiring.
- `internal/download/episode_test.go` - Single-episode artwork path and 404 non-fatal test.

## Decisions Made

- **Artwork URL extraction rule:** use direct CMS fields `poster_url` and `backdrop_url` on `SeriesInfo` from `GetSeriesInfo`. Single-episode code falls back to `EpisodeMetadata.PosterURL`/`BackdropURL` if a pre-enriched object carries those fields.
- **No dedicated images endpoint:** not added. Plan 03 already established the series-level CMS response and this plan reuses it.
- **Final FetchArtwork allowlist:** HTTPS only, with host suffixes `.crunchyroll.com`, `.vmdcdn.com`, and `.akamaized.net`. Tests inject `127.0.0.1` only by mutating the unexported allowlist seam under `t.Cleanup`.
- **Diagnostic logger spelling:** live source uses `diag.ApiLogger`; the plan text's `diag.APILogger` spelling was adapted to match the codebase.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Used live `diag.ApiLogger` spelling**
- **Found during:** Task 2
- **Issue:** The plan text referenced `diag.APILogger`, but the live diagnostic package exposes `ApiLogger`.
- **Fix:** Used `diag.ApiLogger` for `artwork_fetch_failed` and `artwork_no_url` events.
- **Files modified:** `internal/download/season.go`, `internal/download/episode.go`
- **Verification:** `go build ./...`, `go vet ./...`, and `go test ./internal/api/... ./internal/download/... -count=1` passed.
- **Committed in:** `7cef9ec`

---

**Total deviations:** 1 auto-fixed (1 Rule 1 bug)
**Impact on plan:** No scope change; this was required to compile against the live diagnostic API.

## Issues Encountered

- Requested read file `.planning/phases/07-organized-output-folder-metadata/07-03-CONTEXT.md` was not present on disk. I used `07-03-SUMMARY.md`, `07-CONTEXT.md`, `07-PATTERNS.md`, and live Plan 03 code as the source of truth.
- Initial Go verification inside the sandbox could not write to `~/.cache/go-build`; the same commands passed after rerunning with approved cache access.

## Threat Flags

None. The only new network surface is the planned artwork fetch helper, and T-07-08-T is mitigated by HTTPS + CDN suffix validation before any outbound request.

## Known Stubs

None.

## Verification

- `go build ./...` - passed
- `go vet ./...` - passed
- `go test ./internal/api/... ./internal/download/... -count=1` - passed (69 tests)
- Focused task tests - passed:
  - `go test ./internal/api/... -count=1 -v -run 'TestFetchArtwork'`
  - `go test ./internal/download/... ./internal/api/... -count=1 -v -run 'TestFetchArtwork|TestRunSeasonWritesArtwork|TestRunSeasonArtwork404|TestEpisodeWritesArtwork'`

## Self-Check: PASSED

- Created files exist: `internal/api/artwork.go`, `internal/api/artwork_test.go`.
- Task commits exist in git log: `5e861f1`, `7cef9ec`.
- No tracked file deletions were introduced.
- `main.go` was not modified.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Phase 7's planned code work is complete. The remaining validation for the phase is live Jellyfin/Kodi media-library UAT and any live Crunchyroll artwork host expansion if a legitimate CDN host appears outside the current allowlist.

---
*Phase: 07-organized-output-folder-metadata*
*Completed: 2026-07-11*
