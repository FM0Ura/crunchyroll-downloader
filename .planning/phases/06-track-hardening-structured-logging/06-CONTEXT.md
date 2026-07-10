# Phase 6: Track Hardening + Structured Logging - Context

**Gathered:** 2026-07-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Two deliverables: (A) graceful missing-track handling so partial audio/subtitle dubs no longer abort episodes, and (B) a configurable structured diagnostic log (`log/slog`) with per-subsystem grouping, lumberjack rotation, and PII redaction. Output plane (`output.Global`) remains a separate, unchanged plane.

**In scope:** ERR-01, ERR-02, ERR-03, ERR-04, LOG-01, LOG-02, LOG-03, LOG-04, LOG-05.
**Out of scope:** Resumable downloads of skipped tracks (persisted `.part` state), `--log-format=json` (LOG-06, deferred to v2), replacing Outputter with slog.

</domain>

<decisions>
## Implementation Decisions

### Missing-Track Failure Rules

- **D-01: Primary track = first element of the language list.** `audioLangs[0]` is the primary audio; `subsLangs[0]` is the primary subtitle. The user's first listed language is explicitly the protected one.
- **D-02: `config.json` `audio_lang` schema changes from string to array.** `"audio_lang": "ja-JP"` becomes `"audio_lang": ["ja-JP"]` so "first element" is unambiguous. The `--audio-lang` CLI flag stays a comma-separated string (backward compatible). Apply the same array treatment to `subs_lang` for consistency. Schema migration must remain backward compatible (existing single-string configs should be tolerated or migrated with a warning).
- **D-03: Missing primary track = hard error (erro restrito).** If `audioLangs[0]` or `subsLangs[0]` is not available in the episode's versions/subtitles map, abort the episode with a clear error — even if secondary tracks exist. Prevents delivering an MKV in the wrong language silently.
- **D-04: Missing secondary track = warn + skip.** A requested non-primary locale that is not in the available map is skipped with a warn; the episode continues with the tracks that are available.
- **D-05: Subtitles symmetric to audio.** Same primary/secondary rule: `subsLangs[0]` missing = hard error; secondary subtitles missing = warn + skip.
- **D-06: "Exists but fails to download" = hard error.** A dub/subtitle that IS present in the available list but fails during download (Widevine license error, manifest 404, segment download error) is a hard error, NOT a warn+skip. The service promised the track but infra failed — different from absence. Applies symmetrically to subtitles.
- **D-07: Skipped episode marked as "partial" in summary.** The MKV omits skipped tracks, AND the final summary shows episode as partial. Format: normal success line (✓) followed by a separate warn line listing skipped tracks (e.g. "⚠ Episode 3 downloaded partially: 2 tracks skipped (ja-JP dub, en-US sub)"). Partial marking is informational-only — no state persisted for resuming tracks later.

### Skip Surfacing & NDJSON Contract

- **D-08: Reuse existing `warn` NDJSON event.** When a track is skipped in `--json` mode, emit the existing `Type: "warn"` event with a descriptive message (e.g. "skipping ja-JP dub: not available"). Zero change to the NDJSON contract — respects TUI-03 ("NDJSON output contract stays stable").
- **D-09: Human-mode skip reuses standard warn format.** The yellow ⚠ format from `output.Global.Warn` (consistent with all other warns in the app). No new visual style for skips.

### Mux Input Validation (ERR-04)

- **D-10: `os.Stat` size-only validation before FFmpeg.** Before assembling ffmpeg args, `os.Stat` each input file (videoFile, audioTracks[].File, subTracks[].File). Reject any input with `size == 0`. Zero new dependencies; catches the zero-byte-files case that causes positional `-map` mislabeling. Does not shell out to ffprobe.
- **D-11: Empty input = hard episode error.** When an empty input is detected, return an error from the episode (do not invoke FFmpeg, cleanup tempFiles, mark episode failed in season summary). Input-empty is a bug/infra failure, distinct from a missing track.
- **D-12: Validation lives inside `mux.MergeEverything`.** The os.Stat checks run at the boundary, before assembling ffmpeg args. The "never invoke FFmpeg with empty input" rule lives in the single point that invokes FFmpeg — robust against any future caller.

### --log-level Scope & Rotation

