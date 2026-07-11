---
quick_id: 260711-subtitle-integrity
status: complete
date: 2026-07-11
commit: 0ecbc05
---

# Quick Task 260711: Fix incomplete subtitle download handling

## Changes

- **internal/media/segment.go** — `DownloadSubs` now defaults nil contexts, rejects non-200 subtitle HTTP responses, and verifies declared `Content-Length` when present before writing a temp `.ass` file.
- **internal/media/segment_test.go** — Added regression tests proving HTTP errors and incomplete subtitle bodies are not saved as subtitle temp files.

## Verification

- `rtk go test ./internal/media` — 60 passed.
- `rtk go test ./...` — 348 passed in 12 packages.

## Notes

This fixes a silent failure class where an expired/partial subtitle response could be muxed as if it were a valid complete subtitle. If the real Crunchyroll `.ass` file is valid but still lacks the on-screen translated card text shown in the browser, the next step is to capture the playback JSON and subtitle URL response for that episode to check whether Crunchyroll exposes a separate sign/forced subtitle track that the current `Episode.Subtitles` model does not decode.
