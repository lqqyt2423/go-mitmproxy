package flow

import (
	"net/http"
	"net/url"

	_log "github.com/sirupsen/logrus"
)

var log = _log.WithField("at", "flow")

type Request struct {
	Method string
	URL    *url.URL
	Proto  string
	Header http.Header
	Body   []byte
}

type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

type Flow struct {
	*Request
	*Response

	// https://docs.mitmproxy.org/stable/overview-features/#streaming
	// 如果为 true，则不缓冲 Request.Body 和 Response.Body，且不进入之后的 Addon.Request 和 Addon.Response
	Stream bool
	done   chan struct{}
}

func NewFlow() *Flow {
	return &Flow{done: make(chan struct{})}
}

func (f *Flow) Done() <-chan struct{} {
	return f.done
}

func (f *Flow) Finish() {
	close(f.done)
}
