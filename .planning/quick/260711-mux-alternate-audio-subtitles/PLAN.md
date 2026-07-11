---
status: in-progress
created: 2026-07-11
---

# Mux alternate audio-version subtitles

Investigate why Crunchyroll shows translated on-screen card text that is missing from the downloaded MKV subtitle track, and adjust subtitle selection so audio-version-specific subtitle tracks can be preserved.

## Findings

- The existing MKV contains ASS subtitle streams.
- The extracted Portuguese ASS stream does not contain the full Lihanna profile-card text shown in Crunchyroll.
- The downloader only selected subtitles from the first playback response, even when downloading multiple audio versions.

## Scope

- Keep the primary subtitle track unchanged.
- Add distinct same-locale subtitle URLs from the matching audio playback as alternate subtitle tracks.
- Preserve custom titles for alternate subtitle tracks during muxing.

## Verification

- `rtk go test ./internal/download ./internal/mux`
- `rtk go test ./...`
