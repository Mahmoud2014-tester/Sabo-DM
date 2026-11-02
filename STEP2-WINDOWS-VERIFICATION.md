# Step 2 — Windows verification (v1.3.14)

1. Build with `build.bat` on Windows.
2. Start three downloads. Confirm each tracker card updates independently.
3. Pause one active download. Confirm its state changes to `Pausing…`, then `Paused`; Resume it and confirm transfer continues. Verify the other two downloads are unaffected.
4. Cancel one download and confirm the others continue.
5. Complete a download. Confirm its tracker remains visible and displays both destination folder and full destination file path. Test Open file and Open folder after completion.
6. Change language to English, Arabic, and French. Confirm tracker text and layout update and the tracker does not cover the top-left app controls.
7. Note: a detached native tracker window is not included in this build and remains outstanding.
