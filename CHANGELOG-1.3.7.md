# Sabo DM v1.3.7 — Media format choice and progress popup

- Added a pre-download media format chooser for supported video/audio pages.
- MP3 is the recommended default for audio extraction when FFmpeg is available; M4A, AAC, Opus, WAV, FLAC and Original remain selectable.
- Added format guidance describing compatibility, file size and lossless options.
- Kept the browser confirmation window rectangular and redesigned its post-acceptance state as a compact progress view.
- Added a loopback `/status` API for the extension progress view to read the matching download's live status, percent, transferred bytes and speed.
- Local-only API access remains restricted to loopback and browser-extension origins.
