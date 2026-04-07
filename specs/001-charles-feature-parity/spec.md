# Feature Specification: Charles Proxy Feature Parity

**Feature Branch**: `001-charles-feature-parity`
**Created**: 2026-04-06
**Status**: Draft
**Input**: User description: "比较当前项目和charles的差距，列出功能差异，并思考怎么实现"

## Clarifications

### Session 2026-04-06

- Q: Should throttle profiles, rewrite rules, and block list rules persist across proxy restarts? → A: Yes, persist all to JSON config files, consistent with existing MapRemote/MapLocal pattern.
- Q: When multiple features (Block List, Map Remote/Local, Rewrite) match the same request, what is the execution order? → A: Fixed priority: Block List → Map Remote/Local → Rewrite (Charles-compatible).
- Q: Should rewrite body matching support plain text only or also regex? → A: Support both plain text and regex, with a mode field in each rule to distinguish.

## Current State Analysis

### Features go-mitmproxy Already Has (Parity Achieved)

| Feature | Charles | go-mitmproxy | Notes |
| ------- | ------- | ------------ | ----- |
| SSL Proxying | ✅ | ✅ | MITM with per-host cert generation |
| HTTP/2 | ✅ | ✅ | Full HTTP/2 support |
| WebSocket | ✅ | ✅ | Capture + display messages |
| Map Remote | ✅ | ✅ | URL rewriting with glob patterns |
| Map Local | ✅ | ✅ | Serve local files for matched URLs |
| Breakpoints | ✅ | ✅ | Pause, edit, continue/drop requests |
| Request/Response Inspection | ✅ | ✅ | Headers, body, JSON formatting |
| Content Decompression | ✅ | ✅ | gzip, deflate, brotli |
| Upstream Proxy | ✅ | ✅ | Chain through another proxy |
| Proxy Authentication | ✅ | ✅ | Basic auth with multi-user support |
| Allow/Ignore Hosts | ✅ | ✅ | Host-based filtering |
| Copy as cURL | ✅ | ✅ | Export request as curl command |
| Advanced Filtering | ✅ | ✅ | Regex, boolean logic, multiple scopes |
| Web Interface | ❌ (native) | ✅ | React-based web UI |
| SSE Support | ❌ | ✅ | go-mitmproxy is ahead here |
| Plugin/Addon System | Limited | ✅ | Go-based addon interface |
| Mobile Framework | ❌ | ✅ | gomobile xcframework for iOS/macOS |

### Features Missing (Gap Analysis)

| # | Charles Feature | Impact | Complexity |
| - | -------------- | ------ | ---------- |
| 1 | Throttling (bandwidth/latency simulation) | High | Medium |
| 2 | Rewrite Tool (header/body/URL rules) | High | Medium |
| 3 | Repeat Request | High | Low |
| 4 | Session Save/Load | High | Medium |
| 5 | HAR Export/Import | High | Medium |
| 6 | Compose (craft custom requests) | Medium | Medium |
| 7 | Block List (deny requests, return error) | Medium | Low |
| 8 | No Caching (strip cache headers) | Medium | Low |
| 9 | Reverse Proxy Mode | Medium | Medium |
| 10 | DNS Spoofing | Medium | High |
| 11 | Highlight & Comment Flows | Low | Low |
| 12 | Summary Statistics | Low | Low |
| 13 | Timing Chart View | Low | Medium |
| 14 | Mirror (duplicate requests) | Low | Low |
| 15 | Client Certificates | Low | Medium |
| 16 | SOCKS Proxy | Low | Medium |
| 17 | Transparent Proxy Mode | Low | High |

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Throttling: Simulate Slow Networks (Priority: P1)

As a mobile/web developer, I want to simulate slow network conditions
(3G, 4G, high latency) so I can test how my application handles
degraded connectivity without switching to a real slow network.

**Why this priority**: Throttling is one of the most-used Charles
features. Developers routinely need to test loading states, timeouts,
and progressive loading under constrained bandwidth. This is a major
reason users choose Charles over free alternatives.

**Independent Test**: Configure throttling to "3G" preset, load a
page through the proxy, and verify the page load time increases
proportionally to the configured bandwidth limit.

**Acceptance Scenarios**:

1. **Given** the proxy is running with throttling disabled,
   **When** the user enables throttling with "3G" preset via Web UI,
   **Then** all proxied traffic is rate-limited to ~750 Kbps download
   / ~250 Kbps upload with ~200ms added latency.

