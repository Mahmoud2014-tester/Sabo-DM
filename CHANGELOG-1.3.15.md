# Sabo DM v1.3.15 — Independent Download Tracker Window

## Step 2 correction
- Added a second, independent WebView2 top-level window dedicated to download tracking.
- The tracker runs on its own locked Windows UI thread and reads the same live download queue through the existing loopback dashboard API.
- The tracker window opens when a browser-extension download is accepted and can be restored from the Sabo DM notification-area menu.
- Clicking the tracker's native minimize button or title-bar close hides it instead of terminating the application or stopping downloads.
- Added a `Show Download Tracker` command to the existing Sabo DM system-tray menu.
- Added a dedicated tracker-only dashboard presentation, keeping the main application interface out of the tracker window.
- Added a shared language endpoint so English, Arabic, and French selection in the main window is reflected by the separate tracker window.
- Bumped development build version to 1.3.15-dev.

## Verification status
- JavaScript syntax, HTML parsing, Go formatting, and ZIP integrity are checked in the development environment.
- Windows WebView2 multi-window behavior, native minimize interception, and tray restoration require Windows runtime verification.
- Go dependency tests/build may be unavailable where module downloads are blocked.
