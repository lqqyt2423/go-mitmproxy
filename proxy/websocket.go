package proxy

import (
	"bufio"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type webSocketHandler struct {
	proxy *Proxy
}

func newWebSocketHandler(proxy *Proxy) *webSocketHandler {
	return &webSocketHandler{proxy: proxy}
}

// connResponseWriter 自定义 ResponseWriter，包装 net.Conn
// 用于让 websocket.Upgrader 能够升级连接
type connResponseWriter struct {
	conn        net.Conn
	header      http.Header
	statusCode  int
	wroteHeader bool
}

func newConnResponseWriter(conn net.Conn) *connResponseWriter {
	return &connResponseWriter{
		conn:   conn,
		header: make(http.Header),
	}
}

func (w *connResponseWriter) Header() http.Header {
	return w.header
}

func (w *connResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.conn.Write(data)
}

func (w *connResponseWriter) WriteHeader(statusCode int) {
	w.wroteHeader = true
	w.statusCode = statusCode
}

// Hijack 劫持连接，返回底层的 net.Conn 和 bufio.ReadWriter
// websocket.Upgrader.Upgrade() 需要调用这个方法
func (w *connResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	buf := bufio.NewReadWriter(bufio.NewReader(w.conn), bufio.NewWriter(w.conn))
	return w.conn, buf, nil
}

// handle 处理 WebSocket 连接
// serverConn: 与服务器的连接（已经建立 TCP 连接）
// clientConn: 与客户端的连接（已经完成 CONNECT，客户端发送了 WebSocket 握手请求）
func (h *webSocketHandler) handle(serverConn, clientConn net.Conn, f *Flow) error {
	// 步骤 1: 读取客户端握手请求
	buf := bufio.NewReader(clientConn)
	clientReq, err := http.ReadRequest(buf)
	if err != nil {
		log.Errorf("Failed to read client handshake: %v", err)
		return err
	}

	log.Debugf("Client WebSocket handshake: %s %s", clientReq.Method, clientReq.URL.Path)

	// 步骤 2: 使用 Dialer 连接到服务器
	dialer := &websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			return serverConn, nil
		},
		// 使用已有的连接，不需要重新拨号
		HandshakeTimeout: 0,
	}

	serverURL := "ws://" + clientReq.Host + clientReq.URL.RequestURI()
	log.Debugf("Connecting to server: %s", serverURL)

	// 转发客户端的请求头（Cookie、Origin 等），但排除 WebSocket 握手专用头
	// gorilla/websocket Dialer 会自动生成 Upgrade、Connection、Sec-WebSocket-* 等握手头
	forwardHeaders := filterWebSocketHeaders(clientReq.Header)
	if protocols := clientReq.Header.Get("Sec-WebSocket-Protocol"); protocols != "" {
		dialer.Subprotocols = websocket.Subprotocols(clientReq)
	}

	serverWS, _, err := dialer.Dial(serverURL, forwardHeaders)
	if err != nil {
		log.Errorf("Failed to dial server: %v", err)
		return err
	}
	defer serverWS.Close()

	log.Debugf("Server WebSocket connected, subprotocol: %s", serverWS.Subprotocol())

	// 步骤 3: 使用 Upgrader 升级客户端连接
	respWriter := newConnResponseWriter(clientConn)

	upgrader := &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // 代理模式下总是允许
		},
	}

	clientWS, err := upgrader.Upgrade(respWriter, clientReq, nil)
	if err != nil {
		log.Errorf("Failed to upgrade client connection: %v", err)
		return err
	}
	defer clientWS.Close()

	log.Debugf("Client WebSocket upgraded successfully")

	wsData := newWebSocketData()
	f.WebScoket = wsData

	for _, addon := range h.proxy.Addons {
		addon.WebSocketStart(f)
	}

	// 步骤 4: 双向转发消息
	return h.forwardMessages(clientWS, serverWS, f)
}

