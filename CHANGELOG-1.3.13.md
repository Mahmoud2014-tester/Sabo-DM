# Sabo DM v1.3.13 — Live Download Tracking

## Step 2 implementation
- Added a compact, branded live-tracking card for each queued or active download.
- Displays file name, inferred type, destination, status, transferred/total bytes, percentage, current speed, and ETA when the engine provides enough information.
- Added per-download pause/resume, cancel, open-file, and open-folder actions using the existing Go queue/API.
- Supports simultaneous downloads as independent cards; completed cards remain briefly visible.
- Added Arabic and French labels for the tracking controls and metadata.

## Verification
- JavaScript syntax, HTML parsing, and extension manifest JSON checks passed in the development environment.
- Windows build, real transfer metrics, multi-download concurrency, and action behavior still require Windows runtime verification.