2. **Given** throttling is enabled globally,
   **When** the user adds a host to the throttling whitelist,
   **Then** only that host's traffic is throttled; other traffic
   flows at full speed.

3. **Given** throttling is active,
   **When** the user switches preset from "3G" to "Custom" with
   specific values (bandwidth: 1 Mbps, latency: 100ms),
   **Then** the new settings apply immediately to subsequent requests.

4. **Given** throttling is active,
   **When** the user disables throttling,
   **Then** all traffic resumes at full speed immediately.

---

### User Story 2 - Rewrite Tool: Automated Request/Response Modification (Priority: P1)

As a developer, I want to define persistent rewrite rules that
automatically modify headers, URLs, query parameters, or response
bodies for matched requests, so I can test API changes without
modifying server code.

**Why this priority**: The Rewrite tool is Charles's most powerful
debugging feature. Unlike breakpoints (which require manual
intervention per request), rewrite rules apply automatically and
persistently. MapRemote and MapLocal cover URL rewriting and file
serving, but many workflows require header injection, status code
changes, or body text replacement.

**Independent Test**: Create a rewrite rule that adds a custom header
to all requests matching `*.example.com`, then verify the header
appears in traffic captured by the proxy.

**Acceptance Scenarios**:

1. **Given** no rewrite rules exist,
   **When** the user creates a rule to add header
   `X-Debug: true` to all requests matching `api.example.com/*`,
   **Then** every request to that host includes the header.

2. **Given** a rewrite rule exists for modifying response body,
   **When** a response matches the rule's URL pattern,
   **Then** the specified text replacement is applied to the body
   before it reaches the client.

3. **Given** multiple rewrite rules exist,
   **When** a request matches more than one rule,
   **Then** all matching rules are applied in their defined order.

4. **Given** rewrite rules are configured,
   **When** the user exports/imports the rule set as a JSON file,
   **Then** the rules persist and can be shared across environments.

---

### User Story 3 - Repeat & Compose Requests (Priority: P2)

As a developer debugging an API, I want to repeat a captured request
(with optional modifications) or compose a new request from scratch,
so I can test endpoints without leaving the proxy tool.

**Why this priority**: Repeat and Compose are frequently used for
quick API testing. They reduce context-switching between tools
(no need to open Postman or curl for quick checks).

**Independent Test**: Capture a POST request in the proxy, click
"Repeat", verify the same request is sent again and the new
response is captured separately.

**Acceptance Scenarios**:

1. **Given** a flow is selected in the Web UI,
   **When** the user clicks "Repeat",
   **Then** the exact same request is sent and a new flow appears
   in the flow list with the new response.

2. **Given** the user clicks "Compose",
   **When** the user fills in method, URL, headers, and body,
   **Then** the custom request is sent through the proxy and
   captured as a new flow.

3. **Given** a flow is selected,
   **When** the user clicks "Repeat Advanced" and specifies
   count=10 and concurrency=3,
   **Then** 10 copies of the request are sent with up to 3
   concurrent, and results are shown with timing statistics.

---

### User Story 4 - Session Save/Load & HAR Export (Priority: P2)

As a developer, I want to save captured traffic sessions and export
them in standard formats (HAR), so I can share debug sessions with
teammates, attach them to bug reports, or analyze them later.

**Why this priority**: Session persistence and HAR export are table
stakes for professional proxy tools. Without them, all captured data
is lost when the proxy restarts.

**Independent Test**: Capture several flows, save the session, restart
the proxy, load the session, and verify all flows are restored with
full request/response data.

**Acceptance Scenarios**:

1. **Given** the proxy has captured multiple flows,
   **When** the user clicks "Save Session",
   **Then** all flows (including request/response headers and bodies)
   are saved to a file.

2. **Given** a saved session file exists,
   **When** the user loads it on a fresh proxy instance,
   **Then** all flows appear in the Web UI exactly as captured.

3. **Given** the proxy has captured flows,
   **When** the user clicks "Export HAR",
   **Then** a valid HAR 1.2 JSON file is generated that can be
   opened in Chrome DevTools or other HAR viewers.

4. **Given** the user has a HAR file from another tool,
   **When** the user imports it into go-mitmproxy,
   **Then** the flows appear in the Web UI for inspection.

---

### User Story 5 - Block List: Deny Specific Requests (Priority: P3)

As a developer, I want to block specific URLs or hosts and return
a configurable error response, so I can test how my application
handles unavailable services or blocked resources.

**Why this priority**: Blocking is simpler than Map Local but very
useful for testing error handling. It's a quick way to simulate
service outages.

