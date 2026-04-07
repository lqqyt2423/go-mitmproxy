# Quickstart: Charles Feature Parity

## Prerequisites

- Go 1.21+
- Node.js 18+ (for Web UI development)
- Existing go-mitmproxy build working (`make mitmproxy`)

## Using New Features

### Throttling

Create a throttle config file `throttle.json`:
```json
{
  "Enable": true,
  "Hosts": [],
  "Profile": {
    "Name": "3G",
    "DownloadKbps": 750,
    "UploadKbps": 250,
    "LatencyMs": 200,
    "PacketLossPercent": 0
  }
}
```

Run with throttling:
```bash
go-mitmproxy -throttle throttle.json
```

Or enable/disable via Web UI at http://localhost:9081.

### Rewrite Rules

Create `rewrite.json`:
```json
{
  "Enable": true,
  "Items": [
    {
      "Enable": true,
      "Name": "Add debug header to API",
      "From": {
        "Host": "api.example.com",
        "Path": "/v1/*"
      },
      "Rules": [
        {
          "Type": "addHeader",
          "Target": "request",
          "Name": "X-Debug",
          "Value": "true"
        }
      ]
    }
  ]
}
```

Run:
```bash
go-mitmproxy -rewrite rewrite.json
```

### Block List

Create `blocklist.json`:
```json
{
  "Enable": true,
  "Items": [
    {
      "Enable": true,
      "Host": "ads.example.com",
      "StatusCode": 403,
      "Body": "Blocked"
    }
  ]
}
```

Run:
```bash
go-mitmproxy -block_list blocklist.json
```

### No Caching

```bash
go-mitmproxy -no_caching
```

### Combined

```bash
go-mitmproxy \
  -throttle throttle.json \
  -rewrite rewrite.json \
  -block_list blocklist.json \
  -no_caching
```

### Web UI Features

After starting the proxy, open http://localhost:9081:

- **Repeat**: Right-click a flow → "Repeat" to re-send the request
- **Compose**: Click "Compose" button to craft a new request
- **Save Session**: File → Save Session (saves as `.gmps`)
- **Export HAR**: File → Export HAR
- **Highlight**: Right-click a flow → "Highlight" → select color
- **Comment**: Click flow detail → add comment in the annotation field
- **Statistics**: View → Statistics panel

## Verification

```bash
# Build
make mitmproxy

# Run tests
make test

# Start with all features
./go-mitmproxy -throttle throttle.json -rewrite rewrite.json \
  -block_list blocklist.json -no_caching

# Test throttling: should be slow
curl -x http://localhost:9080 https://httpbin.org/get

# Test blocking: should return 403
curl -x http://localhost:9080 https://ads.example.com/

# Test rewrite: should have X-Debug header
curl -x http://localhost:9080 https://api.example.com/v1/test -v

# Test no caching: response should have no cache headers
curl -x http://localhost:9080 https://httpbin.org/cache/60 -v
```

## Using as a Library

```go
package main

import (
    "log"
    "github.com/lqqyt2423/go-mitmproxy/addon"
    "github.com/lqqyt2423/go-mitmproxy/proxy"
)

func main() {
    opts := &proxy.Options{
        Addr:              ":9080",
        StreamLargeBodies: 1024 * 1024 * 5,
    }

    p, err := proxy.NewProxy(opts)
    if err != nil {
        log.Fatal(err)
    }

    // Add block list
    blockList := addon.NewBlockList()
    blockList.AddRule("ads.example.com", "", 403, "Blocked")
    p.AddAddon(blockList)

    // Add rewrite
    rewrite := addon.NewRewrite()
    rewrite.AddHeaderRule("api.example.com/*", "request",
        "addHeader", "X-Debug", "true")
    p.AddAddon(rewrite)

    // Add throttle
    throttle := addon.NewThrottle(addon.Preset3G)
    p.AddAddon(throttle)

    // Add no caching
    p.AddAddon(addon.NewNoCaching())

    log.Fatal(p.Start())
}
```
