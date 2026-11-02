# Sabo DM v1.3.12 — Step 1 Audio Verification Fixes

- Corrected audio quality presets to advertise only tested encoder-supported bitrates.
- Limited Opus to 256 kbps maximum and OGG Vorbis to 192 kbps in the preset selector.
- Replaced misleading AAC/M4A/WMA VBR labels with explicit bitrate presets.
- Reserved output filenames atomically before FFmpeg starts, preventing existing files from being overwritten even under concurrent conversions.
- Added post-conversion validation that the output exists, is non-empty, and contains a readable audio stream.
- Added a Windows Step 1 verification checklist and PowerShell codec smoke test.

Windows build and in-app WebView2 end-to-end verification remain required before Step 1 can be marked complete.
