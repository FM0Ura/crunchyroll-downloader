---
phase: 07-organized-output-folder-metadata
verified: 2026-07-11T14:11:30Z
status: human_needed
next_action: "Human verification required. Complete the manual tests in the phase's *-UAT.md, then re-run the verify step until status is passed."
next_command: ""
score: 20/21 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Run a real Jellyfin and a real Kodi library scan against a completed Phase 7 output tree containing tvshow.nfo plus at least one per-episode .nfo."
    expected: "Both scanners ingest the series title, plot/genre/studio when present, season/episode numbers, and <uniqueid type=\"crunchyroll\"> without rejecting the NFO XML."
    why_human: "Unit tests prove well-formed encoding/xml output and escaping, but they cannot prove live Jellyfin/Kodi scanner behavior in this environment."
  - test: "Run a live Crunchyroll download whose CMS response contains artwork URLs and confirm the actual poster/backdrop CDN hosts are accepted by FetchArtwork."
    expected: "poster.jpg and backdrop.jpg are written at the series root when available; if a legitimate Crunchyroll CDN host is rejected, append that host suffix to allowedArtworkHostSuffixes and re-run artwork tests."
    why_human: "Automated tests prove the allowlist, 200/404/500 behavior, and non-fatal handling with httptest; only a live CMS/CDN response can confirm current production artwork host coverage."
---

# Phase 7: Organized Output + Folder Metadata Verification Report

**Phase Goal:** Users get Jellyfin/Kodi-friendly organized download folders with correct NFO metadata and fetched artwork
**Verified:** 2026-07-11T14:11:30Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|---|---|---|
| 1 | Downloads land in the organized nested layout | VERIFIED | `internal/download/episode.go` builds `outputBase/Season %02d/<Series> S%02dE%02d - <Title>.mkv`; `TestOutputDirCreatesSeriesSubfolderInOutputDir` and `TestEpisodeSingleEpisodeMirrorsSeasonLayout` cover the shape. Note: PLAN D-01 intentionally uses `Series Title/Season NN/...` without ROADMAP's literal `(Year)`. |
| 2 | Re-running skips already-downloaded episodes in the nested layout | VERIFIED | The skip uses the same deep `outputFile` variable as the mux target; `TestEpisodeOutputSkipsAlreadyDownloadedInNestedLayout` asserts `episodeMerge` is not invoked. |
| 3 | MKV and NFO metadata use corrected season number | VERIFIED | `internal/mux/mux.go` uses `EpisodeMetadata.SeasonNumber` for `season_number=`; `internal/nfo/nfo.go` uses `SeasonNumber` for `<season>`. Mux and NFO tests cover distinct season/episode values. |
| 4 | `tvshow.nfo` and per-episode `.nfo` are emitted with Crunchyroll unique IDs | VERIFIED | `internal/nfo` writes XML via `xml.MarshalIndent` + `xml.Header`; `episode.go` writes per-episode NFO beside the MKV; `season.go` and single-episode flow write `tvshow.nfo` at series root; tests cover unique IDs and non-fatal errors. |
| 5 | NFO files are readable by real Jellyfin and Kodi scanners | NEEDS HUMAN | Code and tests prove well-formed XML, escaping, and expected roots (`tvshow`, `episodedetails`), but live scanner ingestion requires real Jellyfin/Kodi instances. |
| 6 | Artwork is fetched to `poster.jpg`/`backdrop.jpg` when available, and 404 is non-fatal | VERIFIED | `internal/api/artwork.go` implements bearer-authenticated `FetchArtwork`, 404 sentinel, status checks before writes, and HTTPS/CDN allowlist; download wiring writes root-level artwork and warns without aborting. API/download tests pass. |

**Score:** 20/21 must-haves verified (0 present, behavior-unverified; 1 live-scanner item requires human verification)

### Deferred Items

