---
status: testing
phase: 07-organized-output-folder-metadata
source: [07-VERIFICATION.md]
started: 2026-07-11T14:11:30Z
updated: 2026-07-11T14:11:30Z
---

## Current Test

number: 1
name: Run a real Jellyfin and a real Kodi library scan against a completed Phase 7 output tree containing tvshow.nfo plus at least one per-episode .nfo.
expected: |
  Both scanners ingest the series title, plot/genre/studio when present, season/episode numbers, and <uniqueid type="crunchyroll"> without rejecting the NFO XML.
awaiting: user response

## Tests

### 1. Jellyfin/Kodi live scan
expected: Both scanners ingest the series title, plot/genre/studio when present, season/episode numbers, and <uniqueid type="crunchyroll"> without rejecting the NFO XML.
result: pending

### 2. Live Crunchyroll artwork CDN host coverage
expected: poster.jpg and backdrop.jpg are written at the series root when available; if a legitimate Crunchyroll CDN host is rejected, append that host suffix to allowedArtworkHostSuffixes and re-run artwork tests.
result: pending

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
