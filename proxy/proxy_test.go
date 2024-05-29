package proxy

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/cert"
)

func handleError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func testSendRequest(t *testing.T, endpoint string, client *http.Client, bodyWant string) {
	t.Helper()
	req, err := http.NewRequest("GET", endpoint, nil)
	handleError(t, err)
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	handleError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	handleError(t, err)
	if string(body) != bodyWant {
		t.Fatalf("expected %s, but got %s", bodyWant, body)
	}
}

type testProxyHelper struct {
	server    *http.Server
	proxyAddr string

	ln                     net.Listener
	tlsPlainLn             net.Listener
	tlsLn                  net.Listener
	httpEndpoint           string
	httpsEndpoint          string
	testOrderAddonInstance *testOrderAddon
	testProxy              *Proxy
	getProxyClient         func() *http.Client
}

func (helper *testProxyHelper) init(t *testing.T) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	helper.server.Handler = mux

	// start http server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	handleError(t, err)
	helper.ln = ln

	// start https server
	tlsPlainLn, err := net.Listen("tcp", "127.0.0.1:0")
	handleError(t, err)
	helper.tlsPlainLn = tlsPlainLn
	ca, err := cert.NewSelfSignCAMemory()
	handleError(t, err)
	cert, err := ca.GetCert("localhost")
	handleError(t, err)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
	}
	helper.server.TLSConfig = tlsConfig
	helper.tlsLn = tls.NewListener(tlsPlainLn, tlsConfig)

	httpEndpoint := "http://" + ln.Addr().String() + "/"
	httpsPort := tlsPlainLn.Addr().(*net.TCPAddr).Port
	httpsEndpoint := "https://localhost:" + strconv.Itoa(httpsPort) + "/"
	helper.httpEndpoint = httpEndpoint
	helper.httpsEndpoint = httpsEndpoint

	// start proxy
	testProxy, err := NewProxy(&Options{
		Addr:        helper.proxyAddr, // some random port
		SslInsecure: true,
	})
	handleError(t, err)
	testProxy.AddAddon(&interceptAddon{})
	testOrderAddonInstance := &testOrderAddon{
		orders: make([]string, 0),
	}
	testProxy.AddAddon(testOrderAddonInstance)
	helper.testOrderAddonInstance = testOrderAddonInstance
	helper.testProxy = testProxy

	getProxyClient := func() *http.Client {
		return &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				Proxy: func(r *http.Request) (*url.URL, error) {
					return url.Parse("http://127.0.0.1" + helper.proxyAddr)
				},
			},
		}
	}
	helper.getProxyClient = getProxyClient
}

// addon for test intercept
type interceptAddon struct {
	BaseAddon
}

func (addon *interceptAddon) Request(f *Flow) {
	// intercept request, should not send request to real endpoint
	if f.Request.URL.Path == "/intercept-request" {
		f.Response = &Response{
			StatusCode: 200,
			Body:       []byte("intercept-request"),
		}
	}
}

func (addon *interceptAddon) Response(f *Flow) {
	if f.Request.URL.Path == "/intercept-response" {
		f.Response = &Response{
			StatusCode: 200,
			Body:       []byte("intercept-response"),
		}
	}
}

// addon for test functions' execute order
type testOrderAddon struct {
	BaseAddon
	orders []string
	mu     sync.Mutex
}

func (addon *testOrderAddon) reset() {
	addon.mu.Lock()
	defer addon.mu.Unlock()
	addon.orders = make([]string, 0)
}

func (addon *testOrderAddon) contains(t *testing.T, name string) {
	t.Helper()
	addon.mu.Lock()
	defer addon.mu.Unlock()
	for _, n := range addon.orders {
		if name == n {
			return
		}
	}
	t.Fatalf("expected contains %s, but not", name)
}

