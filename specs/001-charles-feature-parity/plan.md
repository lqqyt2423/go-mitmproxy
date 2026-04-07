# Implementation Plan: Charles Feature Parity

**Branch**: `001-charles-feature-parity` | **Date**: 2026-04-06 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-charles-feature-parity/spec.md`

## Summary

Implement the major missing Charles Proxy features in go-mitmproxy:
throttling, rewrite rules, repeat/compose, session save/load, HAR
export, block list, no-caching, flow annotations, and statistics.
All features are implemented as addons following the existing
architecture, with JSON config file persistence and Web UI controls.

## Technical Context

**Language/Version**: Go 1.21+ (backend), TypeScript/React (frontend)
**Primary Dependencies**: `tidwall/match` (glob patterns), `golang.org/x/time/rate` (optional), `gorilla/websocket` (existing)
**Storage**: JSON config files on disk, gzip-compressed session files (`.gmps`)
**Testing**: `go test ./...`, race detection with `-race`
**Target Platform**: macOS, Linux, Windows (CLI + Web UI); iOS/macOS (mobile, deferred)
**Project Type**: Library + CLI + Web application
**Performance Goals**: <5ms throttling overhead; <5s session load for 1000 flows
**Constraints**: All addons MUST conform to existing `Addon` interface; no new dependencies unless MIT-compatible
**Scale/Scope**: 8 user stories, ~15 new Go files, ~10 new React components

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
| --------- | ------ | -------- |
| I. Addon-First | ✅ PASS | All features implemented as addons (BlockList, Rewrite, Throttle, NoCaching). Session/Repeat handled by WebAddon extension. |
| II. Protocol Fidelity | ✅ PASS | No modifications to core proxy behavior. Throttle wraps streams without altering content. Rewrite only modifies when rules explicitly match. |
| III. Cross-Platform | ✅ PASS | All features usable from CLI (config files) + Web UI. Mobile deferred per spec assumptions. |
| IV. Performance | ✅ PASS | Throttle uses streaming (no buffering). SC-008 requires <5ms overhead. NoCaching/BlockList are O(n) on small rule sets. |
| V. Developer Experience | ✅ PASS | All addons usable as library imports. Clean constructors: `addon.NewThrottle()`, `addon.NewRewrite()`, etc. |

**Technical Constraints Check**:
- Language: Go ✅
- Dependencies: `tidwall/match` already used ✅, no new non-MIT deps ✅
- Certificate compatibility: unchanged ✅

## Project Structure

### Documentation (this feature)

```text
specs/001-charles-feature-parity/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── cli-flags.md
│   └── websocket-protocol.md
└── tasks.md               # Created by /speckit.tasks
```

### Source Code (repository root)

```text
addon/
├── blocklist.go            # NEW: Block List addon
├── rewrite.go              # NEW: Rewrite rules addon
├── throttle.go             # NEW: Throttling addon
├── nocaching.go            # NEW: No Caching addon
├── har.go                  # NEW: HAR export/import
├── session.go              # NEW: Session save/load
├── decoder.go              # existing
├── dumper.go               # existing
├── maplocal.go             # existing
└── mapremote.go            # existing

proxy/
├── flow.go                 # MODIFY: add Annotation, Timing fields
├── addon.go                # existing (no changes needed)
├── proxy.go                # existing (no changes needed)
└── ...

cmd/go-mitmproxy/
├── main.go                 # MODIFY: add new CLI flags, register new addons
└── config.go               # MODIFY: add new config fields

web/
├── web.go                  # MODIFY: handle new message types
├── conn.go                 # MODIFY: annotation storage, repeat/compose
├── message.go              # MODIFY: add new message type constants
└── client/src/
    ├── utils/
    │   ├── message.ts      # MODIFY: add new message types
    │   └── flow.ts         # MODIFY: add annotation, timing fields
    ├── containers/
    │   ├── Compose.tsx      # NEW: Compose request UI
    │   ├── RepeatAdvanced.tsx # NEW: Repeat advanced UI
    │   ├── Statistics.tsx   # NEW: Statistics panel
    │   ├── Timing.tsx       # NEW: Timing waterfall
    │   ├── SessionMenu.tsx  # NEW: Save/Load/Export menu
    │   └── AnnotationPanel.tsx # NEW: Highlight + comment
    └── App.tsx             # MODIFY: add new views/panels
```

**Structure Decision**: Follow existing project layout. New addons go
in `addon/`. Web UI extensions go in `web/client/src/containers/`.
No new top-level directories needed.

## Complexity Tracking

No constitution violations to justify.
