# Sabo DM 1.3.2 — Universal downloads and adaptive UI

- Fixed semantic version comparison so stable releases supersede development builds (for example, v1.3.1 is newer than 1.3.1-dev).
- Added dynamic translations for updater status messages in Arabic and French.
- Improved automatic filename extension detection using Content-Disposition and Content-Type metadata for generic HTTP/HTTPS downloads.
- Made the dashboard adapt to window size, keeping the outer application viewport fixed and scrolling the content pane only when required.
- Retained support for generic direct file downloads, video/media extraction through yt-dlp, and torrent jobs.

Note: Actual downloads still depend on the remote server allowing direct access; DRM, login walls, expiring links, and unsupported streaming protocols may require site-specific support.
