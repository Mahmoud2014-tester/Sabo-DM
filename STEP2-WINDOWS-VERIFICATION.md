# Step 2 — Windows verification (v1.3.15)

## Separate native tracker window
1. Run `build.bat` and launch Sabo DM.
2. Start a download from the browser extension. Confirm a separate Windows window titled `Sabo DM — Download Tracker` appears; it must not be embedded in the main dashboard.
3. Confirm the tracker shows the same live progress, speed, ETA, status, and destination details as the Go download queue.
4. Start three downloads. Confirm all cards appear in the separate tracker and update independently.
5. Click the tracker's native minimize button. Confirm it disappears from the taskbar and downloads continue.
6. Open the Sabo DM notification-area icon menu and select `Show Download Tracker`. Confirm the same tracker window is restored with current progress.
7. Click the tracker's title-bar X. Confirm it hides rather than exiting Sabo DM or cancelling downloads. Restore it from the tray menu.

## Existing Step 2 behavior
8. Pause and resume one download; verify other downloads are unaffected and state transitions complete correctly.
9. Cancel one download; verify other downloads continue.
10. Complete a download. Confirm its tracker card remains visible and Open file / Open folder still work.
11. Change the main app language to English, Arabic, and French. Confirm the tracker updates without restarting either window and RTL layout is correct.

## Expected application behavior
- The tracker is a separate top-level Windows window, not an in-dashboard overlay.
- Minimizing or closing the tracker hides only the tracker.
- The shared Sabo DM tray menu restores the tracker.
- Hiding the tracker does not pause, cancel, or terminate downloads.
