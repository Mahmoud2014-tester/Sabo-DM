# Sabo DM 1.29.0 — root-fix package

- Refreshed the browser toolbar popup and IDM-style download confirmation window.
- Confirmation window supports choosing video or audio output on supported media pages.
- Improved title-based filenames for media detected by the extension.
- Made extension-to-desktop hand-off wait for an explicit successful HTTP acceptance before cancelling the paused browser copy.
- Added a timeout and recovery behavior for failed hand-offs.
- Added an explicit pause verification before the extension takes over a browser download.
- Added a stable GitHub Release workflow that publishes the executable and extension ZIP for the in-app updater.
- Updated the in-app updater and FFmpeg/yt-dlp tools documentation.
- Kept FFmpeg conversion formats disabled when FFmpeg/FFprobe are unavailable.
