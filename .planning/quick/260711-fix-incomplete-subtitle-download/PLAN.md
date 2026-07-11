---
status: in-progress
created: 2026-07-11
---

# Fix incomplete subtitle download

Investigate why a downloaded episode can miss Crunchyroll on-screen text subtitles and harden subtitle download handling so invalid or partial CDN/API responses are not silently saved and muxed as subtitle files.

## Scope

- Inspect playback subtitle selection and subtitle download path.
- Add focused regression coverage for subtitle HTTP failure handling.
- Keep the change limited to subtitle download integrity unless the code reveals a clear track-selection bug.

## Verification

- `rtk go test ./internal/media`
- `rtk go test ./...`