None. Later phases cover compression and TUI work, not Phase 7 metadata/output correctness.

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/mux/mux.go` | `season_number=` from `EpisodeMetadata.SeasonNumber` | VERIFIED | `grep` evidence line 108; `TestMergeEverythingSetsCorrectSeasonNumber` passes. |
| `internal/download/episode.go` | Nested layout, deep skip, per-episode NFO, single-episode tvshow/artwork | VERIFIED | Substantive implementation with seams and tests; output path and skip share `outputFile`. |
| `internal/download/season.go` | Series-root tvshow/artwork before episode loop | VERIFIED | Uses same `cleanSeriesTitle` + `outputDir` root as `episode.go`; os.Stat guards; non-fatal warn paths. |
| `internal/api/types.go` | Enriched `EpisodeMetadata`, `SeasonEpisode.SeriesID`, `SeriesInfo` | VERIFIED | Fields present for `series_id`, poster/backdrop URLs, description, genres, studio. |
| `internal/api/season.go` | `GetSeriesInfo` CMS call | VERIFIED | `GetSeriesInfo(ctx, seriesId, ...)` uses `c.newRequest`, `c.Do`, `json.Unmarshal`; tests assert path and decoded fields. |
| `internal/api/artwork.go` | `FetchArtwork` + `ErrArtworkNotFound` | VERIFIED | HTTPS/CDN allowlist, bearer request, 404 sentinel, non-200 error, disk write only after 200. |
| `internal/nfo/` | XML NFO emitters | VERIFIED | `WriteEpisode` and `WriteTVShow` use `encoding/xml`, include unique IDs, and tests cover escaping/header/write behavior. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `download.Episode` path build | skip check and mux output | shared `outputFile` | WIRED | Deep path is used for `os.Stat`, `episodeMerge`, and per-episode NFO path derivation. |
| `EpisodeMetadata.SeasonNumber` | mux/NFO season metadata | direct field references | WIRED | Mux `season_number=` and NFO `<season>` both source `SeasonNumber`, not `EpisodeNumber`. |
| `api.GetSeriesInfo` | `tvshow.nfo` default path | `episodeGetSeriesInfo` / `seriesGetSeriesInfo` seams | WIRED | Single-episode and season flows call the series API before rich tvshow writes; title-only fallback only on fetch failure. |
| series root computation | `tvshow.nfo`, `poster.jpg`, `backdrop.jpg`, `Season NN/` | `cleanSeriesTitle` plus optional `outputDir` | WIRED | `season.go` and `episode.go` compute the same root; artifacts are siblings of `Season NN`. |
| artwork URL fields | FetchArtwork | `seriesArtworkFromInfo` + EpisodeMetadata fallback | WIRED | Uses `SeriesInfo.PosterURL/BackdropURL`, falling back to enriched episode metadata for single-episode flow. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|---|---|---|---|---|
| `internal/download/episode.go` | `outputFile` | `EpisodeInfo.EpisodeMetadata` + `outputDir` | Yes | FLOWING — tested by path/skip tests. |
| `internal/download/season.go` | `seriesInfo` | `seriesGetSeriesInfo(ctx, client, episodes[0].SeriesID, "", "")` | Yes | FLOWING — tests assert rich `SeriesInfo` reaches `WriteTVShow`. |
| `internal/nfo/nfo.go` | XML title/plot/genre/studio/uniqueid | `EpisodeInfo` / `SeriesInfo` | Yes | FLOWING — marshal tests parse/check content and escaping. |
| `internal/api/artwork.go` | JPG bytes | `c.Do` response body | Yes | FLOWING — API tests assert bytes are written on 200 and not on 404. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| Full regression gate | `rtk test go test ./...` | all packages ok | PASS |
| Go vet | `rtk go vet ./...` | no issues found | PASS |
| Mux season-number regression | `rtk test go test ./internal/mux/... -run TestMergeEverythingSetsCorrectSeasonNumber -count=1 -v` | PASS | PASS |
| Fresh download package tests | `rtk test go test ./internal/download/... -count=1` | ok | PASS |
| Fresh API package tests | `rtk test go test ./internal/api/... -count=1` | ok | PASS |
| Fresh NFO package tests | `rtk test go test ./internal/nfo/... -count=1` | ok | PASS |

### Probe Execution

No `scripts/.../probe-*.sh` files were declared or discovered for this phase. Step 7c skipped.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| OUT-01 | 07-02 | Organized `Series Title/Season 01/Series Title S01E01 - Title.mkv` layout | SATISFIED | `episode.go` nested path and path tests pass. ROADMAP literal includes `(Year)`, but 07-02 PLAN D-01 explicitly chose no parenthetical year. |
| OUT-02 | 07-02 | Re-run skips existing nested-layout downloads | SATISFIED | Deep-path skip test proves `episodeMerge` is not invoked. |
| OUT-03 | 07-01 | `season_number=EpisodeNumber` bug fixed | SATISFIED | Mux code uses `SeasonNumber`; mux regression test passes; NFO also uses `SeasonNumber`. |
| META-01 | 07-03 | `tvshow.nfo` at series root for media server metadata | SATISFIED_CODE; HUMAN_SCAN_PENDING | Code writes rich `tvshow.nfo` in single-episode and season flows; live Jellyfin/Kodi scan remains human verification. `REQUIREMENTS.md` still marks this Pending, which appears stale. |
| META-02 | 07-03 | Per-episode `.nfo` with `<uniqueid type="crunchyroll">` | SATISFIED_CODE; HUMAN_SCAN_PENDING | NFO package and episode wiring write beside the MKV; tests assert uniqueid and escaping. `REQUIREMENTS.md` still marks this Pending, which appears stale. |
| META-03 | 07-04 | `poster.jpg`/`backdrop.jpg` artwork, non-fatal on 404 | SATISFIED_CODE; LIVE_CDN_PENDING | Fetch/wiring/tests verify behavior; live Crunchyroll CDN host coverage remains human verification. |

**Orphaned requirements:** None. The six Phase 7 requirement IDs are all declared across plans and accounted for above.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---|---|---|---|
| `.planning/REQUIREMENTS.md` | 33-34, 112-113 | `META-01`/`META-02` still marked Pending | INFO | Code/tests satisfy the implementation side; update requirements traceability after human scan passes. |

No blocker debt markers (`TBD`, `FIXME`, `XXX`) were found in the touched production files. Test stubs are normal seam-based test doubles, not production placeholders.

### Human Verification Required

#### 1. Jellyfin/Kodi Live Scan

**Test:** Run a real Jellyfin and a real Kodi library scan against a completed Phase 7 output tree containing `tvshow.nfo` plus at least one per-episode `.nfo`.
**Expected:** Both scanners ingest the series title, plot/genre/studio when present, season/episode numbers, and `<uniqueid type="crunchyroll">` without rejecting the NFO XML.
**Why human:** Unit tests prove well-formed `encoding/xml` output and escaping, but they cannot prove live scanner behavior in this environment.

#### 2. Live Crunchyroll Artwork CDN Host Coverage

**Test:** Run a live Crunchyroll download whose CMS response contains artwork URLs and confirm the actual poster/backdrop CDN hosts are accepted by `FetchArtwork`.
**Expected:** `poster.jpg` and `backdrop.jpg` are written at the series root when available; if a legitimate Crunchyroll CDN host is rejected, append that host suffix to `allowedArtworkHostSuffixes` and re-run artwork tests.
**Why human:** Automated tests prove the allowlist gate and 200/404/500 behavior with `httptest`; only live CMS/CDN data confirms current production host coverage.

### Gaps Summary

No automated/code gaps found. Phase 7 should remain at `human_needed` until live Jellyfin/Kodi scanner ingestion and live Crunchyroll artwork CDN host coverage are confirmed.

---

_Verified: 2026-07-11T14:11:30Z_
_Verifier: the agent (gsd-verifier)_
