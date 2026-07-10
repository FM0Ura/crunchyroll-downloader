# Phase 7: Organized Output + Folder Metadata - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-10
**Phase:** 7-Organized Output + Folder Metadata
**Areas discussed:** Series year source, Single-episode layout, Filename metadata suffix, NFO depth + artwork scope

---

## Series Year Source

| Option | Description | Selected |
|--------|-------------|----------|
| Earliest availability_starts | Derive year from earliest episode timestamp across fetched episodes. Zero new API calls. | |
| New series-detail endpoint | Add richer Crunchyroll CMS call for premiere/air date. Most accurate; adds call + research risk. | |
| Omit (Year) when unknown | Use `Series Title/Season 01/` without parenthetical; deviates from OUT-01 literal. | |

**User's choice:** Free-text — "Não precisamos de ano" (Omit year unconditionally).
**Notes:** User explicitly rejected `(Year)` for ALL cases, not just the unknown-year case. Deviation from OUT-01 literal and Jellyfin canonical disambiguation convention is accepted; collision risk across distinct Crunchyroll series is low.

### Follow-up — Resumability key disambiguator

| Option | Description | Selected |
|--------|-------------|----------|
| Aceitável, só título | Keep `Series Title/` pure. Collisions rare; manual rename is fine. | ✓ |
| Anexar ID da série | `Series Title [CR-{id}]/` for collision-free resume. Injects opaque ID. | |
| Decida você | Agent's call during planning. | |

**User's choice:** Aceitável, só título.
**Notes:** OUT-02 skip key is the sanitized `Series Title` directory with no Crunchyroll ID appended.

---

## Single-episode Layout

| Option | Description | Selected |
|--------|-------------|----------|
| Sim, espelha temporada | Single `/watch/<id>` produces nested `Series Title/Season 01/` + `tvshow.nfo` + per-ep NFO. Uniform for Jellyfin. | ✓ |
| Não, flatten para single | Single episode writes directly to `outputDir` with `.nfo` beside it, no nested folders or `tvshow.nfo`. | |
| Decida você | Agent's call. | |

**User's choice:** Sim, espelha temporada.
**Notes:** Uniform structure regardless of invocation (single vs season). Jellyfin/Kodi always see the same tree; re-runs identify series-root identically.

---

## Filename Metadata Suffix

| Option | Description | Selected |
|--------|-------------|----------|
| Remover (alinhado ao OUT-01) | `Series Title S01E01 - Title.mkv` — no bracket. Matches OUT-01 literal. | ✓ |
| Manter [quality] | `Series Title S01E01 - Title [1080p].mkv` — deviates from OUT-01 but distinguishes multi-qualities. | |
| Remover agora, deixar hook p/ Phase 8 | Drop bracket now, but extract `buildFilename()` seam for Phase 8 compression marker. | |

**User's choice:** Remover (alinhado ao OUT-01).
**Notes:** Zero scraper impact (Jellyfin ignores the bracket). User did NOT require the Phase 8 filename seam; compression marker reintroduction is Phase 8's call.

---

## NFO Depth + Artwork Scope

### NFO depth

| Option | Description | Selected |
|--------|-------------|----------|
| Mínimo | Use existing-data fields only. Zero new CMS calls. | |
| Rico (novas chamadas) | Enrich NFO with plot/runtime/ratings via additional CMS data. | ✓ |
| Decida você | Researcher confirms response shape first. | |

**User's choice:** Rico (novas chamadas).
**Notes:** Discovering that `GetEpisodeInfo` (`internal/api/episode.go:47`) ALREADY calls `/content/v2/cms/objects/{id}` and only decodes `episode_metadata` + `title` — per-episode NFO enrichment is mostly extending the decoded struct of the existing response, NOT a new call. Series-level `tvshow.nfo` enrichment (plot/genre/studio) DOES need a new series-level CMS call.

### Artwork — de onde e onde gravar

| Option | Description | Selected |
|--------|-------------|----------|
| Série-root, fonte CMS existente | `poster.jpg`/`backdrop.jpg` at series root; URLs from existing `/objects/{id}` response. 404 non-fatal warn. | ✓ |
| Série-root, endpoint separado de imagens | Dedicated images API call. More robust if object endpoint lacks images; adds a hit. | |
| Decida você | Researcher confirms the object response's image fields. | |

**User's choice:** Série-root, fonte CMS existente.
**Notes:** Avoid dedicated images endpoint unless researcher confirms object response lacks usable poster/backdrop URLs. Artwork at series root only — never per-season (META-04 deferred) or per-episode.

### tvshow.nfo — chamada série-level separada

| Option | Description | Selected |
|--------|-------------|----------|
| Sim, adicionar chamada série | New `GET /content/v2/cms/objects/{seriesId}` for plot/genre/studio in `tvshow.nfo`. | ✓ |
| Não, só campos do episode | `tvshow.nfo` uses SeriesTitle + availability_starts only. No new call. | |
| Decida você | Researcher defines the endpoint. | |

**User's choice:** Sim, adicionar chamada série.
**Notes:** Exact endpoint path + response shape is research-needed (discretion item in CONTEXT.md). `episode_metadata` lacks series-level plot/genre, so the call is needed for richer `tvshow.nfo`.

---

## the agent's Discretion

- Exact `EpisodeMetadata` enrichment field names (plot/description/duration/ratings) — research must confirm Crunchyroll CMS response shape.
- Exact series-level CMS endpoint path (objects/{seriesId} vs series/{id}/...) — researcher confirms.
- Whether to add a dedicated `nfo` subsystem slog logger or route NFO warnings through `diag.MuxLogger`/`diag.APILogger`.
- NFO XML correctness verified against real Jellyfin AND real Kodi scans (success criterion 4) — researcher's job to confirm schema.
- New package location: extend `internal/mux/` vs new `internal/nfo/` vs `internal/metadata/` — agent's call (mux is FFmpeg-focused; NFO is XML emission).
- Inline filename edit vs extracted `buildOutputPath()` seam — either fine; user did not require the seam.

## Deferred Ideas

- Season-specific posters (`season01-poster.jpg`) — META-04, v2.
- Online TVDB/anidb NFO enrichment — Out-of-Scope.
- `--no-artwork` opt-out flag — not requested.
- Filename seam for Phase 8 compression marker — Phase 8's call.
- Resumable `.part` segment state (broader "Resumable downloads" Active requirement) — unmapped to any phase; OUT-02 only preserves v1.0 episode-skip in the nested layout.

---

*Discussion log: 2026-07-10*