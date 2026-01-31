package proxy

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lqqyt2423/go-mitmproxy/cert"
	log "github.com/sirupsen/logrus"
)

// testWebSocketServer 创建一个测试用的 WS 服务器
func testWebSocketServer(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	t.Helper()

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade connection: %v", err)
			return
		}
		handler(conn)
	})

	return httptest.NewServer(mux)
}

// testTLSWebSocketServer 创建一个测试用的 WSS (TLS WebSocket) 服务器
func testTLSWebSocketServer(t *testing.T, handler func(*websocket.Conn)) *testTLSServer {
	t.Helper()

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade connection: %v", err)
			return
		}
		handler(conn)
	})

	// 创建 TLS 监听器
	tlsPlainLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	// 生成自签名证书
	ca, err := cert.NewSelfSignCA("")
	if err != nil {
		t.Fatalf("Failed to create CA: %v", err)
	}
	tlsCert, err := ca.GetCert("localhost")
	if err != nil {
		t.Fatalf("Failed to get cert: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*tlsCert},
	}

	server := &http.Server{
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	httpsPort := tlsPlainLn.Addr().(*net.TCPAddr).Port
	tlsLn := tls.NewListener(tlsPlainLn, tlsConfig)

	go server.Serve(tlsLn)

	return &testTLSServer{
		server: server,
		tlsLn:  tlsLn,
		port:   httpsPort,
	}
}

// testTLSServer 测试用的 TLS 服务器包装
type testTLSServer struct {
	server *http.Server
	tlsLn  net.Listener
	port   int
}

// Close 关闭服务器
func (s *testTLSServer) Close() {
	s.server.Close()
	s.tlsLn.Close()
}

// Addr 返回服务器地址
func (s *testTLSServer) Addr() string {
	return "localhost:" + strconv.Itoa(s.port)
}

// testEchoWebSocketHandler 创建一个回显处理器
func testEchoWebSocketHandler(t *testing.T) func(*websocket.Conn) {
	return func(conn *websocket.Conn) {
		defer conn.Close()

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					t.Logf("WebSocket read error: %v", err)
				}
				return
			}

			err = conn.WriteMessage(messageType, message)
			if err != nil {
				t.Logf("WebSocket write error: %v", err)
				return
			}
		}
	}
}

