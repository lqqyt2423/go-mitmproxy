package proxy

import (
	"io"
	"net/http"
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

	// onAccessProxyServer
	AccessProxyServer(req *http.Request, res http.ResponseWriter)

	WebSocketStart(*Flow)
	WebSocketMessage(*Flow)
	WebSocketEnd(*Flow)

	// HTTP request failed with error
	RequestError(*Flow, error)

	// HTTP CONNECT request failed with error
	HTTPConnectError(*Flow, error)
}

// BaseAddon do nothing
type BaseAddon struct{}

func (addon *BaseAddon) ClientConnected(*ClientConn)                                  {}
func (addon *BaseAddon) ClientDisconnected(*ClientConn)                               {}
func (addon *BaseAddon) ServerConnected(*ConnContext)                                 {}
func (addon *BaseAddon) ServerDisconnected(*ConnContext)                              {}
func (addon *BaseAddon) TlsEstablishedServer(*ConnContext)                            {}
func (addon *BaseAddon) Requestheaders(*Flow)                                         {}
func (addon *BaseAddon) Request(*Flow)                                                {}
func (addon *BaseAddon) Responseheaders(*Flow)                                        {}
func (addon *BaseAddon) Response(*Flow)                                               {}
func (addon *BaseAddon) StreamRequestModifier(f *Flow, in io.Reader) io.Reader        { return in }
func (addon *BaseAddon) StreamResponseModifier(f *Flow, in io.Reader) io.Reader       { return in }
func (addon *BaseAddon) AccessProxyServer(req *http.Request, res http.ResponseWriter) {}
func (addon *BaseAddon) WebSocketStart(*Flow)                                         {}
func (addon *BaseAddon) WebSocketMessage(*Flow)                                       {}
func (addon *BaseAddon) WebSocketEnd(*Flow)                                           {}
func (addon *BaseAddon) RequestError(*Flow, error)                                    {}
func (addon *BaseAddon) HTTPConnectError(*Flow, error)                                {}

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
	log.Infof("%v server disconnect %v (%v->%v) - %v\n", connCtx.ClientConn.Conn.RemoteAddr(), connCtx.ServerConn.Address, connCtx.ServerConn.Conn.LocalAddr(), connCtx.ServerConn.Conn.RemoteAddr(), connCtx.FlowCount.Load())
}

func (addon *LogAddon) Requestheaders(f *Flow) {
	log.Debugf("%v Requestheaders %v %v\n", f.ConnContext.ClientConn.Conn.RemoteAddr(), f.Request.Method, f.Request.URL.String())
}

func (addon *LogAddon) Response(f *Flow) {
	var StatusCode int
	if f.Response != nil {
		StatusCode = f.Response.StatusCode
	}
	var contentLen int
	if f.Response != nil && f.Response.Body != nil {
		contentLen = len(f.Response.Body)
	}
	log.Infof("%v %v %v %v %v - %v ms\n", f.ConnContext.ClientConn.Conn.RemoteAddr(), f.Request.Method, f.Request.URL.String(), StatusCode, contentLen, time.Since(f.StartTime).Milliseconds())
}

func (addon *LogAddon) RequestError(f *Flow, err error) {
	var StatusCode int
	if f.Response != nil {
		StatusCode = f.Response.StatusCode
	}
	log.Errorf("%v %v %v %v - ERROR: %v - %v ms\n", f.ConnContext.ClientConn.Conn.RemoteAddr(), f.Request.Method, f.Request.URL.String(), StatusCode, err, time.Since(f.StartTime).Milliseconds())
}

func (addon *LogAddon) HTTPConnectError(f *Flow, err error) {
	log.Errorf("%v CONNECT ERROR %v - %v\n", f.ConnContext.ClientConn.Conn.RemoteAddr(), f.Request.URL.Host, err)
}

// WebSocketStart 记录 WebSocket 连接建立
func (addon *LogAddon) WebSocketStart(f *Flow) {
	log.Infof("%v WebSocket START %s - %s\n",
		f.ConnContext.ClientConn.Conn.RemoteAddr(),
		f.Request.URL.String(),
		f.ConnContext.ServerConn.Address)
}

// WebSocketMessage 记录 WebSocket 消息
func (addon *LogAddon) WebSocketMessage(f *Flow) {
	lastMsg := f.WebScoket.Messages[len(f.WebScoket.Messages)-1]
	direction := "C->S"
	if !lastMsg.FromClient {
		direction = "S->C"
	}
	msgType := "TEXT"
	if lastMsg.Type == 2 {
		msgType = "BINARY"
	}

	// 记录消息内容和方向
	content := string(lastMsg.Content)
	if len(content) > 100 {
		content = content[:100] + "..."
	}
	log.Infof("%v WebSocket MSG %s %s [%s] len=%d %s\n",
		f.ConnContext.ClientConn.Conn.RemoteAddr(),
		f.Request.URL.String(),
		direction,
		msgType,
		len(lastMsg.Content),
		content)
}

// WebSocketEnd 记录 WebSocket 连接结束
func (addon *LogAddon) WebSocketEnd(f *Flow) {
	log.Infof("%v WebSocket END %s - %d messages\n",
		f.ConnContext.ClientConn.Conn.RemoteAddr(),
		f.Request.URL.String(),
		len(f.WebScoket.Messages))
}


type UpstreamCertAddon struct {
	BaseAddon
	UpstreamCert bool // Connect to upstream server to look up certificate details.
}

func NewUpstreamCertAddon(upstreamCert bool) *UpstreamCertAddon {
	return &UpstreamCertAddon{UpstreamCert: upstreamCert}
}

func (addon *UpstreamCertAddon) ClientConnected(conn *ClientConn) {
	conn.UpstreamCert = addon.UpstreamCert
}
