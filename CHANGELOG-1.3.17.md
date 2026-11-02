# Sabo DM v1.3.17 — Step 2 Audio Prompt and Confirmation Cleanup

- Restrict post-download audio extraction prompts to the dedicated per-file tracker window.
- Hide the audio extraction modal in the main dashboard as a defensive UI rule.
- Close the browser confirmation pop-up shortly after Sabo DM successfully accepts Download or Add to Queue; errors remain visible and actionable.
- Preserve per-file tracker behavior, language synchronization, and existing download actions.

## Verification
- JavaScript syntax checks and HTML parsing are run for this source package.
- Windows/WebView2 runtime behavior must still be verified on Windows.
