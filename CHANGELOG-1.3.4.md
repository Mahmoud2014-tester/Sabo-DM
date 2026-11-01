# Sabo DM v1.3.4

## Visual identity
- Introduced a unified black and red theme across the WebView2 dashboard.
- Added the red download arrow inside a black circular mark to the application, tray, installer, and extension icons.
- Improved responsive layout styling and small-window spacing.

## Desktop behavior
- Preserved the agreed close-to-tray behavior: the window X hides the UI while the engine and downloads continue.
- Preserved System Tray Exit as the full shutdown action that cancels active transfers and closes the application.
- Kept per-user Windows startup registration and the installer desktop/startup tasks.

## Packaging
- Updated development version to 1.3.4-dev and extension version to 1.3.4.
- Updated installer product version to 1.3.4.


### Follow-up fixes
- Fixed the dashboard logo by including the branding PNG in the embedded WebView2 UI assets.
- Removed duplicate in-dashboard minimize, maximize, and close controls; use the native Windows title bar controls.
- Audio-page confirmation now defaults to MP3 when FFmpeg is available and keeps the output selector enabled for supported media pages, allowing the user to choose the target format instead of silently keeping M4A.
