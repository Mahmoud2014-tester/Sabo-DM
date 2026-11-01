# Sabo DM v1.3.6

## Fixes
- Fixed HTTP downloads from servers that ignore Range requests when the URL has no filename and the response Content-Type is `text/plain`.
- Prevented MIME extension inference from creating a bogus filename such as `asc` when no filename base exists.
- Preserved the caller-provided filename (for example `single.bin`) as the fallback when the server and URL do not provide a usable filename.

## Validation
- Targeted HTTP engine tests passed locally: `go test engine.go resolve.go util.go debug_release.go engine_http_test.go engine_segments_test.go`.
- Full `go test ./...` was not run to completion because this environment cannot download the `modernc.org/sqlite` dependency from `proxy.golang.org`.
