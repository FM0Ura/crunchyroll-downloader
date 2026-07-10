# Phase 6: Track Hardening + Structured Logging - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-10
**Phase:** 6-Track Hardening + Structured Logging
**Areas discussed:** Missing-track failure rules, Skip surfacing & NDJSON contract, Mux input validation depth (ERR-04), --log-level scope & rotation defaults, PII redaction strategy

---

## Missing-track failure rules

| Option | Description | Selected |
|--------|-------------|----------|
| First in --audio-lang | primary = audioLangs[0], the user's first listed language (already used as default/first track) | ✓ |
| Episode's native AudioLocale | primary = info.EpisodeMetadata.AudioLocale (language-stable across users) | |
| Both — intersection rule | primary = audioLangs[0]; also protect the episode's AudioLocale | |

**User's choice:** First in --audio-lang (with clarification: config.json `audio_lang` becomes an array `["ja-JP"]`, first element is primary)
**Notes:** User clarified that currently `config.json` has `"audio_lang": "ja-JP"` (string) and it should change to `["ja-JP"]` (array). The `--audio-lang` flag stays a comma-separated string.

| Option | Description | Selected |
|--------|-------------|----------|
| Primaria faltando = erro restrito | If audioLangs[0] not available, abort regardless of secondary dubs | ✓ |
| Degradar para primeira disponivel | Use first available dub as primary (with warn) | |
| Sempre erro se qualquer solicitada faltar | Current behavior extended — any missing = error | |

**User's choice:** Primária faltando = erro restrito
**Notes:** Protects user from getting an MKV in the wrong language without noticing.

| Option | Description | Selected |
|--------|-------------|----------|
| Simétrico ao áudio | subsLangs[0] = primary; missing primary = hard error; secondary = warn+skip | ✓ |
| Legendas sempre warn+skip | Any subtitle missing = warn+skip (never abort) | |

**User's choice:** Simétrico ao áudio

| Option | Description | Selected |
|--------|-------------|----------|
| Apenas omite a faixa | MKV has available tracks, warn already communicated what's missing | |
| Marca episodio como parcial | Accumulate skips; final summary shows "Ep 3 (2 faixas puladas)" | ✓ |

**User's choice:** Marca episodio como parcial

| Option | Description | Selected |
|--------|-------------|----------|
| Só aviso informativo | Partial mark is purely informational; no state persisted | |
| Estado persistido para retomar | Save skipped tracks in .part for re-run to resume | ✓ |

**User's choice:** Estado persistido para retomar
**Notes:** SCOPE CREEP detected — persisting .part state for resuming tracks is a new capability (Active requirement "Resumable downloads", unmapped to any phase). Redirected to deferred ideas. Phase 6 keeps only the informational partial marking.

| Option | Description | Selected |
|--------|-------------|----------|
| Warn+skip | Failed download of secondary dub = same as missing from list | |
| Erro restrito | Service promised the dub but infra failed — different from absent | ✓ |
| Diferenciar por tipo de falha | 404/manifest = warn+skip; license/timeout = hard error | |

**User's choice:** Erro restrito
**Notes:** Distinguishes "inexistent in list" (warn+skip) vs "exists but fails download" (hard error).

| Option | Description | Selected |
|--------|-------------|----------|
| Simétrico ao áudio | Subtitle inexistente = warn+skip; exists but fails = hard error | ✓ |
| Legendas sempre warn+skip | No DRM — any failure treated as warn+skip | |

**User's choice:** Simétrico ao áudio

---

## Skip surfacing & NDJSON contract

| Option | Description | Selected |
|--------|-------------|----------|
| Warn genérico existente | Reuse Type: "warn" event from ndjson.go; zero contract change | ✓ |
| Novo evento tipado `skipped_track` | Adds typed event; extends NDJSON contract (violates TUI-03) | |
| Reusar warn + campo extra | Type: "warn" + optional fields (track_type, locale, reason) | |

**User's choice:** Warn genérico existente
**Notes:** Respects TUI-03 (NDJSON output contract stays stable).

| Option | Description | Selected |
|--------|-------------|----------|
| Formato warn padrão | Reuse yellow ⚠ from output.Global.Warn | ✓ |
| Skip dedicado com cor diferente | New cyan ⏭ style for skips | |

**User's choice:** Formato warn padrão

| Option | Description | Selected |
|--------|-------------|----------|
| Sufixo na linha de sucesso | "(2 faixas puladas)" appended to success line | |
| Linha separada de warn após o sucesso | Success line + separate warn line listing skipped tracks | ✓ |

**User's choice:** Linha separada de warn após o sucesso

---

## Mux input validation depth (ERR-04)

| Option | Description | Selected |
|--------|-------------|----------|
| os.Stat size-only | Reject size==0 on each input; zero new deps | ✓ |
| os.Stat + ffprobe stream check | + ffprobe per input to verify stream type/language | |
| Stat video, ffprobe audio/subs | ffprobe only on the tracks at real mislabeling risk | |

**User's choice:** os.Stat size-only

