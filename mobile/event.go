package mobile

// EventHandler is the callback interface that Swift/Kotlin implements to receive proxy events.
// gomobile will generate platform-specific protocol/interface from this.
type EventHandler interface {
	// OnFlowRequest is called when a complete HTTP request has been read.
	// flowJSON contains flow metadata (id, method, url, headers, bodyLen) as JSON.
	OnFlowRequest(flowJSON string)

	// OnFlowResponse is called when a complete HTTP response has been read.
	// flowJSON contains flow metadata (id, statusCode, headers, bodyLen, durationMs) as JSON.
	OnFlowResponse(flowJSON string)

	// OnFlowError is called when an HTTP request fails.
	OnFlowError(flowID string, errMsg string)

	// OnWebSocketMessage is called for each WebSocket message.
	OnWebSocketMessage(flowID string, msgJSON string)

	// OnSSEEvent is called for each Server-Sent Event.
	OnSSEEvent(flowID string, eventJSON string)

	// OnStateChanged is called when proxy state changes.
	// state: "starting", "running", "stopping", "stopped", "error"
	OnStateChanged(state string, message string)
}
