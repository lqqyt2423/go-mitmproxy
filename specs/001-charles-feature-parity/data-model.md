# Data Model: Charles Feature Parity

**Date**: 2026-04-06
**Branch**: `001-charles-feature-parity`

## New Entities

### ThrottleConfig

Top-level configuration persisted as JSON.

```
ThrottleConfig
├── Enable       bool
├── Hosts        []string          // glob patterns; empty = all traffic
└── Profile      ThrottleProfile
```

### ThrottleProfile

```
ThrottleProfile
├── Name            string   // e.g., "3G", "4G/LTE", "Custom"
├── DownloadKbps    int64    // download bandwidth in Kbps
├── UploadKbps      int64    // upload bandwidth in Kbps
├── LatencyMs       int      // added round-trip latency in ms
└── PacketLossPercent float64 // 0.0 - 100.0
```

**Built-in presets**:

| Name | Download | Upload | Latency | Loss |
| ---- | -------- | ------ | ------- | ---- |
| 3G | 750 Kbps | 250 Kbps | 200ms | 0% |
| 4G/LTE | 12000 Kbps | 5000 Kbps | 50ms | 0% |
| WiFi (lossy) | 30000 Kbps | 15000 Kbps | 5ms | 1% |
| 100% Loss | 0 | 0 | 0 | 100% |

### RewriteRuleSet

Top-level configuration persisted as JSON.

```
RewriteRuleSet
├── Enable    bool
└── Items     []RewriteItem
```

### RewriteItem

A single rewrite rule matching a URL pattern.

```
RewriteItem
├── Enable    bool
├── Name      string             // user-friendly label
├── From      RewriteMatch       // URL matching criteria
└── Rules     []RewriteAction    // ordered list of modifications
```

### RewriteMatch

```
RewriteMatch
├── Protocol  string   // "http", "https", or "" (any)
├── Host      string   // glob pattern, e.g., "*.example.com"
├── Port      string   // port or "" (any)
├── Path      string   // glob pattern, e.g., "/api/*"
└── Method    []string // ["GET","POST"] or [] (any)
```

### RewriteAction

```
RewriteAction
├── Type       string   // "addHeader", "modifyHeader", "removeHeader",
│                       // "host", "path", "queryParam", "addQueryParam",
│                       // "removeQueryParam", "status", "body"
├── Target     string   // which: "request" or "response"
├── Name       string   // header name or param name (where applicable)
├── Value      string   // match value or new value
├── Replace    string   // replacement value
└── MatchMode  string   // "text" or "regex" (default: "text")
```

### BlockListConfig

Top-level configuration persisted as JSON.

```
BlockListConfig
├── Enable    bool
└── Items     []BlockRule
```

### BlockRule

```
BlockRule
├── Enable     bool
├── Host       string   // glob pattern
├── Path       string   // glob pattern, "" = all paths
├── Method     []string // [] = all methods
├── StatusCode int      // response status (default: 403)
└── Body       string   // response body text (default: "Blocked")
```

### SessionFile

Gzip-compressed JSON. Extension: `.gmps`.

```
SessionFile
├── Version      string           // "1.0"
├── CreatedAt    time.Time
├── ProxyVersion string           // go-mitmproxy version
├── Flows        []SessionFlow
```

### SessionFlow

```
SessionFlow
├── Id          string
├── Request     SessionRequest
├── Response    SessionResponse  // nullable
├── WebSocket   []WebSocketMessage // nullable
├── SSE         []SSEEvent         // nullable
├── Annotations FlowAnnotation     // nullable
├── Timing      TimingData         // nullable
└── StartTime   time.Time
```

### SessionRequest / SessionResponse

```
SessionRequest
├── Method   string
├── URL      string
├── Proto    string
├── Header   map[string][]string
└── Body     string              // base64 encoded

SessionResponse
├── StatusCode int
├── Header     map[string][]string
└── Body       string              // base64 encoded
```

### FlowAnnotation

In-memory only (persists in session file, not standalone config).

```
FlowAnnotation
├── Color    string   // "red","blue","green","yellow","purple",""
└── Comment  string
```

### TimingData

Populated during flow lifecycle, in-memory.

```
TimingData
├── DnsMs       int64   // DNS resolution time
├── ConnectMs   int64   // TCP connection time
├── TlsMs       int64   // TLS handshake time
├── SendMs      int64   // Request send time
├── WaitMs      int64   // TTFB (time to first byte)
└── ReceiveMs   int64   // Content download time
```

## Modified Entities

### Flow (proxy/flow.go)

Add fields:

```
Flow (existing)
├── ...existing fields...
├── Annotation  *FlowAnnotation  // new: highlight + comment
└── Timing      *TimingData      // new: per-phase timing
```

### Config (cmd/go-mitmproxy/config.go)

Add fields:

```
Config (existing)
├── ...existing fields...
├── Rewrite     string   // new: rewrite config file path
├── BlockList   string   // new: block list config file path
├── Throttle    string   // new: throttle config file path
└── NoCaching   bool     // new: enable no-caching mode
```

## Entity Relationships

```
Proxy
├── registers → BlockListAddon (config: BlockListConfig)
├── registers → MapRemote (existing)
├── registers → MapLocal (existing)
├── registers → RewriteAddon (config: RewriteRuleSet)
├── registers → NoCachingAddon (no config)
├── registers → ThrottleAddon (config: ThrottleConfig)
└── registers → WebAddon
                ├── manages → FlowAnnotation (per flow)
                ├── handles → Repeat/Compose requests
                ├── handles → Session Save/Load
                └── handles → HAR Export/Import

Flow
├── has → Request
├── has → Response
├── has → *FlowAnnotation (optional)
└── has → *TimingData (optional)
```