func (addon *testOrderAddon) before(t *testing.T, a, b string) {
	t.Helper()
	addon.mu.Lock()
	defer addon.mu.Unlock()
	aIndex, bIndex := -1, -1
	for i, n := range addon.orders {
		if a == n {
			aIndex = i
		} else if b == n {
			bIndex = i
		}
	}
	if aIndex == -1 {
		t.Fatalf("expected contains %s, but not", a)
	}
	if bIndex == -1 {
		t.Fatalf("expected contains %s, but not", b)
	}
	if aIndex > bIndex {
		t.Fatalf("expected %s executed before %s, but not", a, b)
	}
}

func (addon *testOrderAddon) ClientConnected(*ClientConn) {
	addon.mu.Lock()
	defer addon.mu.Unlock()
	addon.orders = append(addon.orders, "ClientConnected")
}
func (addon *testOrderAddon) ClientDisconnected(*ClientConn) {
	addon.mu.Lock()
	defer addon.mu.Unlock()
	addon.orders = append(addon.orders, "ClientDisconnected")
}
func (addon *testOrderAddon) ServerConnected(*ConnContext) {
	addon.mu.Lock()
	defer addon.mu.Unlock()
	addon.orders = append(addon.orders, "ServerConnected")
}
func (addon *testOrderAddon) ServerDisconnected(*ConnContext) {
	addon.mu.Lock()
	defer addon.mu.Unlock()
	addon.orders = append(addon.orders, "ServerDisconnected")
}
func (addon *testOrderAddon) TlsEstablishedServer(*ConnContext) {
	addon.mu.Lock()
	defer addon.mu.Unlock()
	addon.orders = append(addon.orders, "TlsEstablishedServer")
}
func (addon *testOrderAddon) Requestheaders(*Flow) {
	addon.mu.Lock()
	defer addon.mu.Unlock()
	addon.orders = append(addon.orders, "Requestheaders")
}
func (addon *testOrderAddon) Request(*Flow) {
	addon.mu.Lock()
	defer addon.mu.Unlock()
	addon.orders = append(addon.orders, "Request")
}
func (addon *testOrderAddon) Responseheaders(*Flow) {
	addon.mu.Lock()
	defer addon.mu.Unlock()
	addon.orders = append(addon.orders, "Responseheaders")
}
func (addon *testOrderAddon) Response(*Flow) {
	addon.mu.Lock()
	defer addon.mu.Unlock()
	addon.orders = append(addon.orders, "Response")
}
func (addon *testOrderAddon) StreamRequestModifier(f *Flow, in io.Reader) io.Reader {
	addon.mu.Lock()
	defer addon.mu.Unlock()
	addon.orders = append(addon.orders, "StreamRequestModifier")
	return in
}
func (addon *testOrderAddon) StreamResponseModifier(f *Flow, in io.Reader) io.Reader {
	addon.mu.Lock()
	defer addon.mu.Unlock()
	addon.orders = append(addon.orders, "StreamResponseModifier")
	return in
}

