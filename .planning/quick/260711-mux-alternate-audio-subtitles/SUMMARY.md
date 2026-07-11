---
quick_id: 260711-alt-audio-subs
status: complete
date: 2026-07-11
commit: pending
---

# Quick Task 260711: Mux alternate audio-version subtitles

## Changes

- **internal/download/episode.go** — when a requested subtitle locale matches a downloaded audio locale, fetch that audio version's playback and add a distinct same-locale subtitle URL as an alternate subtitle track.
- **internal/download/episode.go** — cache playback responses fetched for alternate subtitles so the later audio download phase reuses them instead of opening a second stream token.
- **internal/mux/mux.go** — allow subtitle tracks to carry a custom title, used for alternate same-locale tracks.
- **internal/download/episode_test.go** and **internal/mux/mux_test.go** — added regression coverage for alternate subtitle download/mux metadata.

## Verification

- `rtk go test ./internal/download ./internal/mux` — 66 passed.
- `rtk go test ./...` — 350 passed in 12 packages.

## Notes

The extracted MKV subtitle did not include the full on-screen Lihanna card text, so the problem was not caused by the mux losing events. The likely source is that Crunchyroll exposes different subtitle/sign tracks per audio playback. This change keeps the original subtitle track and adds the matching-audio alternate when Crunchyroll returns a different URL.
