package flow

import (
	"net/http"
	"net/url"
	"time"

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

type Addon interface {
	// HTTP request headers were successfully read. At this point, the body is empty.
	Requestheaders(*Flow)

	// The full HTTP request has been read.
	Request(*Flow)

	// HTTP response headers were successfully read. At this point, the body is empty.
	Responseheaders(*Flow)

	// The full HTTP response has been read.
	Response(*Flow)
}

// BaseAddon do nothing
type BaseAddon struct{}

func (addon *BaseAddon) Requestheaders(*Flow)  {}
func (addon *BaseAddon) Request(*Flow)         {}
func (addon *BaseAddon) Responseheaders(*Flow) {}
func (addon *BaseAddon) Response(*Flow)        {}

// LogAddon log http record
type LogAddon struct {
	BaseAddon
}

func (addon *LogAddon) Requestheaders(flo *Flow) {
	log := log.WithField("in", "LogAddon")
	start := time.Now()
	go func() {
		<-flo.Done()
		var StatusCode int
		if flo.Response != nil {
			StatusCode = flo.Response.StatusCode
		}
		var contentLen int
		if flo.Response != nil && flo.Response.Body != nil {
			contentLen = len(flo.Response.Body)
		}
		log.Infof("%v %v %v %v - %v ms\n", flo.Request.Method, flo.Request.URL.String(), StatusCode, contentLen, time.Since(start).Milliseconds())
	}()
}
