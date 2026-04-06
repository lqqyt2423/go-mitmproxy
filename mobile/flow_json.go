package mobile

import (
	"encoding/json"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

type flowRequestJSON struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Proto   string            `json:"proto"`
	Headers map[string]string `json:"headers"`
	BodyLen int               `json:"bodyLen"`
	Time    string            `json:"time"`
}

type flowResponseJSON struct {
	ID         string            `json:"id"`
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	BodyLen    int               `json:"bodyLen"`
	DurationMs int64            `json:"durationMs"`
}

type webSocketMsgJSON struct {
	Type       int    `json:"type"`
	Content    string `json:"content"`
	FromClient bool   `json:"fromClient"`
	Timestamp  string `json:"timestamp"`
}

type sseEventJSON struct {
	ID    string `json:"id,omitempty"`
	Event string `json:"event,omitempty"`
	Data  string `json:"data"`
	Time  string `json:"timestamp"`
}

// flattenHeaders converts http.Header (map[string][]string) to map[string]string,
// taking the first value for each key.
func flattenHeaders(h map[string][]string) map[string]string {
	result := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}

func marshalFlowRequest(f *proxy.Flow) (string, error) {
	bodyLen := 0
	if f.Request.Body != nil {
		bodyLen = len(f.Request.Body)
	}
	j := flowRequestJSON{
		ID:      f.Id.String(),
		Method:  f.Request.Method,
		URL:     f.Request.URL.String(),
		Proto:   f.Request.Proto,
		Headers: flattenHeaders(f.Request.Header),
		BodyLen: bodyLen,
		Time:    f.StartTime.Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(j)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func marshalFlowResponse(f *proxy.Flow) (string, error) {
	bodyLen := 0
	headers := make(map[string]string)
	statusCode := 0
	if f.Response != nil {
		statusCode = f.Response.StatusCode
		headers = flattenHeaders(f.Response.Header)
		if f.Response.Body != nil {
			bodyLen = len(f.Response.Body)
		}
	}
	j := flowResponseJSON{
		ID:         f.Id.String(),
		StatusCode: statusCode,
		Headers:    headers,
		BodyLen:    bodyLen,
		DurationMs: time.Since(f.StartTime).Milliseconds(),
	}
	data, err := json.Marshal(j)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func marshalWebSocketMessage(f *proxy.Flow) (string, error) {
	msgs := f.WebScoket.Messages
	if len(msgs) == 0 {
		return "{}", nil
	}
	msg := msgs[len(msgs)-1]
	j := webSocketMsgJSON{
		Type:       msg.Type,
		Content:    string(msg.Content),
		FromClient: msg.FromClient,
		Timestamp:  msg.Timestamp.Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(j)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func marshalSSEEvent(f *proxy.Flow) (string, error) {
	events := f.SSE.Events
	if len(events) == 0 {
		return "{}", nil
	}
	event := events[len(events)-1]
	j := sseEventJSON{
		ID:    event.ID,
		Event: event.Event,
		Data:  event.Data,
		Time:  event.Time.Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(j)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
