# Research: Charles Feature Parity

**Date**: 2026-04-06
**Branch**: `001-charles-feature-parity`

## R1: Throttling Implementation Approach

**Decision**: Token bucket rate limiter wrapping `io.Reader`/`io.Writer`
at the stream level via `StreamRequestModifier` and
`StreamResponseModifier` hooks, with additional latency injection
in `Requestheaders`.

**Rationale**: The proxy already supports streaming via the
`StreamRequestModifier`/`StreamResponseModifier` hooks. Wrapping the
reader/writer with a rate-limited reader provides per-connection
bandwidth control without buffering entire bodies. Token bucket
is the standard algorithm for HTTP traffic shaping (used by tc/netem,
Charles, and most proxy tools).

**Alternatives considered**:
- Delaying entire response (rejected: breaks streaming, adds memory
  pressure for large files)
- TCP socket-level throttling via `net.Conn` wrapper (rejected:
  more complex, harder to make per-host, conflicts with TLS layer)
- `golang.org/x/time/rate` Limiter (considered: viable, but a
  simpler custom token bucket is sufficient and avoids dependency)

## R2: Rewrite Rule Engine

**Decision**: New `addon/rewrite.go` implementing `Requestheaders`
(for URL/header/query param modifications) and `Response` (for
body/status code modifications). Rules match via `tidwall/match`
glob patterns on full URL, consistent with MapRemote.

**Rationale**: Using the same `tidwall/match` library ensures URL
pattern consistency across MapRemote, MapLocal, Rewrite, and Block
List. The rule engine applies modifications in order within each
rule set, and all matching rules execute (no early exit).

**Rule operations** (matching Charles):
- Header: Add, Modify (name match → value replace), Remove
- Host: Replace (plain text or regex)
- Path: Replace (plain text or regex)
- Query Param: Add, Modify, Remove
- Response Status: Replace
- Body: Replace (plain text or regex, per FR-021)

**Alternatives considered**:
- Lua/JS scripting engine (rejected: over-engineered for this scope,
  addon API already covers programmatic use cases)
- Single "modify" operation (rejected: Charles differentiates
  Add/Modify/Remove for headers, users expect this granularity)

## R3: Session Serialization Format

**Decision**: gzip-compressed JSON. Each flow serialized with full
request/response headers and base64-encoded bodies. File extension:
`.gmps` (go-mitmproxy session).

**Rationale**: JSON is debuggable and consistent with existing
serialization patterns in the codebase (Flow.MarshalJSON). Gzip
compression typically reduces session files by 80-90% since HTTP
traffic is highly compressible. 1000 flows with average 10KB
body ≈ 10MB uncompressed → ~1-2MB compressed.

**Alternatives considered**:
- Protocol Buffers (rejected: adds dependency, harder to debug,
  no existing protobuf usage in project)
- SQLite (rejected: overkill for sequential read/write, adds CGO
  dependency)
- Raw JSON without compression (rejected: session files would be
  too large for sharing)

## R4: HAR 1.2 Export Format

**Decision**: Implement HAR 1.2 export as a standalone function in
a new `addon/har.go` file. Only export direction (go-mitmproxy →
HAR). Import direction (HAR → go-mitmproxy) converts HAR entries
to Flow structs.

**Rationale**: HAR 1.2 is the de facto standard for HTTP archive
exchange. Chrome DevTools, Firefox, and Charles all support it.
The format maps well to go-mitmproxy's Flow structure.

**Key mappings**:
- `Flow.Request` → `HAR.Entry.Request`
- `Flow.Response` → `HAR.Entry.Response`
- `Flow.StartTime` + `Flow.Response` timing → `HAR.Entry.Timings`
- Binary bodies → base64 encoded with `encoding` field set to "base64"
- WebSocket/SSE → `_webSocketMessages`/`_sseEvents` custom fields
  (HAR spec allows underscore-prefixed extensions)

## R5: Addon Registration Order for Priority

**Decision**: Control execution priority via registration order in
`main.go`. Register addons in this order:
1. UpstreamCertAddon (existing)
2. LogAddon (existing)
3. **BlockList** (new - highest priority, can early-exit)
4. **MapRemote** (existing, moved after BlockList)
5. **MapLocal** (existing, moved after BlockList)
6. **Rewrite** (new - lowest priority among modifiers)
7. **NoCaching** (new - header stripping after rewrite)
8. **Throttle** (new - rate limiting via stream hooks)
9. WebAddon (existing - always last for UI visibility)
10. Dumper (existing - recording after all modifications)

**Rationale**: This matches the clarified priority: Block > Map >
Rewrite. BlockList sets `f.Response` to trigger early exit, so
MapRemote/MapLocal/Rewrite never execute for blocked requests.
NoCaching runs after Rewrite so rewritten headers are also stripped.
Throttle uses stream hooks which chain independently.

## R6: WebSocket Message Protocol Extensions

**Decision**: Add new message types to the existing binary protocol
for Repeat, Compose, Session, HAR, Annotations, and Statistics.

**New message types**:
```
messageTypeRepeatRequest       = 40
messageTypeComposeRequest      = 41
messageTypeRepeatAdvanced      = 42
messageTypeSaveSession         = 50
messageTypeLoadSession         = 51
messageTypeExportHAR           = 52
messageTypeImportHAR           = 53
messageTypeSetAnnotation       = 60
messageTypeStatistics          = 70
messageTypeTimingData          = 71
```

**Rationale**: Extending the existing binary protocol maintains
backward compatibility (version field handles it). New types use
ranges 40-71 to avoid collision with existing 0-32 range.

## R7: Repeat & Compose Implementation

**Decision**: Implement in `web/web.go` as WebSocket message handlers.
Repeat creates a new `http.Request` from the captured flow and sends
it via `http.DefaultClient` (not through the proxy itself). The
response is wrapped in a new Flow and pushed to all connected
WebSocket clients.

**Rationale**: Sending through an external HTTP client avoids
infinite loops (proxy intercepting its own repeated requests).
The response is still captured as a Flow for display in the UI.

**Repeat Advanced**: Uses a goroutine pool with configurable
concurrency. Results are aggregated and sent as a summary message.

## R8: Timing Data Collection

**Decision**: Extend `ConnContext` with timing fields populated
during the connection lifecycle. Add `TimingData` struct to `Flow`
populated from connection events.

**Timing phases**:
- DNS: captured via custom `net.Dialer` with `DialContext` hook
- TCP Connect: time between DNS resolution and TCP handshake complete
- TLS Handshake: time between TCP connect and TLS established
- Request Sent: time to write request headers + body
- TTFB (Waiting): time between request sent and first response byte
- Content Download: time between first response byte and last

**Rationale**: Go's `net/http/httptrace` package provides hooks for
all these phases. Wrap the transport's dialer and TLS config to
capture timing without modifying the core proxy loop.
