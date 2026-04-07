# CLI Flags Contract

New flags for `go-mitmproxy` command:

```
-rewrite string
    rewrite rules config filename (JSON)

-block_list string
    block list config filename (JSON)

-throttle string
    throttle config filename (JSON)

-no_caching
    strip cache headers from requests and responses
```

These flags follow the same pattern as existing `-map_remote` and
`-map_local` flags: they accept a path to a JSON config file.

## Config File Formats

### Throttle Config (`-throttle`)

```json
{
  "Enable": true,
  "Hosts": ["*.example.com"],
  "Profile": {
    "Name": "3G",
    "DownloadKbps": 750,
    "UploadKbps": 250,
    "LatencyMs": 200,
    "PacketLossPercent": 0
  }
}
```

### Rewrite Config (`-rewrite`)

```json
{
  "Enable": true,
  "Items": [
    {
      "Enable": true,
      "Name": "Add debug header",
      "From": {
        "Protocol": "https",
        "Host": "api.example.com",
        "Path": "/v1/*",
        "Method": ["GET", "POST"]
      },
      "Rules": [
        {
          "Type": "addHeader",
          "Target": "request",
          "Name": "X-Debug",
          "Value": "true"
        },
        {
          "Type": "body",
          "Target": "response",
          "Value": "oldValue",
          "Replace": "newValue",
          "MatchMode": "text"
        }
      ]
    }
  ]
}
```

### Block List Config (`-block_list`)

```json
{
  "Enable": true,
  "Items": [
    {
      "Enable": true,
      "Host": "ads.example.com",
      "Path": "",
      "Method": [],
      "StatusCode": 403,
      "Body": "Blocked by go-mitmproxy"
    }
  ]
}
```