func TestProxy(t *testing.T) {
	helper := &testProxyHelper{
		server:    &http.Server{},
		proxyAddr: ":29080",
	}
	helper.init(t)
	httpEndpoint := helper.httpEndpoint
	httpsEndpoint := helper.httpsEndpoint
	testOrderAddonInstance := helper.testOrderAddonInstance
	testProxy := helper.testProxy
	getProxyClient := helper.getProxyClient
	defer helper.ln.Close()
	go helper.server.Serve(helper.ln)
	defer helper.tlsPlainLn.Close()
	go helper.server.Serve(helper.tlsLn)
	go testProxy.Start()
	time.Sleep(time.Millisecond * 10) // wait for test proxy startup

	t.Run("test http server", func(t *testing.T) {
		testSendRequest(t, httpEndpoint, nil, "ok")
	})

	t.Run("test https server", func(t *testing.T) {
		t.Run("should generate not trusted error", func(t *testing.T) {
			_, err := http.Get(httpsEndpoint)
			if err == nil {
				t.Fatal("should have error")
			}
			if !strings.Contains(err.Error(), "certificate is not trusted") {
				t.Fatal("should get not trusted error, but got", err.Error())
			}
		})

		t.Run("should get ok when InsecureSkipVerify", func(t *testing.T) {
			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}
			testSendRequest(t, httpsEndpoint, client, "ok")
		})
	})

	t.Run("test proxy", func(t *testing.T) {
		proxyClient := getProxyClient()

		t.Run("can proxy http", func(t *testing.T) {
			testSendRequest(t, httpEndpoint, proxyClient, "ok")
		})

		t.Run("can proxy https", func(t *testing.T) {
			testSendRequest(t, httpsEndpoint, proxyClient, "ok")
		})

		t.Run("can intercept request", func(t *testing.T) {
			t.Run("http", func(t *testing.T) {
				testSendRequest(t, httpEndpoint+"intercept-request", proxyClient, "intercept-request")
			})
			t.Run("https", func(t *testing.T) {
				testSendRequest(t, httpsEndpoint+"intercept-request", proxyClient, "intercept-request")
			})
		})

		t.Run("can intercept request with wrong host", func(t *testing.T) {
			t.Run("http", func(t *testing.T) {
				httpEndpoint := "http://some-wrong-host/"
				testSendRequest(t, httpEndpoint+"intercept-request", proxyClient, "intercept-request")
			})
			t.Run("https can't", func(t *testing.T) {
				httpsEndpoint := "https://some-wrong-host/"
				_, err := http.Get(httpsEndpoint + "intercept-request")
				if err == nil {
					t.Fatal("should have error")
				}
				if !strings.Contains(err.Error(), "dial tcp") {
					t.Fatal("should get dial error, but got", err.Error())
				}
			})
		})

		t.Run("can intercept response", func(t *testing.T) {
			t.Run("http", func(t *testing.T) {
				testSendRequest(t, httpEndpoint+"intercept-response", proxyClient, "intercept-response")
			})
			t.Run("https", func(t *testing.T) {
				testSendRequest(t, httpsEndpoint+"intercept-response", proxyClient, "intercept-response")
			})
		})
	})

	t.Run("test proxy when DisableKeepAlives", func(t *testing.T) {
		proxyClient := getProxyClient()
		proxyClient.Transport.(*http.Transport).DisableKeepAlives = true

		t.Run("http", func(t *testing.T) {
			testSendRequest(t, httpEndpoint, proxyClient, "ok")
		})

		t.Run("https", func(t *testing.T) {
			testSendRequest(t, httpsEndpoint, proxyClient, "ok")
		})
	})

	t.Run("should trigger disconnect functions when DisableKeepAlives", func(t *testing.T) {
		proxyClient := getProxyClient()
		proxyClient.Transport.(*http.Transport).DisableKeepAlives = true

		t.Run("http", func(t *testing.T) {
			time.Sleep(time.Millisecond * 10)
			testOrderAddonInstance.reset()
			testSendRequest(t, httpEndpoint, proxyClient, "ok")
			time.Sleep(time.Millisecond * 10)
			testOrderAddonInstance.contains(t, "ClientDisconnected")
			testOrderAddonInstance.contains(t, "ServerDisconnected")
		})

		t.Run("https", func(t *testing.T) {
			time.Sleep(time.Millisecond * 10)
			testOrderAddonInstance.reset()
			testSendRequest(t, httpsEndpoint, proxyClient, "ok")
			time.Sleep(time.Millisecond * 10)
			testOrderAddonInstance.contains(t, "ClientDisconnected")
			testOrderAddonInstance.contains(t, "ServerDisconnected")
		})
	})

	t.Run("should not have eof error when DisableKeepAlives", func(t *testing.T) {
		proxyClient := getProxyClient()
		proxyClient.Transport.(*http.Transport).DisableKeepAlives = true
		t.Run("http", func(t *testing.T) {
			for i := 0; i < 10; i++ {
				testSendRequest(t, httpEndpoint, proxyClient, "ok")
			}
		})
		t.Run("https", func(t *testing.T) {
			for i := 0; i < 10; i++ {
				testSendRequest(t, httpsEndpoint, proxyClient, "ok")
			}
		})
	})

	t.Run("should trigger disconnect functions when client side trigger off", func(t *testing.T) {
		proxyClient := getProxyClient()
		var clientConn net.Conn
		proxyClient.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			c, err := (&net.Dialer{}).DialContext(ctx, network, addr)
			clientConn = c
			return c, err
		}

		t.Run("http", func(t *testing.T) {
			time.Sleep(time.Millisecond * 10)
			testOrderAddonInstance.reset()
			testSendRequest(t, httpEndpoint, proxyClient, "ok")
			clientConn.Close()
			time.Sleep(time.Millisecond * 10)
			testOrderAddonInstance.contains(t, "ClientDisconnected")
			testOrderAddonInstance.contains(t, "ServerDisconnected")
			testOrderAddonInstance.before(t, "ClientDisconnected", "ServerDisconnected")
		})

		t.Run("https", func(t *testing.T) {
			time.Sleep(time.Millisecond * 10)
			testOrderAddonInstance.reset()
			testSendRequest(t, httpsEndpoint, proxyClient, "ok")
			clientConn.Close()
			time.Sleep(time.Millisecond * 10)
			testOrderAddonInstance.contains(t, "ClientDisconnected")
			testOrderAddonInstance.contains(t, "ServerDisconnected")
			testOrderAddonInstance.before(t, "ClientDisconnected", "ServerDisconnected")
		})
	})
}

