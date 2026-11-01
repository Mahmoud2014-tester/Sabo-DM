# Sabo DM 1.3.5 — UI and audio output fixes

- Fixed the missing/corrupted dashboard logo by including `sabo-logo.png` in the embedded `ui-preview` assets.
- Removed duplicate minimize, maximize, and close buttons from the dashboard header. The native Windows title-bar controls remain the desktop window controls.
- Improved audio-page confirmation defaults: when FFmpeg is available, MP3 is preselected for audio media pages; users can explicitly choose MP3, M4A, AAC, Opus, WAV, FLAC, or original audio.
- When FFmpeg is unavailable, conversion choices remain disabled and original audio is clearly retained rather than pretending conversion will happen.
- Updated the application and browser extension version to 1.3.5.

## Validation

- JavaScript syntax checks passed for the extension and dashboard scripts.
- Extension manifest JSON and dashboard/extension HTML parsing passed.
- Go tests and Windows executable builds could not be completed in this environment because Go dependencies could not be downloaded from `proxy.golang.org` (network/DNS unavailable). Run the Windows GitHub Actions workflow to perform the full build and test.
