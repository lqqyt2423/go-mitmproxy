package test

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/addon"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

type addonTestHelper struct {
	httpServer *http.Server
	httpLn     net.Listener
	httpAddr   string

	proxy     *proxy.Proxy
	proxyAddr string
	client    *http.Client
}

func newAddonTestHelper(t *testing.T) *addonTestHelper {
	t.Helper()
	h := &addonTestHelper{}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=3600")
		w.Header().Set("ETag", `"abc123"`)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("hello world"))
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/echo-headers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		for name, vals := range r.Header {
			for _, v := range vals {
				w.Write([]byte(name + ": " + v + "\n"))
			}
		}
	})

	h.httpServer = &http.Server{Handler: mux}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h.httpLn = ln
	h.httpAddr = ln.Addr().String()
	go h.httpServer.Serve(ln)

	opts := &proxy.Options{
		Addr:              "127.0.0.1:0",
		StreamLargeBodies: 1024 * 1024 * 5,
	}
	p, err := proxy.NewProxy(opts)
	if err != nil {
		t.Fatal(err)
	}
	h.proxy = p

	return h
}

func (h *addonTestHelper) startProxy(t *testing.T) {
	t.Helper()
	pln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h.proxyAddr = pln.Addr().String()
	h.proxy.Opts.Listener = pln
	go h.proxy.Start()

	proxyURL, _ := url.Parse("http://" + h.proxyAddr)
	h.client = &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

func (h *addonTestHelper) close() {
	h.httpServer.Close()
	h.httpLn.Close()
	if h.proxy.Opts != nil && h.proxy.Opts.Listener != nil {
		h.proxy.Opts.Listener.Close()
	}
}

func (h *addonTestHelper) get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := h.client.Get("http://" + h.httpAddr + path)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestBlockListIntegration(t *testing.T) {
	h := newAddonTestHelper(t)
	defer h.close()

	bl := addon.NewBlockList()
	bl.AddRule(h.httpAddr, "", 403, "Blocked")
	h.proxy.AddAddon(bl)
	h.proxy.AddAddon(&proxy.LogAddon{})
	h.startProxy(t)

	resp := h.get(t, "/")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 403 {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	if string(body) != "Blocked" {
		t.Errorf("expected 'Blocked', got '%s'", string(body))
	}
}

func TestNoCachingIntegration(t *testing.T) {
	h := newAddonTestHelper(t)
	defer h.close()

	h.proxy.AddAddon(&proxy.LogAddon{})
	h.proxy.AddAddon(addon.NewNoCaching())
	h.startProxy(t)

	resp := h.get(t, "/")
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.Header.Get("Cache-Control") != "no-cache, no-store, must-revalidate" {
		t.Errorf("Cache-Control should be overridden, got: %s", resp.Header.Get("Cache-Control"))
	}
	if resp.Header.Get("Etag") != "" {
		t.Error("ETag should be stripped")
	}
}

func TestRewriteIntegration(t *testing.T) {
	h := newAddonTestHelper(t)
	defer h.close()

	rw := addon.NewRewrite()
	rw.Items = append(rw.Items, &addon.RewriteItem{
		Enable: true,
		From:   addon.RewriteMatch{Host: "*"},
		Rules: []addon.RewriteAction{
			{Type: "addHeader", Target: "response", Name: "X-Proxy", Value: "go-mitmproxy"},
		},
	})

	h.proxy.AddAddon(&proxy.LogAddon{})
	h.proxy.AddAddon(rw)
	h.startProxy(t)

	resp := h.get(t, "/json")
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.Header.Get("X-Proxy") != "go-mitmproxy" {
		t.Errorf("expected X-Proxy header, got: '%s'", resp.Header.Get("X-Proxy"))
	}
}

func TestBlockListPriorityOverRewrite(t *testing.T) {
	h := newAddonTestHelper(t)
	defer h.close()

	// Block first, then rewrite — block should win
	bl := addon.NewBlockList()
	bl.AddRule(h.httpAddr, "", 403, "Blocked")
	h.proxy.AddAddon(bl)

	rw := addon.NewRewrite()
	rw.Items = append(rw.Items, &addon.RewriteItem{
		Enable: true,
		From:   addon.RewriteMatch{Host: "*"},
		Rules: []addon.RewriteAction{
			{Type: "addHeader", Target: "response", Name: "X-Should-Not-Exist", Value: "yes"},
		},
	})
	h.proxy.AddAddon(rw)
	h.proxy.AddAddon(&proxy.LogAddon{})
	h.startProxy(t)

	resp := h.get(t, "/")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 403 {
		t.Errorf("block list should take priority, got status %d", resp.StatusCode)
	}
	if string(body) != "Blocked" {
		t.Errorf("expected 'Blocked', got '%s'", string(body))
	}
	// Rewrite should NOT have applied
	if resp.Header.Get("X-Should-Not-Exist") != "" {
		t.Error("rewrite should not apply when blocked")
	}
}

func TestThrottleLatencyIntegration(t *testing.T) {
	h := newAddonTestHelper(t)
	defer h.close()

	throttle := addon.NewThrottle(addon.ThrottleProfile{
		Name:         "test",
		DownloadKbps: 100000,
		UploadKbps:   100000,
		LatencyMs:    100,
	})

	h.proxy.AddAddon(&proxy.LogAddon{})
	h.proxy.AddAddon(throttle)
	h.startProxy(t)

	start := time.Now()
	resp := h.get(t, "/")
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	elapsed := time.Since(start)

	if elapsed.Milliseconds() < 80 {
		t.Errorf("expected at least 80ms with throttle latency, got %dms", elapsed.Milliseconds())
	}
}
