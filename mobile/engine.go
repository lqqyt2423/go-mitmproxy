package mobile

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/cert"
	"github.com/lqqyt2423/go-mitmproxy/internal/helper"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"github.com/lqqyt2423/go-mitmproxy/web"
)

const (
	defaultFlowStoreLimit    = 5000
	defaultStreamLargeBodies = 1024 * 1024 * 5 // 5MB
)

// Engine wraps proxy.Proxy for mobile platforms.
// All exported methods use gomobile-compatible types only.
type Engine struct {
	proxy   *proxy.Proxy
	handler EventHandler
	store   *flowStore

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc

	// config fields set before Start()
	addr              string
	certPath          string
	sslInsecure       bool
	streamLargeBodies int64
	ignoreHosts       []string
	allowHosts        []string
	flowStoreLimit    int
	webAddr           string // optional: enable WebAddon for IPC (e.g. "127.0.0.1:9081")
}

// NewEngine creates a new proxy engine for mobile use.
// addr: listen address, e.g. "127.0.0.1:9080"
// certPath: certificate storage path; empty string uses in-memory CA
// handler: event callback interface implemented by Swift/Kotlin
func NewEngine(addr string, certPath string, handler EventHandler) (*Engine, error) {
	if handler == nil {
		return nil, fmt.Errorf("handler must not be nil")
	}
	return &Engine{
		addr:              addr,
		certPath:          certPath,
		handler:           handler,
		streamLargeBodies: defaultStreamLargeBodies,
		flowStoreLimit:    defaultFlowStoreLimit,
	}, nil
}

// SetSslInsecure controls whether to verify upstream server TLS certificates.
// Must be called before Start().
func (e *Engine) SetSslInsecure(insecure bool) {
	e.sslInsecure = insecure
}

// SetStreamLargeBodies sets the threshold in bytes for streaming large bodies.
// Bodies larger than this are streamed without buffering.
// Must be called before Start().
func (e *Engine) SetStreamLargeBodies(bytes int64) {
	e.streamLargeBodies = bytes
}

// SetFlowStoreLimit sets the maximum number of flows kept in memory for body retrieval.
// Default: 5000 for macOS, set to ~100 for iOS Network Extension.
// Must be called before Start().
func (e *Engine) SetFlowStoreLimit(limit int) {
	e.flowStoreLimit = limit
}

// AddIgnoreHost adds a host pattern to the ignore list (these hosts won't be intercepted).
// Supports wildcards, e.g. "*.example.com".
// Must be called before Start().
func (e *Engine) AddIgnoreHost(pattern string) {
	e.ignoreHosts = append(e.ignoreHosts, pattern)
}

// AddAllowHost adds a host pattern to the allow list (only these hosts will be intercepted).
// Must be called before Start().
func (e *Engine) AddAllowHost(pattern string) {
	e.allowHosts = append(e.allowHosts, pattern)
}

// SetWebAddr enables the WebAddon for real-time flow streaming via WebSocket.
// addr: e.g. "127.0.0.1:9081". Used for iOS App <-> Extension IPC.
// Must be called before Start().
func (e *Engine) SetWebAddr(addr string) {
	e.webAddr = addr
}

