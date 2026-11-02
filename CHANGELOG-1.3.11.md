# Sabo DM v1.3.11 — Audio workflow integration

## Added
- WebView2 dashboard prompt after a completed media download to optionally extract or convert its audio.
- Separate FFmpeg conversion job with live progress and completion/error reporting.
- MP3, M4A, AAC, Opus, OGG Vorbis, WAV, FLAC, and WMA output presets.
- WMA output through FFmpeg's `wmav2` encoder when available.
- Direct downloaded audio files can also be converted to another supported audio format.

## Safety and behavior
- The original downloaded file is never modified; output is saved as a separate `_audio` file with collision-safe naming.
- Conversion requires a working FFmpeg and FFprobe installation.
- The application validates the completed source item and requested format/quality before starting conversion.

## Verification limits
- JavaScript syntax, HTML parsing, preset structure, and ZIP integrity were checked.
- Local FFmpeg codec smoke tests succeeded for MP3, M4A/AAC, Opus, OGG Vorbis, WAV, FLAC, and WMA. These do not replace testing with the Windows-bundled FFmpeg binaries.
- Go tests and Windows runtime/build tests could not be completed in this environment because required Go modules were unavailable and `proxy.golang.org` DNS resolution failed.
