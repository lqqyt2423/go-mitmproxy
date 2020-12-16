package proxy

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
)

// 当前仅做了转发 websocket 流量

type WebSocket struct{}

var DefaultWebSocket WebSocket

func (s *WebSocket) WS(conn net.Conn, host string) {
	log := log.WithField("in", "WebSocket.WS").WithField("host", host)

	defer conn.Close()
	remoteConn, err := net.Dial("tcp", host)
	if err != nil {
		LogErr(log, err)
		return
	}
	defer remoteConn.Close()
	Transfer(log, conn, remoteConn)
}

func (s *WebSocket) WSS(res http.ResponseWriter, req *http.Request) {
	log := log.WithField("in", "WebSocket.WSS").WithField("host", req.Host)

	upgradeBuf, err := httputil.DumpRequest(req, false)
	if err != nil {
		log.Errorf("DumpRequest: %v\n", err)
		res.WriteHeader(502)
		return
	}

	cconn, _, err := res.(http.Hijacker).Hijack()
	if err != nil {
		log.Errorf("Hijack: %v\n", err)
		res.WriteHeader(502)
		return
	}
	defer cconn.Close()

	host := req.Host
	if !strings.Contains(host, ":") {
		host = host + ":443"
	}
	conn, err := tls.Dial("tcp", host, nil)
	if err != nil {
		log.Errorf("tls.Dial: %v\n", err)
		return
	}
	defer conn.Close()

	_, err = conn.Write(upgradeBuf)
	if err != nil {
		log.Errorf("wss upgrade: %v\n", err)
		return
	}
	Transfer(log, conn, cconn)
}
