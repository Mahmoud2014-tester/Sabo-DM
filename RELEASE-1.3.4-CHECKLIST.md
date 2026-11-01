# Sabo DM v1.3.4 release checklist

## Included source changes
- Black/red dashboard theme and shared red-arrow / black-ring brand artwork.
- WebView2 dashboard uses the shared `sabo-logo.png` asset.
- Application/tray/installer icon source is `sabo.ico`; tray image is `tray.png`.
- Close (X) hides the window and leaves the engine running.
- System Tray `Exit` cancels active transfers, saves queue state, and shuts down the WebView2 app.
- Installer creates desktop and Start Menu shortcuts and offers startup registration (selected by default in Inno Setup).
- Development and installer version strings updated to 1.3.4.

## Before publishing
1. Run `go mod tidy` and `go test ./...` on Windows or in GitHub Actions.
2. Confirm `Build GUI executable` succeeds and the EXE carries the red/black icon.
3. Confirm `Build installer` succeeds and the installer includes the EXE.
4. Install on a clean Windows user profile. Verify desktop shortcut and startup checkbox.
5. Verify X hides to tray and downloads continue.
6. Verify tray Exit stops active transfers and exits the process.
7. Restart Windows (or sign out/in) and verify startup launches the app hidden.
8. Only after all checks pass, tag `v1.3.4`.

## Validation limitation
JavaScript syntax, HTML parsing, manifest JSON, and ZIP integrity were checked in the preparation environment. Go tests and Windows installer compilation could not be completed here because required Go modules could not be downloaded from `proxy.golang.org`.
