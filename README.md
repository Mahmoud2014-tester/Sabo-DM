# Sabo DM

Sabo DM is an independent Windows download-manager project based on the user's existing Go application. It uses the supplied DLMan screenshot and feature list as product/design requirements only; it does not copy DLMan source code or branding assets.

## What's included

- `*.go` — Go/Walk Windows desktop application, download engine, local dashboard API, and SQLite queue persistence.
- `extension/` — Chromium browser extension source.
- `ui-preview/` — dashboard HTML/CSS/JavaScript served by the application through its embedded filesystem.
- `persistence.go` — transactional SQLite queue store with legacy JSON import and JSON fallback.
- `persistence_test.go` — SQLite persistence round-trip test.
- `engine_segments_test.go` — segment planning, exact byte coverage, and resume-sidecar validation tests.
- `engine_http_test.go` — local HTTP integration tests for multi-range downloads and fallback when servers ignore Range requests.
- `FEATURE-IMPLEMENTATION-STATUS.md` — feature-by-feature implementation and validation status.
- `build.bat` — local Windows build script.
- `.github/workflows/windows-build.yml` — Windows CI build and test workflow.

## Current capabilities

The retained application supports direct HTTP/HTTPS downloads, adaptive HTTP byte-range transfers with a configurable maximum of 1–32 segments per file (default maximum: 16; actual segment count adapts to file size and server range support), pause/resume/retry paths, video downloads through yt-dlp, torrent/magnet downloads through aria2, FFmpeg tooling, browser handoff, selected browser cookies, a global speed limit, concurrency settings, language selection, Windows startup and optional resume-on-launch.

The direct-download segment setting is in the desktop Settings panel under “Max segments per file (1–32)”. It is a maximum: the engine adapts the actual number of ranges to file size (approximately 8 MiB per segment), uses one stream below 1 MiB or when the server does not support byte ranges, and applies the setting to downloads started after the setting is changed. Live segment progress is shown in the dashboard while a direct file is downloading.

The download queue is now persisted in SQLite with transactional snapshots, WAL journaling, full synchronous writes, startup integrity checking, legacy `queue.json` import, and a JSON compatibility fallback. Active direct-file downloads also retain their existing `.gdm` segment maps and partial data for resume. See `FEATURE-IMPLEMENTATION-STATUS.md` for limitations and unverified work.

## Open the connected dashboard

After starting Sabo DM, open:

`http://127.0.0.1:19346/dashboard/`

The dashboard reads the live queue and supports adding HTTP/HTTPS/magnet URLs, starting/resuming, pausing, removing, searching, filtering, and switching themes. The API binds to `127.0.0.1` only and is not intended to be exposed to a LAN or the public internet.

## Build on Windows

Requirements: Go 1.21+ and a Windows machine. Run:

```bat
build.bat
```

The script creates `Sabo-DM.exe` and `Sabo-DM-Diagnostic.exe` when dependencies resolve successfully. No prebuilt executable is included in this source archive because a successful Windows build has not yet been verified.

## Build and test through GitHub Actions

1. Create a GitHub repository and upload the contents of this folder.
2. Open **Actions** and run the Windows build workflow (or push to `main`/`master`).
3. The workflow resolves Go modules, runs `go test ./...`, builds both Windows executables, and packages the browser extension.
4. Download the generated workflow artifact after a successful run. Review the build log; a workflow file is not itself proof of a successful build.

## Scope and third-party terms

The DLMan screenshot and feature list are used as product/design requirements only. DLMan source code, branding assets and its license file are not included. Third-party dependencies and bundled tools remain subject to their respective terms. This repository does not claim that every requested roadmap feature is already implemented.
