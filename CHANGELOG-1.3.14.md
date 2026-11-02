# Sabo DM v1.3.14 — Step 2 Tracker Reliability Fixes

## Changes
- Tracker cards now remain available after completion until manually dismissed or finished downloads are cleared.
- Shows both destination folder and full destination file path; completed items retain the Open file action.
- Pause-in-progress is represented as `Pausing…` and disables duplicate pause/resume clicks until the engine confirms the paused state.
- Tracker actions are serialized per download/action key rather than by one global UI lock, so acting on one download does not block controls for other downloads.
- Moved the in-interface tracker to the lower-right/lower-left-safe area (RTL-aware) to avoid covering the top-left application controls.
- Added Arabic labels for the new tracker status and destination fields.

## Verification
- JavaScript syntax check passed with Node.js.
- Go formatting passed.
- `go test ./...` could not run because required Go modules are unavailable and DNS access to `proxy.golang.org` failed.
- Windows build and runtime verification are still required.

## Known limitation
- The tracker is still rendered within the main WebView2 interface. A true independent native WebView2 tracker window has not yet been implemented or verified in this build; this remains an outstanding Step 2 requirement.