- **D-13: slog is a SEPARATE plane from `output.Global`.** `--log-level` controls only what goes to `--log-file` (diagnostic). `output.Global` remains controlled by `--json`/`--quiet`/`--debug-manifest` as today. The two coexist (per Out-of-Scope decision: "Replacing Outputter with slog — different planes").
- **D-14: Default log destination is `./logs/animeheaven.log`.** When `--log-file` is unspecified, the diagnostic slog writes to `./logs/animeheaven.log` by default — diagnostic always available. `--log-file` overrides the path.
- **D-15: Lumberjack rotation: MaxSize=10MB, MaxBackups=5, MaxAge=0.** ~60MB total disk (active + 5 backups). More history for long-season diagnostics.
- **D-16: Add `gopkg.in/natefinch/lumberjack.v2` as a new dependency.** De facto standard for Go log rotation (~500 LOC, zero deps). `slog.Handler` writes to `lumberjack.Writer` which wraps `io.Writer`.
- **D-17: `log.Global` singleton + `slog.WithGroup` per package.** A `log.Global` slog.Logger set in `main()` (mirrors the accepted `config`/`output` global pattern). Each subsystem (download/drm/media/mux/api) gets a child logger via `slog.WithGroup("subsystem")` at package init.
- **D-18: Strategic events = `slog.Info`; noise = `slog.Debug`.** Explicit marking:
  - **Info (strategic):** episode start, episode finish (with duration + size summary), FFmpeg summary (stderr excerpt), token refresh, season failure.
  - **Debug (noise):** per-segment progress, per-dub download start, per-subtitle download start, manifest fetch, cache hit/miss.
- **D-19: `--log-level`/`--log-file` follow the `resolveString` precedence pattern.** Flag > `CRUNCHYROLL_LOG_LEVEL`/`CRUNCHYROLL_LOG_FILE` env > config > default (Info level, `./logs/animeheaven.log`). Add `LogLevel` and `LogFile` as `*string` fields in `config.Config`.

### PII Redaction

- **D-20: Redact by key-name match via `slog.Handler` `ReplaceAttr`.** A custom `redactingHandler` wraps the base handler (TextHandler). When the attr key is `token`, `cookie`, `etp_rt`, `client_id`, `private_key`, or `authorization`, the value is replaced with `[REDACTED]`. Simple, zero regex, covers the 5 named fields + the hardcoded Basic Auth string in `internal/api/auth.go`.
- **D-21: Redaction scope = 5 named fields + `authorization` only.** Do NOT redact `device_id` (a UUID generated by the tool, not user data). Do NOT redact URLs/content IDs (not PII). Conservative-minimum scope per LOG-04.
- **D-22: Redaction centralized in the handler layer.** Producers never think about PII — the `redactingHandler` intercepts every record before it's written. One place to audit.
- **D-23: TextHandler default (LOG-06 `--log-format=json` deferred to v2).** Log format is `slog.TextHandler` (human-readable: `time=... level=INFO msg=episode_start group=download ep=3`). JSON format is deferred.

### the agent's Discretion

- Exact string format of the partial-episode warn line — the agent may refine wording.
- Whether `subs_lang` in config also becomes an array (decision D-02 implies yes for consistency, but the agent can confirm during research).
- Internal package structure for the new logging code (e.g., `internal/diag/` vs `internal/log/` — `log` may collide with stdlib import name).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements
- `.planning/REQUIREMENTS.md` — defines ERR-01..ERR-04, LOG-01..LOG-05 (Phase 6 scope) and the Out-of-Scope table (slog vs Outputter separation, no `--log-format=json` in v1.1).
- `.planning/ROADMAP.md` §"Phase 6: Track Hardening + Structured Logging" — goal, dependencies, 5 success criteria.
- `.planning/PROJECT.md` — Key Decisions table (output.Global singleton pattern, table-driven stdlib tests only), Constraints (Go 1.25, no new framework deps beyond stdlib).

### Codebase maps (note: dated 2026-07-08, pre-refactor to internal/ packages — structural details stale, but concerns/anti-patterns valid)
- `.planning/codebase/ARCHITECTURE.md` — data flow for single-episode download (steps 1-11); identifies the `season_number=EpisodeNumber` bug at `output.go:87` (now `internal/mux/mux.go:87`).
- `.planning/codebase/CONCERNS.md` §Security Considerations — flags the hardcoded Basic Auth (`token.go:31`, now `internal/api/auth.go`), `etp_rt` CLI exposure, and global mutable state as PII/architecture concerns this phase addresses.