// forwardMessages 双向转发 WebSocket 消息
func (h *webSocketHandler) forwardMessages(clientWS, serverWS *websocket.Conn, f *Flow) error {
	defer func() {
		for _, addon := range h.proxy.Addons {
			addon.WebSocketEnd(f)
		}
	}()

	errChan := make(chan error, 2)

	// 客户端 -> 服务器
	go func() {
		defer func() {
			// 优雅关闭服务器连接
			serverWS.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			serverWS.Close()
		}()

		for {
			msgType, msg, err := clientWS.ReadMessage()
			if err != nil {
				// 判断是否是正常的关闭
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					errChan <- nil // 正常关闭，不返回错误
					return
				}
				errChan <- err
				return
			}

			f.WebScoket.addMessage(msgType, msg, true)
			for _, addon := range h.proxy.Addons {
				addon.WebSocketMessage(f)
			}

			if err := serverWS.WriteMessage(msgType, msg); err != nil {
				log.Errorf("Client -> Server: Write error: %v", err)
				errChan <- err
				return
			}
		}
	}()

	// 服务器 -> 客户端
	go func() {
		defer func() {
			// 优雅关闭客户端连接
			clientWS.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			clientWS.Close()
		}()

		for {
			msgType, msg, err := serverWS.ReadMessage()
			if err != nil {
				// 判断是否是正常的关闭
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					errChan <- nil // 正常关闭，不返回错误
					return
				}
				errChan <- err
				return
			}

			f.WebScoket.addMessage(msgType, msg, false)
			for _, addon := range h.proxy.Addons {
				addon.WebSocketMessage(f)
			}

			if err := clientWS.WriteMessage(msgType, msg); err != nil {
				log.Errorf("Server -> Client: Write error: %v", err)
				errChan <- err
				return
			}
		}
	}()

	// 等待任一方向出错或关闭
	err := <-errChan
	// 如果是正常关闭（nil），返回 nil
	return err
}

func (h *webSocketHandler) handleWSS(res http.ResponseWriter, req *http.Request) error {
	// 修复 WebSocket URL，确保包含完整的 scheme 和 host
	serverURL := "wss://" + req.Host + req.URL.RequestURI()
	log.Debugf("Connecting to WSS server: %s", serverURL)
	if parsedURL, err := url.Parse(serverURL); err == nil {
		req.URL = parsedURL
	}

	connCtx := req.Context().Value(connContextKey).(*ConnContext)

	// 步骤 1: 获取上游连接
	plainConn, err := h.proxy.getUpstreamConn(req.Context(), req)
	if err != nil {
		log.Errorf("Failed to get upstream connection: %v", err)
		return err
	}

	// 步骤 2: 创建并初始化 ServerConn
	serverConn := newServerConn()
	serverConn.Address = req.Host
	serverConn.Conn = &wrapServerConn{
		Conn:    plainConn,
		proxy:   h.proxy,
		connCtx: connCtx,
	}
	connCtx.ServerConn = serverConn

	// 步骤 3: 调用 addon 的 ServerConnected 回调
	for _, addon := range connCtx.proxy.Addons {
		addon.ServerConnected(connCtx)
	}

	// 步骤 4: 创建 Dialer，使用自定义 NetDial
	dialer := &websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			return serverConn.Conn, nil
		},
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: h.proxy.Opts.SslInsecure,
		},
	}

	// 转发客户端的请求头（Cookie、Origin 等），但排除 WebSocket 握手专用头
	// gorilla/websocket Dialer 会自动生成 Upgrade、Connection、Sec-WebSocket-* 等握手头
	forwardHeaders := filterWebSocketHeaders(req.Header)
	if protocols := req.Header.Get("Sec-WebSocket-Protocol"); protocols != "" {
		dialer.Subprotocols = websocket.Subprotocols(req)
	}

	serverWS, _, err := dialer.Dial(serverURL, forwardHeaders)
	if err != nil {
		log.Errorf("Failed to dial WSS server: %v", err)
		return err
	}
	defer serverWS.Close()

	upgrader := &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // 代理模式下总是允许
		},
	}

	clientWS, err := upgrader.Upgrade(res, req, nil)
	if err != nil {
		log.Errorf("Failed to upgrade client connection: %v", err)
		return err
	}
	defer clientWS.Close()

	log.Debugf("Client WSS upgraded successfully")

	wsData := newWebSocketData()
	f := newFlow()
	f.Request = newRequest(req)
	f.ConnContext = connCtx
	f.WebScoket = wsData
	defer f.finish()

	for _, addon := range h.proxy.Addons {
		addon.WebSocketStart(f)
	}

	// 双向转发消息
	return h.forwardMessages(clientWS, serverWS, f)
}

// filterWebSocketHeaders 从客户端请求头中提取需要转发给上游服务器的头
// 排除 WebSocket 握手专用头（gorilla/websocket Dialer 会自动生成这些头）
func filterWebSocketHeaders(src http.Header) http.Header {
	skipHeaders := map[string]bool{
		"Upgrade":                  true,
		"Connection":               true,
		"Sec-Websocket-Key":        true,
		"Sec-Websocket-Version":    true,
		"Sec-Websocket-Extensions": true,
		"Sec-Websocket-Protocol":   true,
	}
	dst := http.Header{}
	for key, values := range src {
		if !skipHeaders[http.CanonicalHeaderKey(key)] {
			dst[key] = values
		}
	}
	return dst
}
