# Sabo DM implementation status

## Completed in this milestone: SQLite queue persistence
- Queue snapshots are stored in `queue.db` using SQLite (`modernc.org/sqlite`).
- Queue replacement uses a single transaction, so interrupted writes do not leave a half-written queue snapshot.
- SQLite uses WAL journaling, `synchronous=FULL`, a busy timeout, and an integrity check at startup.
- Existing `queue.json` installations are imported into SQLite when the new database is empty; the legacy file is retained as `queue.json.migrated` after a successful import.
- If SQLite cannot be opened or written, Sabo DM falls back to an atomically replaced JSON file rather than silently discarding the queue.
- Queue progress snapshots are persisted periodically while downloads run. The direct HTTP engine also flushes the partial file and its `.gdm` segment map before reporting progress.
- On restart, unfinished transfers are restored as Paused. The existing Auto Resume preference can start them again; direct HTTP downloads use their saved `.gdm` segment metadata to continue.
- Login cookies and video cookie jars continue to be encrypted with the existing Windows DPAPI helper before being stored.
- Added a SQLite round-trip and clear-snapshot test, and added it to the Windows CI workflow.

## Existing connected dashboard and engine capabilities
- Dashboard assets are embedded into the Go application at build time.
- Loopback-only dashboard API at `127.0.0.1:19346`.
- Live queue snapshot from the existing Go queue model.
- Add HTTP/HTTPS/magnet URL to the existing queue.
- Start/resume, pause, and remove an item through the existing queue methods.
- Search/filter, theme switching and periodic refresh in the dashboard.
- HTTP/HTTPS direct downloads now support adaptive byte-range planning up to 32 segments per file. The user-configurable maximum is 1–32 (default 16); actual segment count is selected from file size using an 8 MiB target segment size.
- Range-unsupported servers and files smaller than 1 MiB use one stream.
- Resume sidecars are validated for contiguous ranges, in-bounds offsets, valid progress counters, file size, and a maximum of 32 segments before reuse.
- Live per-segment progress snapshots are sent from the transfer engine to the connected dashboard, which now renders a mini progress bar for each active segment.
- Existing concurrent-download limit and global speed limit.
- Existing video/torrent workflows and browser extension bridge.

## Not implemented / not yet verified
- CDN URL cache optimization.
- Per-download and per-queue bandwidth caps / bandwidth monitor.
- Queue creation, queue priorities, custom categories and per-category folders.
- Weekly scheduling, queue start/stop schedules, and post-download sleep/shutdown/hibernate actions.
- Custom post-download commands.
- Batch import, drag-and-drop and full multi-filter management.
- Domain credential vault, HTTP Basic Auth prompts, and credential settings UI.
- Standalone CLI, Firefox extension, macOS dock badge, in-app updater.
- Successful Windows executable build and end-to-end runtime/download testing.

## Validation status
- `gofmt` has been applied to the modified Go files.
- Adaptive segment-planning, exact range coverage, resume-sidecar validation, parallel HTTP range transfer, and single-stream fallback tests passed when run against the actual transfer engine and local HTTP test servers.
- The same isolated transfer-engine suite passed with Go race detection enabled.
- Dashboard JavaScript syntax validation passed with Node.js.
- The full `go test ./...` run is blocked here because the SQLite Go module is not cached and outbound DNS/network access is blocked. The Windows CI workflow will resolve dependencies and run the complete test suite.
- A complete Windows build and end-to-end download test have **not** been performed here. The CI run is required to confirm dependency resolution and compilation.


## Desktop shell correction — WebView2

- [x] Added a Windows WebView2 desktop entry point as the default application `main` (source change; pending Windows CI confirmation).
- [x] Configured a centered 1440×900 Sabo DM window.
- [x] Embedded the existing modern dashboard inside the native desktop window (not a separate browser tab).
- [x] Connected the WebView2 UI dispatcher to the Go queue's UI-thread callback mechanism.
- [x] Kept the Go download queue, transfer engine, SQLite persistence, video/torrent helpers, and browser extension API.
- [x] Retained the previous Win/Walk entry point only behind the `legacyui` build tag.
- [ ] Windows compile and runtime launch verification still required on GitHub Actions / a Windows machine.
- [ ] Full feature completion remains separate: queue/category editing, speed caps at per-item/per-queue scope, scheduler, post-download actions, and credential manager still need implementation and tests.