func TestProxyWhenServerNotKeepAlive(t *testing.T) {
	server := &http.Server{}
	server.SetKeepAlivesEnabled(false)
	helper := &testProxyHelper{
		server:    server,
		proxyAddr: ":29081",
	}
	helper.init(t)
	httpEndpoint := helper.httpEndpoint
	httpsEndpoint := helper.httpsEndpoint
	testOrderAddonInstance := helper.testOrderAddonInstance
	testProxy := helper.testProxy
	getProxyClient := helper.getProxyClient
	defer helper.ln.Close()
	go helper.server.Serve(helper.ln)
	defer helper.tlsPlainLn.Close()
	go helper.server.Serve(helper.tlsLn)
	go testProxy.Start()
	time.Sleep(time.Millisecond * 10) // wait for test proxy startup

	t.Run("should not have eof error when server side DisableKeepAlives", func(t *testing.T) {
		proxyClient := getProxyClient()
		t.Run("http", func(t *testing.T) {
			for i := 0; i < 10; i++ {
				testSendRequest(t, httpEndpoint, proxyClient, "ok")
			}
		})
		t.Run("https", func(t *testing.T) {
			for i := 0; i < 10; i++ {
				testSendRequest(t, httpsEndpoint, proxyClient, "ok")
			}
		})
	})

	t.Run("should trigger disconnect functions when server DisableKeepAlives", func(t *testing.T) {
		proxyClient := getProxyClient()

		t.Run("http", func(t *testing.T) {
			time.Sleep(time.Millisecond * 10)
			testOrderAddonInstance.reset()
			testSendRequest(t, httpEndpoint, proxyClient, "ok")
			time.Sleep(time.Millisecond * 10)
			testOrderAddonInstance.contains(t, "ClientDisconnected")
			testOrderAddonInstance.contains(t, "ServerDisconnected")
			testOrderAddonInstance.before(t, "ServerDisconnected", "ClientDisconnected")
		})

		t.Run("https", func(t *testing.T) {
			time.Sleep(time.Millisecond * 10)
			testOrderAddonInstance.reset()
			testSendRequest(t, httpsEndpoint, proxyClient, "ok")
			time.Sleep(time.Millisecond * 10)
			testOrderAddonInstance.contains(t, "ClientDisconnected")
			testOrderAddonInstance.contains(t, "ServerDisconnected")
			testOrderAddonInstance.before(t, "ServerDisconnected", "ClientDisconnected")
		})
	})
}