**Independent Test**: Add `ads.example.com` to the block list,
make a request to that host through the proxy, and verify a 403
response is returned without contacting the upstream server.

**Acceptance Scenarios**:

1. **Given** a block list rule exists for `ads.example.com`,
   **When** a request to that host passes through the proxy,
   **Then** the proxy returns a 403 response without connecting
   to the upstream server.

2. **Given** a block list rule with custom status code 503,
   **When** a matching request arrives,
   **Then** the proxy returns 503 with a configurable body message.

3. **Given** a host is in the block list,
   **When** the user removes it,
   **Then** subsequent requests to that host are proxied normally.

---

### User Story 6 - No Caching: Strip Cache Headers (Priority: P3)

As a developer, I want to strip all cache-related headers from
requests and responses, so I always see fresh content from the
server during development.

**Why this priority**: Simple to implement and commonly needed
during web development when stale cached content causes confusion.

**Independent Test**: Enable "No Caching", make a request to a
cacheable resource, verify all cache headers (Cache-Control, ETag,
If-Modified-Since, etc.) are stripped.

**Acceptance Scenarios**:

1. **Given** No Caching is enabled,
   **When** a request passes through the proxy,
   **Then** headers `If-Modified-Since`, `If-None-Match`,
   `Cache-Control`, `Pragma` are removed from the request.

2. **Given** No Caching is enabled,
   **When** a response passes through the proxy,
   **Then** headers `Cache-Control`, `Expires`, `ETag`,
   `Last-Modified` are removed or overridden with no-cache
   directives.

---

### User Story 7 - Highlight & Comment Flows (Priority: P3)

As a developer inspecting traffic, I want to highlight specific
flows with colors and add text comments, so I can mark interesting
requests during a debugging session.

**Why this priority**: Quality-of-life feature for complex debugging
sessions with hundreds of flows.

**Independent Test**: Right-click a flow, select a highlight color,
add a comment, verify the flow displays with the color and the
comment is visible in the detail view.

**Acceptance Scenarios**:

1. **Given** a flow is displayed in the Web UI,
   **When** the user right-clicks and selects "Highlight" with a color,
   **Then** the flow row displays with the chosen color indicator.

2. **Given** a flow is selected,
   **When** the user adds a comment "Bug reproduction - see ticket #123",
   **Then** the comment is visible in the flow detail view and
   persists during the session.

---

### User Story 8 - Summary Statistics & Timing (Priority: P4)

As a developer analyzing traffic patterns, I want to see summary
statistics (total requests, response code distribution, total data
transferred) and per-request timing breakdowns, so I can identify
performance bottlenecks.

**Why this priority**: Nice-to-have analytics. Most developers use
browser DevTools for timing analysis, but having it in the proxy
is convenient for cross-application traffic.

**Independent Test**: Capture 50+ flows, open the statistics view,
verify request counts, status code distribution, and data volume
are displayed correctly.

**Acceptance Scenarios**:

1. **Given** the proxy has captured flows,
   **When** the user opens the Statistics view,
   **Then** total request count, status code distribution
   (2xx/3xx/4xx/5xx), total bytes sent/received, and average
   response time are shown.

2. **Given** a flow is selected,
   **When** the user opens the Timing tab,
   **Then** a waterfall chart shows DNS lookup, TCP connect,
   TLS handshake, request sent, waiting (TTFB), and content
   download phases.

---

### Edge Cases

- What happens when throttling is enabled and a WebSocket connection
  is established? Throttling MUST apply to WebSocket frames as well.
- What happens when a rewrite rule modifies Content-Length but not
  the body? The proxy MUST recalculate Content-Length automatically.
- What happens when a request matches both Block List and Map Remote?
  Block List takes precedence; the request is denied and Map Remote
  is not executed.
- What happens when a saved session file is corrupted? The loader
  MUST display an error and not crash.
- What happens when HAR export encounters binary response bodies?
  Bodies MUST be base64-encoded per the HAR 1.2 spec.
- What happens when "Repeat Advanced" sends 1000 requests? The UI
  MUST remain responsive and display a progress indicator.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support bandwidth throttling with configurable
  download speed, upload speed, and added latency per connection.
- **FR-002**: System MUST provide throttling presets for common network
  profiles (3G, 4G/LTE, Wi-Fi with packet loss, 100% packet loss).
- **FR-003**: System MUST support per-host throttling configuration
  (throttle only specific hosts or all traffic).
- **FR-004**: System MUST support rewrite rules that can modify
  request/response headers, URL paths, query parameters, body
  content, and status codes.
