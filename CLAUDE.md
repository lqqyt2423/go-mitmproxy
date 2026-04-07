# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
make mitmproxy          # Build binary: go-mitmproxy
make dev                # Run from source
make test               # go test ./... -v
make dummycert          # Build cert generation tool
make clean              # Remove binaries

# Single package test
go test ./proxy -v -run TestName

# Race detection (not in Makefile by default)
go test -race ./...

# Web UI (React frontend, embedded via go:embed)
cd web/client && npm install && npm run build

# Mobile (gomobile)
make mobile-init        # Install gomobile toolchain (first time only)
make mobile-framework   # Build .xcframework for iOS + macOS
make mobile-framework-ios   # iOS only
make mobile-framework-mac   # macOS only
make mobile-clean       # Remove .xcframework
```

## Architecture

go-mitmproxy is a MITM proxy with an addon-based plugin system. It intercepts HTTP/HTTPS traffic, with optional WebSocket and SSE support.

### Two-Server Model

- **Entry server** (`proxy/entry.go`): HTTP listener on `:9080`. Accepts client connections, handles HTTP CONNECT for HTTPS tunneling. Wraps connections with `wrapListener`/`wrapClientConn` for lifecycle tracking.
- **Attacker server** (`proxy/attacker.go`): Internal HTTPS server that intercepts tunneled traffic. Generates per-host TLS certs using a self-signed CA. Supports HTTP/2 via `h2Server`. Two interception modes:
  - **Eager** (`UpstreamCert=true`, default): Connects to origin first to fetch cert details, then accepts client TLS
  - **Lazy** (`UpstreamCert=false`): Accepts client TLS first with generated cert, then connects to origin

### Addon System

All extensibility goes through the `Addon` interface (`proxy/addon.go`) with ~20 event hooks. Embed `proxy.BaseAddon` and override only needed hooks. Register via `proxy.AddAddon()`.

Hook order for a typical HTTPS request:
`ClientConnected → ServerConnected → TlsEstablishedServer → Requestheaders → Request → Responseheaders → Response → ServerDisconnected → ClientDisconnected`

For WebSocket: `WebSocketStart → WebSocketMessage (repeated) → WebSocketEnd`
For SSE: `SSEStart → SSEMessage (repeated) → SSEEnd`

Stream hooks (`StreamRequestModifier`/`StreamResponseModifier`) are called instead of `Request`/`Response` when `Flow.Stream = true` or body exceeds `StreamLargeBodies` threshold (default 5MB).

### Key Data Structures

- **`Flow`** (`proxy/flow.go`): Central unit — holds `Request`, `Response`, optional `WebScoket` (note: typo is intentional, it's in the code), `SSE` data, and `ConnContext`
- **`ConnContext`** (`proxy/connection.go`): Links `ClientConn` and `ServerConn`, tracks `FlowCount` and `Intercept` flag
- **`Request`/`Response`**: Wrap `http.Request`/response with `Body []byte` for buffered mode or `BodyReader io.Reader` for streaming

### Web UI (`web/`)

`WebAddon` bridges the proxy to a React frontend via WebSocket at `/echo`. Frontend assets are embedded in the binary via `//go:embed client/build`. The UI receives flow events as typed messages (request headers, body, response headers, body, WebSocket/SSE events).

### Built-in Addons (`addon/`)

- **MapRemote**: URL pattern rewriting with glob support (`tidwall/match`)
- **MapLocal**: Serve local files for matched URLs
- **Dumper**: Export traffic to file
- **Decoder**: Decompresses response bodies (gzip, brotli, deflate)

### Mobile Binding Layer (`mobile/`)

gomobile-compatible Go API for iOS/macOS apps. Key components:
- **`Engine`**: Wraps `proxy.Proxy` with mobile-friendly lifecycle (`Start`/`Stop`/`IsRunning`)
- **`EventHandler`**: Callback interface (Swift implements this) for flow events
- **`bridgeAddon`**: Internal addon that serializes flow data as JSON and forwards to EventHandler
- **`flowStore`**: In-memory flow reference pool for on-demand body retrieval (avoids large data transfers across gomobile boundary)
- `proxy.Options.Listener` field enables injecting a custom `net.Listener` (needed for iOS Network Extension)

## Code Conventions (from AGENTS.md)

- Git commits: concise, single-line messages
- Minimal comments — only where logic isn't self-evident
- Do not predict next steps or proactively modify unrelated code

## Active Technologies
- Go 1.21+ (backend), TypeScript/React (frontend) + `tidwall/match` (glob patterns), `golang.org/x/time/rate` (optional), `gorilla/websocket` (existing) (001-charles-feature-parity)
- JSON config files on disk, gzip-compressed session files (`.gmps`) (001-charles-feature-parity)

## Recent Changes
- 001-charles-feature-parity: Added Go 1.21+ (backend), TypeScript/React (frontend) + `tidwall/match` (glob patterns), `golang.org/x/time/rate` (optional), `gorilla/websocket` (existing)