func TestProxyWhenServerKeepAliveButCloseImmediately(t *testing.T) {
	helper := &testProxyHelper{
		server: &http.Server{
			IdleTimeout: time.Millisecond * 10,
		},
		proxyAddr: ":29082",
	}
	helper.init(t)
	httpEndpoint := helper.httpEndpoint
	httpsEndpoint := helper.httpsEndpoint
	testOrderAddonInstance := helper.testOrderAddonInstance
	testProxy := helper.testProxy
	getProxyClient := helper.getProxyClient
	defer helper.ln.Close()
	go helper.server.Serve(helper.ln)
	defer helper.tlsPlainLn.Close()
	go helper.server.Serve(helper.tlsLn)
	go testProxy.Start()
	time.Sleep(time.Millisecond * 10) // wait for test proxy startup

	t.Run("should not have eof error when server close connection immediately", func(t *testing.T) {
		proxyClient := getProxyClient()
		t.Run("http", func(t *testing.T) {
			for i := 0; i < 10; i++ {
				testSendRequest(t, httpEndpoint, proxyClient, "ok")
			}
		})
		t.Run("http wait server closed", func(t *testing.T) {
			for i := 0; i < 10; i++ {
				testSendRequest(t, httpEndpoint, proxyClient, "ok")
				time.Sleep(time.Millisecond * 20)
			}
		})
		t.Run("https", func(t *testing.T) {
			for i := 0; i < 10; i++ {
				testSendRequest(t, httpsEndpoint, proxyClient, "ok")
			}
		})
		t.Run("https wait server closed", func(t *testing.T) {
			for i := 0; i < 10; i++ {
				testSendRequest(t, httpsEndpoint, proxyClient, "ok")
				time.Sleep(time.Millisecond * 20)
			}
		})
	})

	t.Run("should trigger disconnect functions when server close connection immediately", func(t *testing.T) {
		proxyClient := getProxyClient()

		t.Run("http", func(t *testing.T) {
			time.Sleep(time.Millisecond * 10)
			testOrderAddonInstance.reset()
			testSendRequest(t, httpEndpoint, proxyClient, "ok")
			time.Sleep(time.Millisecond * 20)
			testOrderAddonInstance.contains(t, "ClientDisconnected")
			testOrderAddonInstance.contains(t, "ServerDisconnected")
			testOrderAddonInstance.before(t, "ServerDisconnected", "ClientDisconnected")
		})

		t.Run("https", func(t *testing.T) {
			time.Sleep(time.Millisecond * 10)
			testOrderAddonInstance.reset()
			testSendRequest(t, httpsEndpoint, proxyClient, "ok")
			time.Sleep(time.Millisecond * 20)
			testOrderAddonInstance.contains(t, "ClientDisconnected")
			testOrderAddonInstance.contains(t, "ServerDisconnected")
			testOrderAddonInstance.before(t, "ServerDisconnected", "ClientDisconnected")
		})
	})
}

func TestProxyClose(t *testing.T) {
	helper := &testProxyHelper{
		server:    &http.Server{},
		proxyAddr: ":29083",
	}
	helper.init(t)
	httpEndpoint := helper.httpEndpoint
	httpsEndpoint := helper.httpsEndpoint
	testProxy := helper.testProxy
	getProxyClient := helper.getProxyClient
	defer helper.ln.Close()
	go helper.server.Serve(helper.ln)
	defer helper.tlsPlainLn.Close()
	go helper.server.Serve(helper.tlsLn)

	errCh := make(chan error)
	go func() {
		err := testProxy.Start()
		errCh <- err
	}()

	time.Sleep(time.Millisecond * 10) // wait for test proxy startup

	proxyClient := getProxyClient()
	testSendRequest(t, httpEndpoint, proxyClient, "ok")
	testSendRequest(t, httpsEndpoint, proxyClient, "ok")

	if err := testProxy.Close(); err != nil {
		t.Fatalf("close got error %v", err)
	}

	select {
	case err := <-errCh:
		if err != http.ErrServerClosed {
			t.Fatalf("expected ErrServerClosed error, but got %v", err)
		}
	case <-time.After(time.Millisecond * 10):
		t.Fatal("close timeout")
	}
}

