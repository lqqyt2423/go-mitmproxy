# WebSocket Protocol Extensions

Extends the existing binary message protocol (version 2).

## Existing Message Types (0-32)

Unchanged. See `web/message.go` for reference.

## New Message Types

### Repeat & Compose (40-42)

**Type 40: RepeatRequest** (client → server)
```
version(1) + type(1) + flowId(36)
```
Server re-sends the request from the identified flow and creates a
new flow with the response.

**Type 41: ComposeRequest** (client → server)
```
version(1) + type(1) + id(36) + headerLen(4) + headerJSON + bodyLen(4) + body
```
headerJSON format:
```json
{
  "method": "POST",
  "url": "https://api.example.com/test",
  "header": {"Content-Type": ["application/json"]}
}
```

**Type 42: RepeatAdvanced** (client → server)
```
version(1) + type(1) + flowId(36) + configJSON
```
configJSON format:
```json
{"count": 10, "concurrency": 3}
```
Server responds with Type 70 (Statistics) containing aggregated
results when complete.

### Session Management (50-53)

**Type 50: SaveSession** (client → server)
```
version(1) + type(1) + filenameJSON
```
Server saves all flows to the specified file path as `.gmps`.

**Type 51: LoadSession** (client → server)
```
version(1) + type(1) + filenameJSON
```
Server loads session and pushes all flows via standard Type 1-4
messages.

**Type 52: ExportHAR** (client → server)
```
version(1) + type(1) + filenameJSON
```
Server exports all flows as HAR 1.2 to the specified file.

**Type 53: ImportHAR** (client → server)
```
version(1) + type(1) + filenameJSON
```
Server imports HAR file and pushes flows via standard messages.

### Annotations (60)

**Type 60: SetAnnotation** (client → server)
```
version(1) + type(1) + flowId(36) + annotationJSON
```
annotationJSON format:
```json
{"color": "red", "comment": "Bug reproduction"}
```
Server stores annotation and broadcasts to all connected clients.

### Statistics (70-71)

**Type 70: Statistics** (server → client)
```
version(1) + type(1) + statsJSON
```
statsJSON format:
```json
{
  "totalFlows": 150,
  "statusCodes": {"200": 120, "404": 15, "500": 10, "302": 5},
  "totalBytesSent": 1048576,
  "totalBytesReceived": 5242880,
  "avgResponseMs": 245
}
```

**Type 71: TimingData** (server → client)
```
version(1) + type(1) + flowId(36) + timingJSON
```
timingJSON format:
```json
{
  "dnsMs": 12,
  "connectMs": 35,
  "tlsMs": 48,
  "sendMs": 2,
  "waitMs": 180,
  "receiveMs": 25
}
```
