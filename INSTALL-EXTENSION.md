# Update the Sabo DM browser extension

1. Close any open Sabo DM confirmation popups.
2. Open `chrome://extensions` (Chrome) or `edge://extensions` (Edge).
3. Find the old Sabo DM extension and click **Remove**. Do not leave both old and new copies enabled.
4. Extract the repository ZIP.
5. Enable **Developer mode**.
6. Click **Load unpacked** and select the repository's `extension` folder (the folder containing `manifest.json`).
7. Confirm that the extension version is **1.3.3**.
8. Start Sabo DM desktop app, then click the extension icon and use **Check connection**.

The extension communicates only with the local Sabo DM desktop service at `127.0.0.1:19345`. It pauses a browser download only when Chrome confirms the download is paused. It cancels the browser's copy only after the desktop service accepts the request. If the desktop service rejects or times out, the extension leaves the browser download available or resumes it.

For YouTube and supported media pages, the confirmation window lets you choose video or audio output. Audio conversion (MP3, M4A, AAC, Opus, WAV, FLAC) requires FFmpeg/FFprobe and yt-dlp; install them from Sabo DM Settings. Direct audio files are kept in their original format.