// Start initializes and starts the proxy in a background goroutine.
// Returns immediately. Proxy state changes are reported via EventHandler.OnStateChanged.
func (e *Engine) Start() error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("engine already running")
	}
	e.mu.Unlock()

	e.handler.OnStateChanged("starting", "")

	opts := &proxy.Options{
		Addr:              e.addr,
		StreamLargeBodies: e.streamLargeBodies,
		SslInsecure:       e.sslInsecure,
	}

	if e.certPath != "" {
		opts.CaRootPath = e.certPath
	} else {
		opts.NewCaFunc = func() (cert.CA, error) {
			return cert.NewSelfSignCAMemory()
		}
	}

	p, err := proxy.NewProxy(opts)
	if err != nil {
		e.handler.OnStateChanged("error", err.Error())
		return err
	}

	// Host filtering
	if len(e.ignoreHosts) > 0 {
		hosts := e.ignoreHosts
		p.SetShouldInterceptRule(func(req *http.Request) bool {
			return !helper.MatchHost(req.Host, hosts)
		})
	}
	if len(e.allowHosts) > 0 {
		hosts := e.allowHosts
		p.SetShouldInterceptRule(func(req *http.Request) bool {
			return helper.MatchHost(req.Host, hosts)
		})
	}

	// Flow store and bridge addon
	e.store = newFlowStore(e.flowStoreLimit)
	bridge := &bridgeAddon{
		handler: e.handler,
		store:   e.store,
	}
	p.AddAddon(bridge)

	// Optional WebAddon for IPC (iOS Extension -> App via WebSocket)
	if e.webAddr != "" {
		p.AddAddon(web.NewWebAddon(e.webAddr))
	}

	e.proxy = p

	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel

	go func() {
		e.mu.Lock()
		e.running = true
		e.mu.Unlock()

		e.handler.OnStateChanged("running", e.addr)

		err := p.Start()

		e.mu.Lock()
		e.running = false
		e.mu.Unlock()

		select {
		case <-ctx.Done():
			e.handler.OnStateChanged("stopped", "")
		default:
			if err != nil {
				e.handler.OnStateChanged("error", err.Error())
			}
		}
	}()

	return nil
}

// Stop gracefully stops the proxy.
func (e *Engine) Stop() error {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return nil
	}
	e.mu.Unlock()

	e.handler.OnStateChanged("stopping", "")

	if e.cancel != nil {
		e.cancel()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if e.proxy != nil {
		return e.proxy.Shutdown(ctx)
	}
	return nil
}

// IsRunning returns whether the proxy is currently running.
func (e *Engine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// GetCACertPEM returns the root CA certificate in PEM format.
// Must be called after Start().
func (e *Engine) GetCACertPEM() (string, error) {
	if e.proxy == nil {
		return "", fmt.Errorf("engine not started")
	}
	return getCACertPEM(e.proxy)
}

// GetCACertDER returns the root CA certificate in DER format.
// Useful for iOS .mobileconfig profile generation.
// Must be called after Start().
func (e *Engine) GetCACertDER() ([]byte, error) {
	if e.proxy == nil {
		return nil, fmt.Errorf("engine not started")
	}
	return getCACertDER(e.proxy)
}

// GetFlowRequestBody returns the request body for a given flow ID.
func (e *Engine) GetFlowRequestBody(flowID string) ([]byte, error) {
	f, err := e.store.Get(flowID)
	if err != nil {
		return nil, err
	}
	if f.Request == nil || f.Request.Body == nil {
		return nil, fmt.Errorf("no request body for flow %s", flowID)
	}
	return f.Request.Body, nil
}

// GetFlowResponseBody returns the response body for a given flow ID.
func (e *Engine) GetFlowResponseBody(flowID string) ([]byte, error) {
	f, err := e.store.Get(flowID)
	if err != nil {
		return nil, err
	}
	if f.Response == nil || f.Response.Body == nil {
		return nil, fmt.Errorf("no response body for flow %s", flowID)
	}
	return f.Response.Body, nil
}

// bridgeAddon forwards proxy events to the EventHandler.
type bridgeAddon struct {
	proxy.BaseAddon
	handler EventHandler
	store   *flowStore
}

func (a *bridgeAddon) Request(f *proxy.Flow) {
	a.store.Put(f)
	j, err := marshalFlowRequest(f)
	if err != nil {
		return
	}
	a.handler.OnFlowRequest(j)
}

func (a *bridgeAddon) Response(f *proxy.Flow) {
	j, err := marshalFlowResponse(f)
	if err != nil {
		return
	}
	a.handler.OnFlowResponse(j)
}

func (a *bridgeAddon) RequestError(f *proxy.Flow, reqErr error) {
	a.handler.OnFlowError(f.Id.String(), reqErr.Error())
}

func (a *bridgeAddon) WebSocketMessage(f *proxy.Flow) {
	j, err := marshalWebSocketMessage(f)
	if err != nil {
		return
	}
	a.handler.OnWebSocketMessage(f.Id.String(), j)
}

func (a *bridgeAddon) SSEMessage(f *proxy.Flow) {
	j, err := marshalSSEEvent(f)
	if err != nil {
		return
	}
	a.handler.OnSSEEvent(f.Id.String(), j)
}
