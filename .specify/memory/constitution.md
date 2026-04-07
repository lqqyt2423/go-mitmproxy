<!--
Sync Impact Report
===================
- Version change: N/A → 1.0.0 (initial ratification)
- Added principles:
  - I. Addon-First Architecture
  - II. Protocol Fidelity
  - III. Cross-Platform Delivery
  - IV. Performance & Transparency
  - V. Developer Experience
- Added sections:
  - Technical Constraints
  - Development Workflow
  - Governance
- Removed sections: none
- Templates requiring updates:
  - .specify/templates/plan-template.md — ✅ no changes needed (generic)
  - .specify/templates/spec-template.md — ✅ no changes needed (generic)
  - .specify/templates/tasks-template.md — ✅ no changes needed (generic)
- Follow-up TODOs: none
-->

# go-mitmproxy Constitution

## Core Principles

### I. Addon-First Architecture

All traffic inspection, modification, and export capabilities MUST be
implemented as addons conforming to the `Addon` interface. The proxy
core (`proxy/`) MUST remain a generic MITM engine; feature-specific
logic belongs in addon packages (`addon/`, `web/`).

Rationale: This mirrors Charles's extensibility while keeping the
core small, testable, and embeddable as a library.

### II. Protocol Fidelity

go-mitmproxy MUST faithfully relay HTTP/1.1, HTTP/2, WebSocket, and
SSE traffic. Intercepted traffic MUST be semantically identical to
the original unless an addon explicitly modifies it. TLS certificate
generation MUST produce certificates that browsers accept when the
CA is trusted.

Rationale: A proxy that silently alters traffic is worse than no
proxy. Users depend on accurate capture for debugging.

### III. Cross-Platform Delivery

The project MUST support three delivery surfaces:

1. **CLI binary** — standalone `go-mitmproxy` executable.
2. **Web UI** — React frontend embedded via `go:embed`, served
   alongside the proxy.
3. **Mobile framework** — gomobile `.xcframework` for iOS/macOS
   integration via the `mobile/` binding layer.

New features SHOULD be usable from at least the CLI and Web UI.
Mobile parity is desirable but not blocking.

### IV. Performance & Transparency

The proxy MUST add negligible latency to proxied connections. Large
bodies (>5 MB by default) MUST be streamed rather than buffered.
Connection lifecycle events MUST be observable through addon hooks,
enabling full flow tracing without modifying the core.

Rationale: Charles is the benchmark. Users will reject a proxy that
noticeably slows their traffic or hides connection state.

### V. Developer Experience

go-mitmproxy MUST be importable as a Go library with a clean public
API (`proxy.Options`, `proxy.NewProxy`, `proxy.AddAddon`). Addon
development MUST require only embedding `proxy.BaseAddon` and
overriding the needed hooks. The certificate store MUST be compatible
with mitmproxy's `~/.mitmproxy` directory so users can reuse
existing trusted CAs.

Rationale: The project competes on developer adoption. A low barrier
to extending the proxy is the primary differentiator vs. Charles.

## Technical Constraints

- **Language**: Go (latest two stable releases supported).
- **Frontend**: React (embedded build artifacts only; no SSR).
- **Mobile**: gomobile; iOS 15+ / macOS deployment targets.
- **Dependencies**: Minimize external dependencies. Prefer stdlib
  where feasible. All dependencies MUST be compatible with the
  MIT license.
- **Certificate compatibility**: MUST read/write certs in the same
  format and path as Python mitmproxy (`~/.mitmproxy`).

## Development Workflow

- **Build**: `make mitmproxy` produces the binary. `make test` runs
  all tests. `make mobile-framework` builds the mobile artifact.
- **Testing**: `go test ./...` is the baseline. Race detection
  (`-race`) SHOULD pass. Integration tests that require a running
  proxy MUST be clearly separated.
- **Code style**: `gofmt` + `go vet`. No additional linters enforced
  at this time.
- **Commits**: Concise single-line messages. No proactive changes to
  unrelated code.

## Governance

This constitution is the authoritative guide for architectural and
process decisions in go-mitmproxy. All feature proposals and PRs
MUST be consistent with the principles above.

**Amendment procedure**:
1. Propose the change with rationale.
2. Update this file with the new principle/section.
3. Increment the version per semantic versioning:
   - MAJOR: principle removal or redefinition.
   - MINOR: new principle or material expansion.
   - PATCH: clarification or wording fix.
4. Update `LAST_AMENDED_DATE`.

**Version**: 1.0.0 | **Ratified**: 2026-04-06 | **Last Amended**: 2026-04-06