| Option | Description | Selected |
|--------|-------------|----------|
| Erro restrito do episodio | Don't invoke FFmpeg, cleanup temps, mark failed | ✓ |
| Degrada: omite o input vazio e segue | Remove empty track, reindex -map, invoke FFmpeg | |

**User's choice:** Erro restrito do episodio

| Option | Description | Selected |
|--------|-------------|----------|
| Dentro de mux.MergeEverything | At the boundary before assembling args — rule lives where FFmpeg is invoked | ✓ |
| No chamador download.Episode | Validate before calling MergeEverything | |

**User's choice:** Dentro de mux.MergeEverything

---

## --log-level scope & rotation defaults

| Option | Description | Selected |
|--------|-------------|----------|
| Separados | --log-level controls only slog diagnostic; output.Global unchanged | ✓ |
| Unificado | --log-level also controls output.Global terminal verbosity | |

**User's choice:** Separados
**Notes:** Matches Out-of-Scope item "Replacing Outputter with slog — different planes; both coexist".

| Option | Description | Selected |
|--------|-------------|----------|
| Descarta (sem arquivo) | logger is no-op / io.Discard when --log-file unspecified | |
| stderr por padrão | slog writes to stderr | |
| Arquivo default em ./logs/ | ./logs/animeheaven.log by default — diagnostic always available | ✓ |

**User's choice:** Arquivo default em .logs/

| Option | Description | Selected |
|--------|-------------|----------|
| 10MB / 3 backups | ~40MB total | |
| 5MB / 3 backups | ~20MB total | |
| 10MB / 5 backups | ~60MB total — more history for long seasons | ✓ |

**User's choice:** 10MB / 5 backups

| Option | Description | Selected |
|--------|-------------|----------|
| Sim, lumberjack.v2 | De facto standard (~500 LOC, zero deps) | ✓ |
| Rotacao manual (sem nova dep) | Implement rotation manually | |

**User's choice:** Sim, lumberjack.v2

| Option | Description | Selected |
|--------|-------------|----------|
| Singleton global + WithGroup per subsistema | log.Global set in main(); child loggers via slog.WithGroup at package init | ✓ |
| Passar logger por parametro | Explicit logger param (breaks 20+ signatures) | |
| Context-carried logger | Logger in context.Context | |

**User's choice:** Singleton global + WithGroup per subsistema
**Notes:** Mirrors the accepted config/output global pattern.

| Option | Description | Selected |
|--------|-------------|----------|
| Marcador explicito | Strategic points = Info; rest = Debug. Explicit list | ✓ |
| Tudo em Info, filtrar por nivel | Log everything Info; user sets warn to reduce | |

**User's choice:** Marcador explicito

| Option | Description | Selected |
|--------|-------------|----------|
| Sim, mesmo padrao resolveString | flag > env > config > default; LogLevel/LogFile as *string in Config | ✓ |
| So via flag, sem config/env | Only --log-level/--log-file flags | |

**User's choice:** Sim, mesmo padrao resolveString

---

## PII redaction strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Match por nome de chave | ReplaceAttr: key in {token,cookie,etp_rt,client_id,private_key,authorization} → [REDACTED] | ✓ |
| Match por padrao de valor (regex) | Regex for bearer/JWT/cookie shapes | |
| Ambos: key-name + fallback regex | Key-name first, then regex fallback | |

**User's choice:** Match por nome de chave

| Option | Description | Selected |
|--------|-------------|----------|
| So os 5 nomeados + authorization | Bearer, cookies, etp_rt, client_id, private_key + BasicAuth | ✓ |
| Tambem redact device_id | device_id is a cookie sent to Crunchyroll | |
| Redact tudo que parece secret | device_id + URLs with content IDs + long random strings | |

**User's choice:** So os 5 nomeados + authorization

| Option | Description | Selected |
|--------|-------------|----------|
| slog.Handler com ReplaceAttr | Custom redactingHandler wrapping base handler; centralized | ✓ |
| No ponto de log (chamada slog) | Each call must avoid passing secrets | |

**User's choice:** slog.Handler com ReplaceAttr

| Option | Description | Selected |
|--------|-------------|----------|
| TextHandler default (LOG-06 diferido) | Human-readable; JSON deferred to v2 | ✓ |
| JSONHandler default | Structured for machine; less readable for quick debug | |

**User's choice:** TextHandler default

---

## the agent's Discretion

- Exact wording of the partial-episode warn line (may refine during implementation).
- Whether `subs_lang` in config also becomes an array (implied by D-02 for consistency; researcher/planner can confirm).
- Internal package name for the new logging code (`internal/diag/` vs `internal/log/` — `log` may collide with stdlib import).

## Deferred Ideas

- **Persist `.part` state to resume skipped tracks** — Active requirement "Resumable downloads — track completed segments in a `.part` state file" (unmapped). Belongs in its own future phase.
- **`--log-format=json` (LOG-06)** — switching from TextHandler to JSONHandler. Explicitly deferred to v2 in REQUIREMENTS.md.
- **ffprobe stream-type validation (deeper ERR-04)** — os.Stat size-only was chosen; ffprobe per-track considered but rejected for subprocess latency without clear v1.1 need.