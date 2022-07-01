package proxy

import (
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
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
}

func (addon *testOrderAddon) ClientConnected(*ClientConn) {
	addon.orders = append(addon.orders, "ClientConnected")
}
func (addon *testOrderAddon) ClientDisconnected(*ClientConn) {
	addon.orders = append(addon.orders, "ClientDisconnected")
}
func (addon *testOrderAddon) ServerConnected(*ConnContext) {
	addon.orders = append(addon.orders, "ServerConnected")
}
func (addon *testOrderAddon) ServerDisconnected(*ConnContext) {
	addon.orders = append(addon.orders, "ServerDisconnected")
}
func (addon *testOrderAddon) TlsEstablishedServer(*ConnContext) {
	addon.orders = append(addon.orders, "TlsEstablishedServer")
}
func (addon *testOrderAddon) Requestheaders(*Flow) {
	addon.orders = append(addon.orders, "Requestheaders")
}
func (addon *testOrderAddon) Request(*Flow) {
	addon.orders = append(addon.orders, "Request")
}
func (addon *testOrderAddon) Responseheaders(*Flow) {
	addon.orders = append(addon.orders, "Responseheaders")
}
func (addon *testOrderAddon) Response(*Flow) {
	addon.orders = append(addon.orders, "Response")
}
func (addon *testOrderAddon) StreamRequestModifier(f *Flow, in io.Reader) io.Reader {
	addon.orders = append(addon.orders, "StreamRequestModifier")
	return in
}
func (addon *testOrderAddon) StreamResponseModifier(f *Flow, in io.Reader) io.Reader {
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
	testProxy.AddAddon(&LogAddon{})
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
			// todo: fail
			t.Run("https", func(t *testing.T) {
				httpsEndpoint := "https://some-wrong-host/"
				testSendRequest(t, httpsEndpoint+"intercept-request", proxyClient, "intercept-request")
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

	t.Run("test proxy when disable client keep alive", func(t *testing.T) {
		proxyClient := getProxyClient()
		proxyClient.Transport.(*http.Transport).DisableKeepAlives = true

		// todo: fail
		t.Run("http", func(t *testing.T) {
			testSendRequest(t, httpEndpoint, proxyClient, "ok")
		})

		// todo: fail
		t.Run("https", func(t *testing.T) {
			testSendRequest(t, httpsEndpoint, proxyClient, "ok")
		})
	})

	t.Run("test addon execute order", func(t *testing.T) {
		proxyClient := getProxyClient()
		proxyClient.Transport.(*http.Transport).DisableKeepAlives = true

		// todo: fail
		t.Run("http", func(t *testing.T) {
			testOrderAddonInstance.orders = make([]string, 0)
			testSendRequest(t, httpEndpoint, proxyClient, "ok")
			wantOrders := []string{
				"ClientConnected",
				"Requestheaders",
				"Request",
				"StreamRequestModifier",
				"ServerConnected",
				"Responseheaders",
				"Response",
				"StreamResponseModifier",
				"ClientDisconnected",
				"ServerDisconnected",
			}
			if !reflect.DeepEqual(testOrderAddonInstance.orders, wantOrders) {
				t.Fatalf("expected order %v, but got order %v", wantOrders, testOrderAddonInstance.orders)
			}
		})
	})
}
