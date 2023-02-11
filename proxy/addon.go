package proxy

import (
	"io"
	"time"

	log "github.com/sirupsen/logrus"
)

type Addon interface {
	// A client has connected to mitmproxy. Note that a connection can correspond to multiple HTTP requests.
	ClientConnected(*ClientConn)

	// A client connection has been closed (either by us or the client).
	ClientDisconnected(*ClientConn)

	// Mitmproxy has connected to a server.
	ServerConnected(*ConnContext)

	// A server connection has been closed (either by us or the server).
	ServerDisconnected(*ConnContext)

	// The TLS handshake with the server has been completed successfully.
	TlsEstablishedServer(*ConnContext)

	// HTTP request headers were successfully read. At this point, the body is empty.
	Requestheaders(*Flow)

	// The full HTTP request has been read.
	Request(*Flow)

	// HTTP response headers were successfully read. At this point, the body is empty.
	Responseheaders(*Flow)

	// The full HTTP response has been read.
	Response(*Flow)

	// Stream request body modifier
	StreamRequestModifier(*Flow, io.Reader) io.Reader

	// Stream response body modifier
	StreamResponseModifier(*Flow, io.Reader) io.Reader
}

// BaseAddon do nothing
type BaseAddon struct{}

func (addon *BaseAddon) ClientConnected(*ClientConn)     {}
func (addon *BaseAddon) ClientDisconnected(*ClientConn)  {}
func (addon *BaseAddon) ServerConnected(*ConnContext)    {}
func (addon *BaseAddon) ServerDisconnected(*ConnContext) {}

func (addon *BaseAddon) TlsEstablishedServer(*ConnContext) {}

func (addon *BaseAddon) Requestheaders(*Flow)  {}
func (addon *BaseAddon) Request(*Flow)         {}
func (addon *BaseAddon) Responseheaders(*Flow) {}
func (addon *BaseAddon) Response(*Flow)        {}
func (addon *BaseAddon) StreamRequestModifier(f *Flow, in io.Reader) io.Reader {
	return in
}
func (addon *BaseAddon) StreamResponseModifier(f *Flow, in io.Reader) io.Reader {
	return in
}

// LogAddon log connection and flow
type LogAddon struct {
	BaseAddon
}

func (addon *LogAddon) ClientConnected(client *ClientConn) {
	log.Infof("%v client connect\n", client.Conn.RemoteAddr())
}

func (addon *LogAddon) ClientDisconnected(client *ClientConn) {
	log.Infof("%v client disconnect\n", client.Conn.RemoteAddr())
}

func (addon *LogAddon) ServerConnected(connCtx *ConnContext) {
	log.Infof("%v server connect %v (%v->%v)\n", connCtx.ClientConn.Conn.RemoteAddr(), connCtx.ServerConn.Address, connCtx.ServerConn.Conn.LocalAddr(), connCtx.ServerConn.Conn.RemoteAddr())
}

func (addon *LogAddon) ServerDisconnected(connCtx *ConnContext) {
	log.Infof("%v server disconnect %v (%v->%v) - %v\n", connCtx.ClientConn.Conn.RemoteAddr(), connCtx.ServerConn.Address, connCtx.ServerConn.Conn.LocalAddr(), connCtx.ServerConn.Conn.RemoteAddr(), connCtx.FlowCount)
}

func (addon *LogAddon) Requestheaders(f *Flow) {
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
		log.Infof("%v %v %v %v %v - %v ms\n", f.ConnContext.ClientConn.Conn.RemoteAddr(), f.Request.Method, f.Request.URL.String(), StatusCode, contentLen, time.Since(start).Milliseconds())
	}()
}
