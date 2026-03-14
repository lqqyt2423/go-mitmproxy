package proxy

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/cert"
	log "github.com/sirupsen/logrus"
)

// testSSEServer 创建一个测试用的 SSE 服务器
func testSSEServer(t *testing.T, eventCount int) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		// 设置 SSE 必需的响应头
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// 创建 flusher 确保数据立即发送
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("Streaming not supported")
		}

		// 发送多个事件
		for i := 0; i < eventCount; i++ {
			event := fmt.Sprintf("data: Message %d\n\n", i)
			_, err := fmt.Fprint(w, event)
			if err != nil {
				t.Logf("Failed to write event: %v", err)
				return
			}
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	})

	return httptest.NewServer(mux)
}

// testTLSSSEServer 创建一个测试用的 HTTPS SSE 服务器
func testTLSSSEServer(t *testing.T, eventCount int) *testTLSServer {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		// 设置 SSE 必需的响应头
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// 创建 flusher 确保数据立即发送
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("Streaming not supported")
		}

		// 发送多个事件
		for i := 0; i < eventCount; i++ {
			event := fmt.Sprintf("data: Secure Message %d\n\n", i)
			_, err := fmt.Fprint(w, event)
			if err != nil {
				t.Logf("Failed to write event: %v", err)
				return
			}
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
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

// consumeSSEStream 消费 SSE 流并返回接收到的事件数量
func consumeSSEStream(t *testing.T, body io.Reader) int {
	t.Helper()

	eventCount := 0
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			eventCount++
		}
	}

	if err := scanner.Err(); err != nil {
		t.Logf("Scanner error: %v", err)
	}

	return eventCount
}

// TestSSEProxy 测试 SSE 代理基本功能
func TestSSEProxy(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	t.Run("should proxy SSE connection over HTTP", func(t *testing.T) {
		eventCount := 5
		sseServer := testSSEServer(t, eventCount)
		defer sseServer.Close()

		// 启动代理
		proxy, err := NewProxy(&Options{
			Addr: ":29097",
		})
		if err != nil {
			t.Fatalf("Failed to create proxy: %v", err)
		}
		go proxy.Start()
		defer proxy.Close()
		time.Sleep(time.Millisecond * 100)

		// 创建代理客户端
		proxyURL, _ := url.Parse("http://127.0.0.1:29097")
		client := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
		}

		// 发送 SSE 请求
		sseEndpoint := sseServer.URL + "/sse"
		req, err := http.NewRequest("GET", sseEndpoint, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// 设置 SSE 请求头
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		// 验证响应头
		if resp.Header.Get("Content-Type") != "text/event-stream" {
			t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", resp.Header.Get("Content-Type"))
		}

		// 消费 SSE 流并验证事件数量
		receivedCount := consumeSSEStream(t, resp.Body)
		if receivedCount != eventCount {
			t.Errorf("Expected %d events, got %d", eventCount, receivedCount)
		}
	})

	t.Run("should proxy SSE connection over HTTPS", func(t *testing.T) {
		eventCount := 5
		sseServer := testTLSSSEServer(t, eventCount)
		defer sseServer.Close()

		// 启动代理
		proxy, err := NewProxy(&Options{
			Addr:        ":29098",
			SslInsecure: true,
		})
		if err != nil {
			t.Fatalf("Failed to create proxy: %v", err)
		}
		go proxy.Start()
		defer proxy.Close()
		time.Sleep(time.Millisecond * 100)

		// 创建代理客户端
		proxyURL, _ := url.Parse("http://127.0.0.1:29098")
		client := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}

		// 发送 SSE 请求
		sseEndpoint := "https://" + sseServer.Addr() + "/sse"
		req, err := http.NewRequest("GET", sseEndpoint, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// 设置 SSE 请求头
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		// 验证响应头
		if resp.Header.Get("Content-Type") != "text/event-stream" {
			t.Errorf("Expected Content-Type 'text/event-stream', got '%s'", resp.Header.Get("Content-Type"))
		}

		// 消费 SSE 流并验证事件数量
		receivedCount := consumeSSEStream(t, resp.Body)
		if receivedCount != eventCount {
			t.Errorf("Expected %d events, got %d", eventCount, receivedCount)
		}
	})
}

// TestSSEWithStreamModifier 测试 SSE 流式响应修改
func TestSSEWithStreamModifier(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	t.Run("should modify SSE stream using StreamResponseModifier", func(t *testing.T) {
		eventCount := 3
		sseServer := testSSEServer(t, eventCount)
		defer sseServer.Close()

		// 创建测试 addon 来修改 SSE 流
		testAddon := &SSETestAddon{
			BaseAddon: BaseAddon{},
			events:    make([]string, 0),
		}

		// 启动代理
		proxy, err := NewProxy(&Options{
			Addr: ":29099",
		})
		if err != nil {
			t.Fatalf("Failed to create proxy: %v", err)
		}
		proxy.AddAddon(testAddon)
		go proxy.Start()
		defer proxy.Close()
		time.Sleep(time.Millisecond * 100)

		// 创建代理客户端
		proxyURL, _ := url.Parse("http://127.0.0.1:29099")
		client := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
		}

		// 发送 SSE 请求
		sseEndpoint := sseServer.URL + "/sse"
		req, err := http.NewRequest("GET", sseEndpoint, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		req.Header.Set("Accept", "text/event-stream")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		// 读取响应
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		// 验证响应中包含修改后的内容
		responseStr := string(body)
		if !strings.Contains(responseStr, "data: Modified Message") {
			t.Error("Expected modified SSE messages, but they were not found")
		}
	})
}

// SSETestAddon 测试用的 SSE Addon
type SSETestAddon struct {
	BaseAddon
	events []string
}

// StreamResponseModifier 修改 SSE 流式响应
func (addon *SSETestAddon) StreamResponseModifier(f *Flow, in io.Reader) io.Reader {
	return io.MultiReader(
		strings.NewReader("data: Modified Message 0\n\n"),
		in,
	)
}