- **FR-005**: System MUST apply rewrite rules automatically to all
  matching traffic without user intervention per request.
- **FR-006**: System MUST support repeating a captured request and
  displaying the new response as a separate flow.
- **FR-007**: System MUST support composing new HTTP requests from
  scratch via the Web UI.
- **FR-008**: System MUST support saving all captured flows to a
  session file and loading them back.
- **FR-009**: System MUST support exporting captured flows in
  HAR 1.2 format.
- **FR-010**: System MUST support a block list that returns
  configurable error responses without contacting the upstream server.
- **FR-011**: System MUST support a "No Caching" mode that strips
  cache-related headers from requests and responses.
- **FR-012**: System MUST support highlighting flows with colors
  in the Web UI.
- **FR-013**: System MUST support adding text comments to
  individual flows.
- **FR-014**: System MUST display summary statistics (request count,
  status code distribution, data volume) for captured traffic.
- **FR-015**: System MUST provide per-request timing breakdown
  showing connection phases (DNS, TCP, TLS, TTFB, transfer).
- **FR-016**: System MUST support "Repeat Advanced" with configurable
  request count and concurrency level.
- **FR-017**: Rewrite rules and block list rules MUST be configurable
  via both the Web UI and JSON configuration files.
- **FR-018**: All new features MUST be accessible via the addon
  interface, allowing programmatic use when go-mitmproxy is imported
  as a library.
- **FR-019**: Throttle profiles, rewrite rule sets, and block list
  rules MUST persist to JSON configuration files on disk and be
  loaded automatically on proxy startup, consistent with the existing
  MapRemote/MapLocal configuration file pattern.
- **FR-020**: When multiple addons match the same request, execution
  MUST follow fixed priority: Block List (highest) → Map Remote/Local
  → Rewrite (lowest). Blocked requests MUST NOT be processed by
  subsequent addons.
- **FR-021**: Rewrite rules for body modification MUST support both
  plain text exact match and regex pattern matching, with a mode
  field in each rule to distinguish the two.

### Key Entities

- **ThrottleProfile**: Network simulation configuration with download
  speed, upload speed, latency, and packet loss percentage.
- **RewriteRule**: Pattern match + set of modifications to apply
  (target: header/body/URL/status, match pattern, replacement).
  Body rules include a match mode field: "text" for exact string
  or "regex" for regular expression patterns.
- **RewriteRuleSet**: Ordered collection of RewriteRules with
  enable/disable toggle. Persisted as a JSON configuration file.
- **Session**: Serializable snapshot of all captured flows with
  metadata (capture start time, proxy configuration, flow count).
- **BlockRule**: URL/host pattern with associated error response
  (status code, body, headers).
- **FlowAnnotation**: Color highlight and text comment for a flow.
- **TimingData**: Per-flow breakdown of connection phase durations.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can simulate 3G network conditions and observe
  proportionally slower page loads (within 10% of configured
  bandwidth limits).
- **SC-002**: Users can define a rewrite rule and see it applied to
  matching traffic within the same session, without restarting
  the proxy.
- **SC-003**: Users can repeat any captured request in under 3 clicks.
- **SC-004**: Users can save a 1000-flow session and reload it with
  all request/response data intact in under 5 seconds.
- **SC-005**: Exported HAR files open successfully in Chrome DevTools
  and Firefox HAR viewer without errors.
- **SC-006**: Users can block a host and verify the block is effective
  within 1 second of rule creation.
- **SC-007**: All new features are available as addons that can be
  used programmatically without the Web UI.
- **SC-008**: Throttling adds less than 5ms overhead beyond the
  configured latency when processing individual requests.

## Assumptions

- Throttling is implemented at the connection level using rate
  limiting, not by delaying entire responses.
- Session files use a compressed format to handle large response
  bodies efficiently.
- HAR export follows the HAR 1.2 specification.
- Rewrite rules use the same glob pattern matching as Map Remote
  (tidwall/match) for URL matching consistency.
- "Repeat Advanced" is capped at a reasonable maximum (e.g., 10,000
  requests) to prevent accidental DoS.
- Mobile framework (gomobile) support for new features is deferred
  to a follow-up phase — initial implementation targets CLI + Web UI.
- DNS Spoofing and Transparent Proxy Mode are excluded from this
  scope as they require OS-level integration that varies significantly
  by platform.
- Reverse Proxy Mode, Client Certificates, SOCKS Proxy, and Mirror
  are excluded from this scope and will be addressed in future
  feature specifications.
