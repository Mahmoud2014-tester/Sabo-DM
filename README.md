# Sabo DM

Sabo DM is an independent Windows download-manager project based on the user's existing Go application. It uses the supplied DLMan screenshot and feature list as product/design requirements only; it does not copy DLMan source code or branding assets.

## What's included

- `*.go` — Go download engine, WebView2 desktop host, local dashboard API, and SQLite queue persistence.
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

## Main desktop dashboard

The normal `Sabo-DM.exe` opens the modern dashboard directly inside its native WebView2 window. The same local dashboard is also served at `http://127.0.0.1:19346/dashboard/` for troubleshooting. It reads the live queue and supports adding HTTP/HTTPS/magnet URLs, starting/resuming, pausing, removing, searching, filtering, and switching themes. The API binds to `127.0.0.1` only and is not intended to be exposed to a LAN or the public internet.

## Build on Windows

Requirements: Go 1.23+, a Windows machine, and the Microsoft Edge WebView2 Runtime. Run:

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


## Desktop interface architecture (WebView2)

The default Windows executable now starts the Sabo DM dashboard as its **main desktop window** using Microsoft Edge WebView2. The previous Win/Walk interface is retained only as an opt-in legacy build (`-tags legacyui`) and is not used by the normal Windows build.

- Go remains responsible for the download queue, HTTP transfer engine, persistence, video/torrent integrations, and browser-extension bridge.
- WebView2 hosts the local dashboard at `http://127.0.0.1:19346/dashboard/` inside the native Sabo DM window.
- The dashboard communicates with the Go engine through the loopback-only API.
- The Microsoft Edge WebView2 Runtime must be installed on the Windows machine. Windows 11 and most updated Windows 10 installations already include the Evergreen runtime; otherwise install the official runtime: https://developer.microsoft.com/microsoft-edge/webview2/

The default build uses `main_webview_windows.go`. The legacy native UI can be built explicitly with `go build -tags legacyui` (not recommended for normal use).


## Dashboard controls and settings

The dashboard includes in-window Minimize, Maximize/Restore, and Exit buttons. They call Go WebView2 bindings and operate on the real Windows host window. Exit asks for confirmation before closing the application.

The Settings button opens an appearance panel (dark/light). The global speed-limit selector sends the selected preset to the Go transfer engine. Other controls should only be considered implemented when their corresponding API/backend behavior is documented here and tested; queue/category management and per-download/per-queue limits are still pending.


## Browser extension refresh and synchronized hand-off

For an upgrade, remove the previous Sabo DM extension from `chrome://extensions` / `edge://extensions`, then load the new extracted `extension` directory using **Load unpacked**. Do not keep two Sabo DM extension copies enabled simultaneously. The extension version is now 1.3.3.

The confirmation window supports choosing Video or audio output (MP3, M4A, AAC, Opus, WAV, FLAC, or original audio). The extension sends one request to the desktop bridge; the browser's paused copy is cancelled only after the local Go application accepts the request. If the desktop application rejects or times out, the original browser download is resumed so the user does not lose it.

Audio extraction/conversion from supported media pages uses yt-dlp and FFmpeg/FFprobe. The app offers Install FFmpeg in Settings; conversion formats remain disabled until FFmpeg and FFprobe pass execution checks.

## Publishing updateable releases

The in-app **Settings → Update Version** feature checks the latest stable GitHub Release. To publish an update that the installed app can install:

1. Push the source changes to `main`.
2. Create and push a semantic version tag, e.g. `v1.3.3`.
3. The `Sabo DM stable release` workflow runs tests, builds `Sabo-DM.exe`, `Sabo-DM-Diagnostic.exe`, packages `Sabo-DM-Extension.zip`, and attaches them to a GitHub Release.
4. Users can then use **Settings → Update Version → Check for updates**.

The updater downloads only the `Sabo-DM.exe` asset from a published stable GitHub Release, validates its size, PE architecture and GitHub SHA-256 digest when supplied, stages it, then restarts the application to replace the running executable. It does not silently install arbitrary branch commits.


## v1.3.3 Windows behavior and installer

- Clicking the window X hides the dashboard while the desktop engine and downloads keep running.
- System Tray provides Open Sabo DM, Settings, Pause all downloads, Resume all downloads, and Exit. Exit cancels active downloads and closes the application.
- New settings default to launching with Windows; the installer also enables the per-user startup entry by default.
- The Inno Setup script `Sabo-DM-Setup.iss` creates Start Menu and optional Desktop shortcuts.
- The red download arrow / black circular ring icon is embedded from `sabo.ico`.

The Windows build and runtime behavior must be verified by GitHub Actions and on a Windows machine before publishing this as a stable release.


## v1.3.5 visual and desktop update

- Unified black/red visual theme and red-arrow/black-ring application identity.
- Close (X) hides the window to the notification area; System Tray Exit performs full shutdown and cancels active downloads.
- Installer provides desktop shortcut and Windows startup tasks.