// TestWebSocketWSProxy 测试 WS 代理基本功能
func TestWebSocketWSProxy(t *testing.T) {
	// 设置日志级别为 Debug，以便查看详细的调试信息
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	t.Run("should proxy WS connection", func(t *testing.T) {
		// 创建测试用的 WS 服务器
		wsServer := testWebSocketServer(t, testEchoWebSocketHandler(t))
		defer wsServer.Close()

		// 解析 WS 服务器地址
		wsURL, err := url.Parse(wsServer.URL)
		if err != nil {
			t.Fatalf("Failed to parse WS server URL: %v", err)
		}

		// 启动代理
		proxy, err := NewProxy(&Options{
			Addr: ":29090",
		})
		if err != nil {
			t.Fatalf("Failed to create proxy: %v", err)
		}
		go proxy.Start()
		defer proxy.Close()
		time.Sleep(time.Millisecond * 100)

		// 创建代理客户端
		proxyURL, _ := url.Parse("http://127.0.0.1:29090")
		dialer := &websocket.Dialer{
			Proxy:            http.ProxyURL(proxyURL),
			HandshakeTimeout: time.Second * 5,
		}

		// 连接到 WS 服务器（通过代理）
		wsEndpoint := "ws://" + wsURL.Host + "/ws"
		conn, resp, err := dialer.Dial(wsEndpoint, nil)
		if err != nil {
			t.Fatalf("Failed to dial WS via proxy: %v, response: %v", err, resp)
		}
		defer conn.Close()

		// 发送测试消息
		testMessage := "Hello, WebSocket!"

		err = conn.WriteMessage(websocket.TextMessage, []byte(testMessage))
		if err != nil {
			t.Fatalf("Failed to send message: %v", err)
		}

		_, receivedMsg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		if string(receivedMsg) != testMessage {
			t.Fatalf("Expected message %q, got %q", testMessage, string(receivedMsg))
		}

		// 关闭连接
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			t.Fatalf("Failed to send close message: %v", err)
		}

		// 等待连接关闭
		time.Sleep(time.Millisecond * 100)
	})

	t.Run("should proxy WS connection when UpstreamCert false", func(t *testing.T) {
		// 创建测试用的 WS 服务器
		wsServer := testWebSocketServer(t, testEchoWebSocketHandler(t))
		defer wsServer.Close()

		// 解析 WS 服务器地址
		wsURL, err := url.Parse(wsServer.URL)
		if err != nil {
			t.Fatalf("Failed to parse WS server URL: %v", err)
		}

		// 启动代理
		proxy, err := NewProxy(&Options{
			Addr: ":29091",
		})
		if err != nil {
			t.Fatalf("Failed to create proxy: %v", err)
		}
		// 添加 UpstreamCertAddon 并设置为 false
		proxy.AddAddon(NewUpstreamCertAddon(false))

		go proxy.Start()
		defer proxy.Close()
		time.Sleep(time.Millisecond * 100)

		// 创建代理客户端
		proxyURL, _ := url.Parse("http://127.0.0.1:29091")
		dialer := &websocket.Dialer{
			Proxy:            http.ProxyURL(proxyURL),
			HandshakeTimeout: time.Second * 5,
		}

		// 连接到 WS 服务器（通过代理）
		wsEndpoint := "ws://" + wsURL.Host + "/ws"
		conn, resp, err := dialer.Dial(wsEndpoint, nil)
		if err != nil {
			t.Fatalf("Failed to dial WS via proxy: %v, response: %v", err, resp)
		}
		defer conn.Close()

		// 发送测试消息
		testMessage := "Hello, WebSocket with UpstreamCert false!"

		err = conn.WriteMessage(websocket.TextMessage, []byte(testMessage))
		if err != nil {
			t.Fatalf("Failed to send message: %v", err)
		}

		_, receivedMsg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		if string(receivedMsg) != testMessage {
			t.Fatalf("Expected message %q, got %q", testMessage, string(receivedMsg))
		}

		// 关闭连接
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			t.Fatalf("Failed to send close message: %v", err)
		}

		// 等待连接关闭
		time.Sleep(time.Millisecond * 100)
	})
}

// TestWebSocketWSSProxy 测试 WSS 代理基本功能
func TestWebSocketWSSProxy(t *testing.T) {
	// 设置日志级别为 Debug，以便查看详细的调试信息
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	t.Run("should proxy WSS connection", func(t *testing.T) {
		// 创建测试用的 WSS 服务器
		wssServer := testTLSWebSocketServer(t, testEchoWebSocketHandler(t))
		defer wssServer.Close()

		// 启动代理
		proxy, err := NewProxy(&Options{
			Addr:        ":29092",
			SslInsecure: true,
		})
		if err != nil {
			t.Fatalf("Failed to create proxy: %v", err)
		}
		go proxy.Start()
		defer proxy.Close()
		time.Sleep(time.Millisecond * 100)

		// 创建代理客户端
		proxyURL, _ := url.Parse("http://127.0.0.1:29092")
		dialer := &websocket.Dialer{
			Proxy: http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			HandshakeTimeout: time.Second * 5,
		}

		// 连接到 WSS 服务器（通过代理）
		wssEndpoint := "wss://" + wssServer.Addr() + "/ws"
		conn, resp, err := dialer.Dial(wssEndpoint, nil)
		if err != nil {
			t.Fatalf("Failed to dial WSS via proxy: %v, response: %v", err, resp)
		}
		defer conn.Close()

		// 发送测试消息
		testMessage := "Hello, Secure WebSocket!"

		err = conn.WriteMessage(websocket.TextMessage, []byte(testMessage))
		if err != nil {
			t.Fatalf("Failed to send message: %v", err)
		}

		_, receivedMsg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		if string(receivedMsg) != testMessage {
			t.Fatalf("Expected message %q, got %q", testMessage, string(receivedMsg))
		}

		// 关闭连接
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			t.Fatalf("Failed to send close message: %v", err)
		}

		// 等待连接关闭
		time.Sleep(time.Millisecond * 100)
	})
}
