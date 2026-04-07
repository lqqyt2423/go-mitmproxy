# Tasks: Charles Feature Parity

**Input**: Design documents from `/specs/001-charles-feature-parity/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Unit and module tests included for each user story.

**Organization**: Tasks grouped by user story for independent implementation.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1-US8)
- Include exact file paths in descriptions

## Path Conventions

- Backend addons: `addon/`
- Proxy core: `proxy/`
- CLI: `cmd/go-mitmproxy/`
- Web backend: `web/`
- Web frontend: `web/client/src/`

---

## Phase 1: Setup

**Purpose**: Shared data structures used by multiple user stories

- [x] T001 [P] Add `FlowAnnotation` struct (Color string, Comment string) and `TimingData` struct (DnsMs, ConnectMs, TlsMs, SendMs, WaitMs, ReceiveMs int64) to `proxy/flow.go`
- [x] T002 [P] Add `Annotation *FlowAnnotation` and `Timing *TimingData` fields to the `Flow` struct in `proxy/flow.go`; include both in `Flow.MarshalJSON()` output when non-nil
- [x] T003 [P] Add new message type constants (40-71) to `web/message.go`: RepeatRequest=40, ComposeRequest=41, RepeatAdvanced=42, SaveSession=50, LoadSession=51, ExportHAR=52, ImportHAR=53, SetAnnotation=60, Statistics=70, TimingData=71
- [x] T004 [P] Add corresponding message type enum values and parsing logic for types 40-71 in `web/client/src/utils/message.ts`
- [x] T005 [P] Add `annotation` (color, comment) and `timing` (dnsMs, connectMs, tlsMs, sendMs, waitMs, receiveMs) fields to the flow model in `web/client/src/utils/flow.ts`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Config infrastructure and flow store needed by multiple stories

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T006 Add `Rewrite`, `BlockList`, `Throttle` (string) and `NoCaching` (bool) fields to Config struct in `cmd/go-mitmproxy/config.go`; add corresponding CLI flag definitions
- [ ] T007 Add in-memory flow store (map[uuid.UUID]*proxy.Flow with mutex) to `web/web.go` in WebAddon; populate in `Request` and `Response` hooks; cap at configurable limit with LRU eviction
- [ ] T008 [P] Add annotation storage (map[string]*FlowAnnotation with mutex) to `web/conn.go`; add handler for messageTypeSetAnnotation that stores annotation and broadcasts to all connected clients
- [x] T009 Update addon registration order in `cmd/go-mitmproxy/main.go`: BlockList → MapRemote → MapLocal → Rewrite → NoCaching → Throttle → WebAddon → Dumper (per research.md R5)

**Checkpoint**: Foundation ready - user story implementation can begin

---

## Phase 3: User Story 1 - Throttling (Priority: P1)

**Goal**: Simulate slow network conditions with bandwidth and latency limits

**Independent Test**: Run proxy with `-throttle throttle.json` (3G preset), curl a URL through it, observe slower transfer

### Tests for User Story 1

- [x] T010 [P] [US1] Create `addon/throttle_test.go`: test `NewThrottleFromFile` loads JSON config correctly; test built-in presets (3G, 4G/LTE) have expected values; test invalid config file returns error
- [x] T011 [P] [US1] Test `rateLimitedReader` in `addon/throttle_test.go`: verify read throughput is within 15% of configured Kbps limit by reading 100KB through a 1000 Kbps limiter and measuring elapsed time; test zero bandwidth blocks reads; test reader with underlying EOF
- [x] T012 [P] [US1] Test host matching in `addon/throttle_test.go`: verify glob pattern matching (exact host, wildcard `*.example.com`, empty hosts = match all); verify non-matching hosts are not throttled

### Implementation for User Story 1

- [x] T013 [P] [US1] Create `addon/throttle.go`: define `ThrottleConfig`, `ThrottleProfile` structs with built-in presets (3G, 4G/LTE, WiFi lossy, 100% Loss) and `NewThrottleFromFile(filename)` constructor using `helper.NewStructFromFile`
- [x] T014 [US1] Implement token bucket rate limiter in `addon/throttle.go`: create `rateLimitedReader` wrapping `io.Reader` that limits read throughput to configured Kbps; handle packet loss by randomly dropping reads
- [x] T015 [US1] Implement `Requestheaders` hook in throttle addon (`addon/throttle.go`): check if flow host matches `Hosts` glob patterns using `tidwall/match`; if match, inject configured latency via `time.Sleep`
- [x] T016 [US1] Implement `StreamRequestModifier` and `StreamResponseModifier` hooks in throttle addon (`addon/throttle.go`): wrap reader with `rateLimitedReader` using upload/download Kbps from profile
- [x] T017 [US1] Register throttle addon in `cmd/go-mitmproxy/main.go`: load config from `-throttle` flag, create addon via `NewThrottleFromFile`, add at correct position in registration order

**Checkpoint**: Throttling works via CLI. `go test ./addon/ -run TestThrottle -v` passes.

---

## Phase 4: User Story 2 - Rewrite Tool (Priority: P1)

**Goal**: Automatically modify request/response headers, URLs, body, status codes based on rules

**Independent Test**: Run proxy with `-rewrite rewrite.json` containing an addHeader rule, curl through proxy, verify header appears

### Tests for User Story 2

- [x] T018 [P] [US2] Create `addon/rewrite_test.go`: test `NewRewriteFromFile` loads JSON correctly; test disabled rule sets are skipped; test empty Items list does nothing
- [x] T019 [P] [US2] Test URL matching in `addon/rewrite_test.go`: verify RewriteMatch matches host glob (`*.example.com`), path glob (`/api/*`), protocol filter, method filter; verify non-matching URLs skip rule; verify empty From fields match all traffic
- [x] T020 [P] [US2] Test request rewrite actions in `addon/rewrite_test.go`: create mock Flow with known headers; apply addHeader → verify header added; apply modifyHeader → verify value changed; apply removeHeader → verify header removed; apply host/path rewrite → verify URL modified
- [x] T021 [P] [US2] Test response rewrite actions in `addon/rewrite_test.go`: test body replacement with text mode (exact match); test body replacement with regex mode; verify Content-Length is recalculated after body change; test status code change
- [x] T022 [P] [US2] Test multiple rules in `addon/rewrite_test.go`: verify two rules matching same URL both apply in order; verify first rule's output is input to second rule

### Implementation for User Story 2

- [x] T023 [P] [US2] Create `addon/rewrite.go`: define `RewriteRuleSet`, `RewriteItem`, `RewriteMatch`, `RewriteAction` structs and `NewRewriteFromFile(filename)` constructor
- [x] T024 [US2] Implement URL matching in rewrite addon (`addon/rewrite.go`): match flow against `RewriteMatch` using `tidwall/match` for Host/Path globs, check Protocol and Method filters
- [x] T025 [US2] Implement request-side rewrite actions in `Requestheaders` hook (`addon/rewrite.go`): handle addHeader, modifyHeader, removeHeader, host, path, queryParam, addQueryParam, removeQueryParam actions on request target
- [x] T026 [US2] Implement response-side rewrite actions in `Response` hook (`addon/rewrite.go`): handle addHeader, modifyHeader, removeHeader, status, body actions on response target; support text and regex MatchMode for body replacement; recalculate Content-Length after body modification
- [x] T027 [US2] Register rewrite addon in `cmd/go-mitmproxy/main.go`: load config from `-rewrite` flag, create addon via `NewRewriteFromFile`, add after MapLocal in registration order

**Checkpoint**: Rewrite rules work via CLI. `go test ./addon/ -run TestRewrite -v` passes.

---

## Phase 5: User Story 3 - Repeat & Compose Requests (Priority: P2)

**Goal**: Re-send captured requests or craft new ones from the Web UI

**Independent Test**: Capture a GET request in Web UI, click Repeat, see new flow appear with fresh response

### Tests for User Story 3

- [ ] T028 [P] [US3] Create `web/repeat_test.go`: test repeat handler creates valid http.Request from stored Flow; test compose handler parses method/URL/headers/body correctly; test repeat with non-existent flow ID returns error
- [ ] T029 [P] [US3] Test Repeat Advanced concurrency in `web/repeat_test.go`: verify goroutine pool limits concurrent requests to configured concurrency; verify count requests are sent; verify aggregated stats are computed correctly

### Implementation for User Story 3

- [ ] T030 [P] [US3] Implement repeat request handler in `web/web.go`: on messageTypeRepeatRequest, look up flow from flow store, create new `http.Request` from stored request data, send via `http.Client`, wrap response as new Flow, push to all WebSocket clients
- [ ] T031 [P] [US3] Implement compose request handler in `web/web.go`: on messageTypeComposeRequest, parse method/URL/headers/body from message, create `http.Request`, send via `http.Client`, wrap response as new Flow, push to clients
- [ ] T032 [US3] Implement Repeat Advanced handler in `web/web.go`: on messageTypeRepeatAdvanced, parse count/concurrency config, use goroutine pool (semaphore pattern) to send concurrent requests, aggregate timing stats, send Type 70 summary to client
- [ ] T033 [P] [US3] Create `web/client/src/containers/Compose.tsx`: form with method dropdown, URL input, headers editor (key-value pairs), body textarea, Send button; sends messageTypeComposeRequest on submit
- [ ] T034 [P] [US3] Create `web/client/src/containers/RepeatAdvanced.tsx`: dialog with count input, concurrency input, Start button; sends messageTypeRepeatAdvanced; displays progress and results summary
- [ ] T035 [US3] Add Repeat button and Compose button to flow context menu / toolbar in `web/client/src/App.tsx`; wire to repeat handler (send messageTypeRepeatRequest) and open Compose dialog

**Checkpoint**: `go test ./web/ -run TestRepeat -v` passes. Web UI functional.

---

## Phase 6: User Story 4 - Session Save/Load & HAR Export (Priority: P2)

**Goal**: Save/load captured traffic sessions and export to HAR 1.2

**Independent Test**: Capture flows, save session, restart proxy, load session, verify flows restored

### Tests for User Story 4

- [x] T036 [P] [US4] Create `addon/session_test.go`: test SaveSession writes gzip-compressed JSON to file; test LoadSession reads it back; verify round-trip preserves flow ID, request method/URL/headers, response status/headers, base64-decoded bodies match originals
- [x] T037 [P] [US4] Test session edge cases in `addon/session_test.go`: test empty flow list; test flow with nil Response; test flow with WebSocket messages; test flow with FlowAnnotation; test corrupted file returns error
- [x] T038 [P] [US4] Create `addon/har_test.go`: test ExportHAR produces valid HAR 1.2 JSON; verify `log.version` is "1.2"; verify entry count matches flow count; verify request method/URL and response status map correctly; test binary body is base64-encoded with `encoding: "base64"`
- [x] T039 [P] [US4] Test HAR import in `addon/har_test.go`: test ImportHAR parses Chrome DevTools HAR export; verify flow request/response data reconstructed correctly; test empty HAR file returns empty flow list

### Implementation for User Story 4

- [x] T040 [P] [US4] Create `addon/session.go`: define `SessionFile`, `SessionFlow`, `SessionRequest`, `SessionResponse` structs; implement `SaveSession(flows []*proxy.Flow, filename string)` that serializes flows to gzip-compressed JSON (`.gmps`)
- [x] T041 [P] [US4] Implement `LoadSession(filename string) ([]*proxy.Flow, error)` in `addon/session.go`: read `.gmps` file, decompress gzip, unmarshal JSON, reconstruct Flow objects with request/response data and base64-decoded bodies
- [x] T042 [P] [US4] Create `addon/har.go`: define HAR 1.2 structs (HarLog, HarEntry, HarRequest, HarResponse, HarContent, HarTimings); implement `ExportHAR(flows []*proxy.Flow, filename string)` mapping Flow fields to HAR 1.2 format with base64-encoded binary bodies
- [x] T043 [US4] Implement `ImportHAR(filename string) ([]*proxy.Flow, error)` in `addon/har.go`: parse HAR JSON, convert entries to Flow objects, decode base64 bodies
- [ ] T044 [US4] Add session/HAR message handlers in `web/web.go`: on SaveSession/LoadSession/ExportHAR/ImportHAR message types, call corresponding addon functions using flows from flow store; on Load/Import, push restored flows to all clients via standard message types
- [ ] T045 [P] [US4] Create `web/client/src/containers/SessionMenu.tsx`: dropdown menu with Save Session, Load Session, Export HAR, Import HAR actions; each triggers file dialog and sends corresponding WebSocket message
- [ ] T046 [US4] Integrate SessionMenu into `web/client/src/App.tsx` toolbar/menu bar

**Checkpoint**: `go test ./addon/ -run TestSession -v` and `go test ./addon/ -run TestHAR -v` pass.

---

## Phase 7: User Story 5 - Block List (Priority: P3)

**Goal**: Block specific hosts/URLs and return configurable error responses

**Independent Test**: Run proxy with `-block_list blocklist.json`, curl blocked host, verify 403 returned

### Tests for User Story 5

- [x] T047 [P] [US5] Create `addon/blocklist_test.go`: test `NewBlockListFromFile` loads config; test `AddRule` programmatic API; test matching host glob pattern; test matching path glob; test method filter; test disabled rules are skipped
- [x] T048 [P] [US5] Test block behavior in `addon/blocklist_test.go`: create mock Flow, apply Requestheaders hook, verify `f.Response` is set with correct StatusCode and Body; verify non-matching flow has nil Response; test custom status codes (403, 503, 200)

### Implementation for User Story 5

- [x] T049 [P] [US5] Create `addon/blocklist.go`: define `BlockListConfig`, `BlockRule` structs with `NewBlockListFromFile(filename)` constructor; implement programmatic `AddRule(host, path string, statusCode int, body string)` method
- [x] T050 [US5] Implement `Requestheaders` hook in blocklist addon (`addon/blocklist.go`): iterate enabled rules, match host/path using `tidwall/match`, check method filter; on match, set `f.Response` with configured StatusCode and Body to trigger early exit
- [x] T051 [US5] Register blocklist addon in `cmd/go-mitmproxy/main.go`: load from `-block_list` flag, create addon via `NewBlockListFromFile`, register BEFORE MapRemote/MapLocal (highest priority per R5)

**Checkpoint**: `go test ./addon/ -run TestBlockList -v` passes.

---

## Phase 8: User Story 6 - No Caching (Priority: P3)

**Goal**: Strip cache-related headers to force fresh content

**Independent Test**: Run proxy with `-no_caching`, curl cacheable resource, verify no Cache-Control/ETag in response

### Tests for User Story 6

- [x] T052 [P] [US6] Create `addon/nocaching_test.go`: test Requestheaders strips If-Modified-Since, If-None-Match, Cache-Control, Pragma from request headers; test Response strips/overrides Cache-Control, Expires, ETag, Last-Modified; verify other headers are preserved unchanged

### Implementation for User Story 6

- [x] T053 [P] [US6] Create `addon/nocaching.go`: implement NoCaching addon with `Requestheaders` hook that removes If-Modified-Since, If-None-Match, Cache-Control, Pragma headers from request
- [x] T054 [US6] Implement `Response` hook in NoCaching addon (`addon/nocaching.go`): remove/override Cache-Control, Expires, ETag, Last-Modified headers in response; set `Cache-Control: no-cache, no-store, must-revalidate`
- [x] T055 [US6] Register NoCaching addon in `cmd/go-mitmproxy/main.go`: enable via `-no_caching` bool flag, register after Rewrite in addon order

**Checkpoint**: `go test ./addon/ -run TestNoCaching -v` passes.

---

## Phase 9: User Story 7 - Highlight & Comment Flows (Priority: P3)

**Goal**: Mark flows with colors and text comments in Web UI

**Independent Test**: Right-click flow in Web UI, set color to red, add comment, verify display

### Implementation for User Story 7

- [ ] T039 [P] [US7] Create `web/client/src/containers/AnnotationPanel.tsx`: color picker (red, blue, green, yellow, purple) and comment text input; sends messageTypeSetAnnotation on change
- [ ] T040 [US7] Update flow list rendering in `web/client/src/App.tsx`: apply background color/indicator when flow has annotation.color; show comment icon when annotation.comment is non-empty
- [ ] T041 [US7] Display annotation comment in flow detail view in `web/client/src/containers/ViewFlow.tsx`: show comment text below flow metadata when present

**Checkpoint**: Flows can be highlighted with colors and annotated with comments.

---

## Phase 10: User Story 8 - Summary Statistics & Timing (Priority: P4)

**Goal**: Display traffic statistics and per-request timing waterfall

**Independent Test**: Capture 10+ flows, open Statistics view, verify counts and timing breakdown

### Implementation for User Story 8

- [ ] T042 [P] [US8] Implement timing data collection in `web/web.go`: in `Request` and `Response` hooks, capture StartTime and compute phase durations from flow lifecycle events; populate `flow.Timing` with DNS/Connect/TLS/Send/Wait/Receive milliseconds
- [ ] T043 [P] [US8] Implement statistics aggregation in `web/web.go`: compute total flows, status code distribution, total bytes sent/received, average response time from flow store; send Type 70 message to clients on request or periodically
- [ ] T044 [P] [US8] Create `web/client/src/containers/Statistics.tsx`: panel showing total requests, status code bar chart (2xx/3xx/4xx/5xx), total data transferred, average response time
- [ ] T045 [P] [US8] Create `web/client/src/containers/Timing.tsx`: waterfall chart component showing DNS, TCP, TLS, Send, Wait, Receive phases as horizontal stacked bars with millisecond labels
- [ ] T046 [US8] Integrate Statistics panel and Timing tab into `web/client/src/App.tsx`: add Statistics view toggle in toolbar; add Timing tab to flow detail view in `web/client/src/containers/ViewFlow.tsx`

**Checkpoint**: Users can view traffic statistics and per-request timing breakdown.

---

## Phase 11: Integration Tests

**Purpose**: End-to-end proxy tests validating addon interaction

- [ ] T056 [P] Create `proxy/integration_test.go`: test proxy with BlockList + MapRemote addons — blocked host returns 403, mapped host redirects; verify Block priority over Map (blocked host never reaches MapRemote)
- [ ] T057 [P] Create `proxy/integration_test.go`: test proxy with Rewrite addon — send request through proxy, verify request header added, response body modified, Content-Length recalculated
- [ ] T058 [P] Create `proxy/integration_test.go`: test proxy with NoCaching addon — verify cache headers stripped from both request and response passing through proxy
- [ ] T059 Create `proxy/integration_test.go`: test proxy with Throttle addon — verify latency injection adds at least configured ms to request; verify stream modifier wraps reader (throughput test optional due to timing sensitivity)

---

## Phase 12: Polish & Cross-Cutting Concerns

**Purpose**: Integration, consistency, and final validation

- [ ] T060 Verify addon registration order in `cmd/go-mitmproxy/main.go` matches R5: UpstreamCert → Log → BlockList → MapRemote → MapLocal → Rewrite → NoCaching → Throttle → WebAddon → Dumper
- [ ] T061 Add all new CLI flags to `-h` help text and update Config JSON file support in `cmd/go-mitmproxy/config.go` to include Rewrite, BlockList, Throttle, NoCaching fields
- [ ] T062 Build and verify Web UI: run `cd web/client && npm run build` to ensure all new React components compile and embed correctly
- [ ] T063 Run full test suite: `go test ./... -v -race` to verify no regressions or race conditions
- [ ] T064 Run quickstart.md verification: start proxy with all flags, test throttle/block/rewrite/no-caching via curl commands from quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **US1 Throttle (Phase 3)**: Depends on Phase 2 (config fields, registration order)
- **US2 Rewrite (Phase 4)**: Depends on Phase 2 (config fields, registration order)
- **US3 Repeat/Compose (Phase 5)**: Depends on Phase 2 (flow store, message types)
- **US4 Session/HAR (Phase 6)**: Depends on Phase 2 (flow store, message types)
- **US5 Block List (Phase 7)**: Depends on Phase 2 (config fields, registration order)
- **US6 No Caching (Phase 8)**: Depends on Phase 2 (config fields, registration order)
- **US7 Annotations (Phase 9)**: Depends on Phase 2 (annotation storage, message types)
- **US8 Statistics (Phase 10)**: Depends on Phase 2 (flow store, message types)
- **Integration Tests (Phase 11)**: Depends on US1, US2, US5, US6 being complete
- **Polish (Phase 12)**: Depends on all user stories and integration tests

### User Story Dependencies

- **US1 (Throttle)**: Independent - no dependencies on other stories
- **US2 (Rewrite)**: Independent - no dependencies on other stories
- **US3 (Repeat/Compose)**: Independent - uses flow store from Phase 2
- **US4 (Session/HAR)**: Independent - uses flow store from Phase 2
- **US5 (Block List)**: Independent - no dependencies on other stories
- **US6 (No Caching)**: Independent - no dependencies on other stories
- **US7 (Annotations)**: Independent - uses annotation storage from Phase 2
- **US8 (Statistics)**: Independent - uses flow store from Phase 2

### Within Each User Story

- Models/structs before logic
- Backend before frontend
- Core implementation before CLI integration

### Parallel Opportunities

- All Phase 1 tasks (T001-T005) can run in parallel
- T006, T008 in Phase 2 can run in parallel
- After Phase 2: US1, US2, US5, US6 can all start in parallel (different addon files)
- After Phase 2: US3, US4, US7, US8 can start in parallel (different files, share flow store)
- Within US1: T010 is independent
- Within US2: T015 is independent
- Within US3: T020, T021 are parallel; T023, T024 are parallel
- Within US4: T026, T027, T028 are parallel; T031 is parallel with backend
- Within US5: T033 is independent
- Within US6: T036 is independent
- Within US7: T039 is parallel with backend
- Within US8: T042, T043, T044, T045 are all parallel

---

## Implementation Strategy

### MVP First (US1 + US5 + US6)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete US1 (Throttle) - highest impact feature
4. Complete US5 (Block List) - quick win, low complexity
5. Complete US6 (No Caching) - quick win, low complexity
6. **STOP and VALIDATE**: Test all three via CLI + curl

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. US1 (Throttle) + US2 (Rewrite) → Two P1 features complete
3. US5 (Block List) + US6 (No Caching) → Quick P3 wins
4. US3 (Repeat/Compose) + US4 (Session/HAR) → P2 Web UI features
5. US7 (Annotations) + US8 (Statistics) → Polish features
6. Phase 11 → Final validation

### Parallel Team Strategy

With 2 developers after Phase 2:
- Developer A: US1 → US3 → US7
- Developer B: US2 → US4 → US8
- Both: US5 + US6 (small, either can pick up)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Each user story is independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- All addons follow MapRemote/MapLocal pattern: struct + JSON config + FromFile constructor

---

## Regression Test Matrix

### Existing Tests (Baseline)

| Package | Tests | Status |
| ------- | ----- | ------ |
| `addon/` | TestMapItemMatch, TestMapItemReplace | PASS |
| `cert/` | TestGetStorePath, TestNewCA | FAIL (pre-existing) |
| `internal/helper/` | TestMatchHost | PASS |
| `proxy/` | TestConnection, TestProxy, TestWebSocket* (20+ subtests) | PASS |
| `web/` | (none) | N/A |

### New Unit Tests

| Package | Test File | Tests | Validates |
| ------- | --------- | ----- | --------- |
| `addon/` | `throttle_test.go` | TestThrottleConfig, TestRateLimitedReader, TestThrottleHostMatch | US1: config loading, bandwidth limiting, host matching |
| `addon/` | `rewrite_test.go` | TestRewriteConfig, TestRewriteMatch, TestRewriteRequestActions, TestRewriteResponseActions, TestRewriteMultiRule | US2: config, URL matching, header/body/URL mods, rule chaining |
| `addon/` | `blocklist_test.go` | TestBlockListConfig, TestBlockListMatch, TestBlockListBehavior | US5: config, pattern matching, response injection |
| `addon/` | `nocaching_test.go` | TestNoCachingRequest, TestNoCachingResponse | US6: header stripping |
| `addon/` | `session_test.go` | TestSessionSaveLoad, TestSessionEdgeCases | US4: gzip round-trip, empty/nil/corrupted |
| `addon/` | `har_test.go` | TestHARExport, TestHARImport | US4: HAR 1.2 compliance |
| `web/` | `repeat_test.go` | TestRepeatHandler, TestComposeHandler, TestRepeatAdvanced | US3: request reconstruction, concurrency |

### Integration Tests

| Test File | Test | Validates |
| --------- | ---- | --------- |
| `proxy/integration_test.go` | TestBlockListPriority | Block > Map addon ordering |
| `proxy/integration_test.go` | TestRewriteProxy | End-to-end header/body rewrite |
| `proxy/integration_test.go` | TestNoCachingProxy | Cache header stripping through proxy |
| `proxy/integration_test.go` | TestThrottleProxy | Latency injection through proxy |

### Regression Checklist (run before each merge)

```bash
# Full regression: all unit + integration tests
go test ./... -v -race -count=1

# Quick smoke test: addon unit tests only
go test ./addon/ -v -count=1

# Frontend build verification
cd web/client && npm run build

# Manual smoke test
./go-mitmproxy -block_list blocklist.json -rewrite rewrite.json \
  -throttle throttle.json -no_caching
# Then: curl -x http://localhost:9080 https://httpbin.org/get
```
