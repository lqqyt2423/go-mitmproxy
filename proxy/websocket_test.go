package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
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
