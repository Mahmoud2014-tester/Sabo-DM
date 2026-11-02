# Sabo DM 1.3.16 — Step 2 tracker corrections

- Suppressed download tracker cards in the main dashboard and browser download-confirmation view.
- Changed tracker windows from one shared multi-download window to one independent window per download URL.
- Each tracker filters its view to its own download only; its controls continue to use the shared Go queue through the dashboard API.
- Tracker language follows the application language through the shared `/meta` API; the tracker language selector is read-only to avoid conflicting settings.
- Existing minimize-to-tray and close-to-hide behavior is retained for each tracker window.
