# Sabo DM 1.3.5

## Fixes
- Embed the Sabo logo in the WebView2 dashboard assets so it loads from the embedded filesystem.
- Remove duplicate in-dashboard window controls; retain the native Windows title bar controls.
- Prefer the server-provided Content-Disposition filename over opaque browser download IDs.
- Default supported audio-page downloads to MP3 when FFmpeg is available; users can select another output format in the confirmation window.
- Restyle the browser download confirmation popup with the Sabo black/red visual theme and a more compact layout.
- Add a compact in-app download-start notification with the download name.
- Raise the default concurrent-download count from 3 to 5 (user-configurable), retain 16-way segmentation, and keep the configured speed limit enforced by the HTTP engine, yt-dlp and aria2c.

## Validation
- JavaScript syntax, HTML parsing, extension manifest JSON and ZIP integrity checks passed.
- Go tests/build remain unverified because required modules could not be downloaded from proxy.golang.org in this environment.