func TestProxyShutdown(t *testing.T) {
	helper := &testProxyHelper{
		server:    &http.Server{},
		proxyAddr: ":29084",
	}
	helper.init(t)
	httpEndpoint := helper.httpEndpoint
	httpsEndpoint := helper.httpsEndpoint
	testProxy := helper.testProxy
	getProxyClient := helper.getProxyClient
	defer helper.ln.Close()
	go helper.server.Serve(helper.ln)
	defer helper.tlsPlainLn.Close()
	go helper.server.Serve(helper.tlsLn)

	errCh := make(chan error)
	go func() {
		err := testProxy.Start()
		errCh <- err
	}()

	time.Sleep(time.Millisecond * 10) // wait for test proxy startup

	proxyClient := getProxyClient()
	testSendRequest(t, httpEndpoint, proxyClient, "ok")
	testSendRequest(t, httpsEndpoint, proxyClient, "ok")

	if err := testProxy.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown got error %v", err)
	}

	select {
	case err := <-errCh:
		if err != http.ErrServerClosed {
			t.Fatalf("expected ErrServerClosed error, but got %v", err)
		}
	case <-time.After(time.Millisecond * 10):
		t.Fatal("shutdown timeout")
	}
}

func TestOnUpstreamCert(t *testing.T) {
	helper := &testProxyHelper{
		server:    &http.Server{},
		proxyAddr: ":29085",
	}
	helper.init(t)
	httpEndpoint := helper.httpEndpoint
	httpsEndpoint := helper.httpsEndpoint
	testOrderAddonInstance := helper.testOrderAddonInstance
	testProxy := helper.testProxy
	getProxyClient := helper.getProxyClient
	defer helper.ln.Close()
	go helper.server.Serve(helper.ln)
	defer helper.tlsPlainLn.Close()
	go helper.server.Serve(helper.tlsLn)
	go testProxy.Start()
	time.Sleep(time.Millisecond * 10) // wait for test proxy startup

	proxyClient := getProxyClient()

	t.Run("http", func(t *testing.T) {
		time.Sleep(time.Millisecond * 10)
		testOrderAddonInstance.reset()
		testSendRequest(t, httpEndpoint, proxyClient, "ok")
		time.Sleep(time.Millisecond * 10)
		testOrderAddonInstance.before(t, "Requestheaders", "ServerConnected")
	})

	t.Run("https", func(t *testing.T) {
		time.Sleep(time.Millisecond * 10)
		testOrderAddonInstance.reset()
		testSendRequest(t, httpsEndpoint, proxyClient, "ok")
		time.Sleep(time.Millisecond * 10)
		testOrderAddonInstance.before(t, "ServerConnected", "Requestheaders")
		testOrderAddonInstance.contains(t, "TlsEstablishedServer")
	})

}

func TestOffUpstreamCert(t *testing.T) {
	helper := &testProxyHelper{
		server:    &http.Server{},
		proxyAddr: ":29086",
	}
	helper.init(t)
	httpEndpoint := helper.httpEndpoint
	httpsEndpoint := helper.httpsEndpoint
	testOrderAddonInstance := helper.testOrderAddonInstance
	testProxy := helper.testProxy
	testProxy.AddAddon(NewUpstreamCertAddon(false))
	getProxyClient := helper.getProxyClient
	defer helper.ln.Close()
	go helper.server.Serve(helper.ln)
	defer helper.tlsPlainLn.Close()
	go helper.server.Serve(helper.tlsLn)
	go testProxy.Start()
	time.Sleep(time.Millisecond * 10) // wait for test proxy startup

	proxyClient := getProxyClient()

	t.Run("http", func(t *testing.T) {
		time.Sleep(time.Millisecond * 10)
		testOrderAddonInstance.reset()
		testSendRequest(t, httpEndpoint, proxyClient, "ok")
		time.Sleep(time.Millisecond * 10)
		testOrderAddonInstance.before(t, "Requestheaders", "ServerConnected")
	})

	t.Run("https", func(t *testing.T) {
		time.Sleep(time.Millisecond * 10)
		testOrderAddonInstance.reset()
		testSendRequest(t, httpsEndpoint, proxyClient, "ok")
		time.Sleep(time.Millisecond * 10)
		testOrderAddonInstance.before(t, "Requestheaders", "ServerConnected")
		testOrderAddonInstance.contains(t, "TlsEstablishedServer")
	})
}