### Source files (live code — read before planning)
- `internal/download/episode.go` — the track-handling code being hardened; lines 72-145 (audio/subtitle locale resolution + hard-error-on-missing), lines 89-100 (audio version loop), lines 141-145 (subtitle hard error).
- `internal/mux/mux.go` — positional `-map` arg building at lines 42-48 (the ERR-04 mislabeling risk); `season_number=EpisodeNumber` bug at line 87.
- `internal/output/output.go` — the `Outputter` interface (separate plane from new slog); `Global` singleton pattern D-17 mirrors.
- `internal/output/ndjson.go` — the `warn` event type to reuse per D-08.
- `internal/config/config.go` — Config struct (D-02, D-19 add array-typed + log fields); `resolveString` precedence pattern in `main.go:226-237`.
- `main.go` — flag definitions (lines 22-35), `resolveString`/`resolveEtpRt` precedence helpers (lines 226-252), `flag.Visit` explicit-flag tracking (lines 272-275), output mode init (lines 278-285).
- `internal/api/auth.go` — hardcoded Basic Auth (redaction target per D-20), `etp_rt` and `device_id` cookies (lines 49-50).
- `internal/api/client.go` — `Bearer <token>` header (line 91, 122), token refresh on 401 (lines 103-130) — these are PII redaction targets and slog strategic-event sources (token refresh = Info per D-18).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `output.Outputter` interface + `output.Global` singleton (`internal/output/output.go:32-43`) — the pattern D-17 mirrors for `log.Global`.
- `resolveString`/`resolveEtpRt` precedence helpers (`main.go:226-252`) — D-19 reuses for `--log-level`/`--log-file`.
- `flag.Visit` explicit-flag tracking (`main.go:272-275`) — required for the `resolveString` precedence to work.
- `ndjson.go` `warn` event type (`internal/output/ndjson.go`) — D-08 reuses for skip surfacing.

### Established Patterns
- **Singleton init in main():** `output.Init(mode)` at `main.go:278-285` — D-17 follows for logger init.
- **Precedence hierarchy:** flag > env > config > default, with `*string`/`*int` pointer fields in Config for explicit-only overrides — D-02, D-19 follow.
- **Error returns, not panic:** all v1.0 panic() already replaced with `fmt.Errorf` wrapping — Phase 6 hardening continues this.
- **Table-driven stdlib tests only:** (D-03 from v1.0) — Phase 6 tests continue this convention.

### Integration Points
- `download.Episode` (`internal/download/episode.go:94-100`, `:141-145`) — the two hard-error-on-missing-locale points that D-03/D-04/D-05/D-06 replace with primary/secondary logic.
- `mux.MergeEverything` (`internal/mux/mux.go:29`) — insertion point for D-10/D-12 os.Stat validation before arg assembly at line 34.
- `main()` flag block (`main.go:22-35`) — insertion point for `--log-level`/`--log-file` flag declarations + `output.Init` block for logger init.
- `config.Config` struct (`internal/config/config.go:12-21`) — D-02 changes `audio_lang`/`subs_lang` to arrays; D-19 adds `LogLevel`/`LogFile` pointer fields.
- `internal/api/client.go` token-refresh path (lines 103-130) — a strategic-event source per D-18 (token refresh = slog.Info) and PII redaction target per D-20 (Bearer token).

</code_context>

<specifics>
## Specific Ideas

- Partial-episode summary format: success line (✓) unchanged, followed by separate warn line: "⚠ Episode 3 downloaded partially: 2 tracks skipped (ja-JP dub, en-US sub)".
- Default log path `./logs/animeheaven.log` (not XDG, not project-relative) — the tool already moved config to project root (`./config.json`), so logs follow the same project-root convention.
- Lumberjack config: `MaxSize=10`, `MaxBackups=5`, `MaxAge=0`, `Compress=false` (compression deferred — adds CPU during downloads).

</specifics>

<deferred>
## Deferred Ideas

- **Persist `.part` state to resume skipped tracks** — track which tracks were skipped per episode so a re-run downloads only what's missing. This is the Active requirement "Resumable downloads — track completed segments in a `.part` state file" (PROJECT.md, unmapped to any phase). Belongs in its own future phase; out of scope for Phase 6 which only delivers the warn+skip graceful handling.
- **`--log-format=json` (LOG-06)** — switching from TextHandler to JSONHandler output. Explicitly deferred to v2 in REQUIREMENTS.md; TextHandler is sufficient for v1.1 diagnostics.
- **ffprobe stream-type validation** (deeper ERR-04) — os.Stat size-only check is the chosen depth; ffprobe per-track stream verification was considered but rejected for adding subprocess latency without a clear v1.1 need.

</deferred>

---

*Phase: 6-Track Hardening + Structured Logging*
*Context gathered: 2026-07-10*