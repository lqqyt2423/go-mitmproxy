package addon

import (
	"time"

	"github.com/lqqyt2423/go-mitmproxy/connection"
	"github.com/lqqyt2423/go-mitmproxy/flow"
	_log "github.com/sirupsen/logrus"
)

var log = _log.WithField("at", "addon")

type Addon interface {
	// A client has connected to mitmproxy. Note that a connection can correspond to multiple HTTP requests.
	ClientConnected(*connection.Client)

	// A client connection has been closed (either by us or the client).
	ClientDisconnected(*connection.Client)

	// HTTP request headers were successfully read. At this point, the body is empty.
	Requestheaders(*flow.Flow)

	// The full HTTP request has been read.
	Request(*flow.Flow)

	// HTTP response headers were successfully read. At this point, the body is empty.
	Responseheaders(*flow.Flow)

	// The full HTTP response has been read.
	Response(*flow.Flow)
}

// Base do nothing
type Base struct{}

func (addon *Base) ClientConnected(*connection.Client)    {}
func (addon *Base) ClientDisconnected(*connection.Client) {}

func (addon *Base) Requestheaders(*flow.Flow)  {}
func (addon *Base) Request(*flow.Flow)         {}
func (addon *Base) Responseheaders(*flow.Flow) {}
func (addon *Base) Response(*flow.Flow)        {}

// Log log http record
type Log struct {
	Base
}

func (addon *Log) ClientConnected(client *connection.Client) {
	log.Infof("%v client connect\n", client.Conn.RemoteAddr())
}

func (addon *Log) ClientDisconnected(client *connection.Client) {
	log.Infof("%v client disconnect\n", client.Conn.RemoteAddr())
}

func (addon *Log) Requestheaders(f *flow.Flow) {
	log := log.WithField("in", "Log")
	start := time.Now()
	go func() {
		<-f.Done()
		var StatusCode int
		if f.Response != nil {
			StatusCode = f.Response.StatusCode
		}
		var contentLen int
		if f.Response != nil && f.Response.Body != nil {
			contentLen = len(f.Response.Body)
		}
		log.Infof("%v %v %v %v - %v ms\n", f.Request.Method, f.Request.URL.String(), StatusCode, contentLen, time.Since(start).Milliseconds())
	}()
}
