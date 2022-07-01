package proxy

import (
	"crypto/tls"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
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
		resp, err := http.Get(httpEndpoint)
		handleError(t, err)
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		handleError(t, err)
		if string(body) != "ok" {
			t.Fatal("expected ok, bug got", string(body))
		}
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
			req, err := http.NewRequest("GET", httpsEndpoint, nil)
			handleError(t, err)
			resp, err := client.Do(req)
			handleError(t, err)
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			handleError(t, err)
			if string(body) != "ok" {
				t.Fatal("expected ok, bug got", string(body))
			}
		})
	})

	// start proxy
	testProxy, err := NewProxy(&Options{
		Addr:        ":29080", // some random port
		SslInsecure: true,
	})
	handleError(t, err)
	testProxy.AddAddon(&LogAddon{})
	go testProxy.Start()
	time.Sleep(time.Millisecond * 10) // wait for test proxy startup

	t.Run("test proxy", func(t *testing.T) {
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				Proxy: func(r *http.Request) (*url.URL, error) {
					return url.Parse("http://127.0.0.1:29080")
				},
			},
		}

		t.Run("can proxy http", func(t *testing.T) {
			req, err := http.NewRequest("GET", httpEndpoint, nil)
			handleError(t, err)
			resp, err := client.Do(req)
			handleError(t, err)
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			handleError(t, err)
			if string(body) != "ok" {
				t.Fatal("expected ok, bug got", string(body))
			}
		})

		t.Run("can proxy https", func(t *testing.T) {
			req, err := http.NewRequest("GET", httpsEndpoint, nil)
			handleError(t, err)
			resp, err := client.Do(req)
			handleError(t, err)
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			handleError(t, err)
			if string(body) != "ok" {
				t.Fatal("expected ok, bug got", string(body))
			}
		})
	})
}
