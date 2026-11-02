# Step 2 — Windows runtime verification

Run `build.bat` on Windows, then verify the following in the actual WebView2 application:

1. Add a direct file download. Confirm a tracker card appears and shows name, file type, destination, status, bytes, percentage, speed, and ETA when total size is known.
2. Pause and resume from the tracker card. Confirm the engine state changes and progress resumes from the saved position.
3. Cancel an active transfer from its tracker card. Confirm the transfer stops and the row is removed; the downloaded destination file is not deleted.
4. Start at least three simultaneous downloads. Confirm each has its own card and independently updating progress/speed.
5. Open the destination folder during a transfer; open the file after completion.
6. Confirm completed cards disappear after a short delay and dismissed cards do not reappear for the same current item.
7. Switch English, Arabic, and French. Confirm tracker labels and layout direction remain correct.
8. Repeat with a video/site download and a torrent if those paths are part of the release test matrix.

Do not mark Step 2 complete until these Windows runtime checks pass.
