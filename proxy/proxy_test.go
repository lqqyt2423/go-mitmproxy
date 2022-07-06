package proxy

import (
	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
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
	body, err := ioutil.ReadAll(resp.Body)
	handleError(t, err)
	if string(body) != bodyWant {
		t.Fatalf("expected %s, but got %s", bodyWant, body)
	}
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
	// start http server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	handleError(t, err)
	defer ln.Close()
	go http.Serve(ln, nil)

	// start https server
	tlsLn, err := net.Listen("tcp", "127.0.0.1:0")
	handleError(t, err)
	defer tlsLn.Close()
	ca, err := cert.NewCAMemory()
	handleError(t, err)
	cert, err := ca.GetCert("localhost")
	handleError(t, err)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
	}
	go http.Serve(tls.NewListener(tlsLn, tlsConfig), nil)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	httpEndpoint := "http://" + ln.Addr().String() + "/"
	httpsPort := tlsLn.Addr().(*net.TCPAddr).Port
	httpsEndpoint := "https://localhost:" + strconv.Itoa(httpsPort) + "/"

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

	// start proxy
	testProxy, err := NewProxy(&Options{
		Addr:        ":29080", // some random port
		SslInsecure: true,
	})
	handleError(t, err)
	testProxy.AddAddon(&interceptAddon{})
	testOrderAddonInstance := &testOrderAddon{
		orders: make([]string, 0),
	}
	testProxy.AddAddon(testOrderAddonInstance)
	go testProxy.Start()
	time.Sleep(time.Millisecond * 10) // wait for test proxy startup

	getProxyClient := func() *http.Client {
		return &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				Proxy: func(r *http.Request) (*url.URL, error) {
					return url.Parse("http://127.0.0.1:29080")
				},
			},
		}
	}

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
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	server := &http.Server{
		Handler: mux,
	}
	server.SetKeepAlivesEnabled(false)

	// start http server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	handleError(t, err)
	defer ln.Close()
	go server.Serve(ln)

	// start https server
	tlsLn, err := net.Listen("tcp", "127.0.0.1:0")
	handleError(t, err)
	defer tlsLn.Close()
	ca, err := cert.NewCAMemory()
	handleError(t, err)
	cert, err := ca.GetCert("localhost")
	handleError(t, err)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
	}
	go server.Serve(tls.NewListener(tlsLn, tlsConfig))

	httpEndpoint := "http://" + ln.Addr().String() + "/"
	httpsPort := tlsLn.Addr().(*net.TCPAddr).Port
	httpsEndpoint := "https://localhost:" + strconv.Itoa(httpsPort) + "/"

	// start proxy
	testProxy, err := NewProxy(&Options{
		Addr:        ":29081", // some random port
		SslInsecure: true,
	})
	handleError(t, err)
	testProxy.AddAddon(&interceptAddon{})
	testOrderAddonInstance := &testOrderAddon{
		orders: make([]string, 0),
	}
	testProxy.AddAddon(testOrderAddonInstance)
	go testProxy.Start()
	time.Sleep(time.Millisecond * 10) // wait for test proxy startup

	getProxyClient := func() *http.Client {
		return &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				Proxy: func(r *http.Request) (*url.URL, error) {
					return url.Parse("http://127.0.0.1:29081")
				},
			},
		}
	}

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
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	server := &http.Server{
		Handler:     mux,
		IdleTimeout: time.Millisecond * 10,
	}

	// start http server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	handleError(t, err)
	defer ln.Close()
	go server.Serve(ln)

	// start https server
	tlsLn, err := net.Listen("tcp", "127.0.0.1:0")
	handleError(t, err)
	defer tlsLn.Close()
	ca, err := cert.NewCAMemory()
	handleError(t, err)
	cert, err := ca.GetCert("localhost")
	handleError(t, err)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
	}
	go server.Serve(tls.NewListener(tlsLn, tlsConfig))

	httpEndpoint := "http://" + ln.Addr().String() + "/"
	httpsPort := tlsLn.Addr().(*net.TCPAddr).Port
	httpsEndpoint := "https://localhost:" + strconv.Itoa(httpsPort) + "/"

	// start proxy
	testProxy, err := NewProxy(&Options{
		Addr:        ":29082", // some random port
		SslInsecure: true,
	})
	handleError(t, err)
	testProxy.AddAddon(&interceptAddon{})
	testOrderAddonInstance := &testOrderAddon{
		orders: make([]string, 0),
	}
	testProxy.AddAddon(testOrderAddonInstance)
	go testProxy.Start()
	time.Sleep(time.Millisecond * 10) // wait for test proxy startup

	getProxyClient := func() *http.Client {
		return &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				Proxy: func(r *http.Request) (*url.URL, error) {
					return url.Parse("http://127.0.0.1:29082")
				},
			},
		}
	}

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
