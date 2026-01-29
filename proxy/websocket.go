package proxy

import (
	"bufio"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"

	log "github.com/sirupsen/logrus"
)

// 当前仅做了转发 websocket 流量

type webSocket struct{}

var defaultWebSocket webSocket

// func (s *webSocket) ws(conn net.Conn, host string) {
// 	log := log.WithField("in", "webSocket.ws").WithField("host", host)

// 	defer conn.Close()
// 	remoteConn, err := net.Dial("tcp", host)
// 	if err != nil {
// 		logErr(log, err)
// 		return
// 	}
// 	defer remoteConn.Close()
// 	transfer(log, conn, remoteConn)
// }

func (s *webSocket) wss(res http.ResponseWriter, req *http.Request) {
	log := log.WithField("in", "webSocket.wss").WithField("host", req.Host)

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
	transfer(log, conn, cconn)
}

type webSocketHandler struct {
}

func newWebSocketHandler() *webSocketHandler {
	return &webSocketHandler{}
}

func (wsh *webSocketHandler) handle(server, client io.ReadWriteCloser) error {
	clientReq, err := http.ReadRequest(bufio.NewReader(client))
	if err != nil {
		return err
	}
	err = clientReq.Write(server)
	if err != nil {
		return err
	}
	serverResp, err := http.ReadResponse(bufio.NewReader(server), nil)
	if err != nil {
		return err
	}
	err = serverResp.Write(client)
	if err != nil {
		return err
	}
	// todo: 判断是否 upgrade 成功
	// todo: 转发消息
	return nil
}
