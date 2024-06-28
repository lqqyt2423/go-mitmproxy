package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	log "github.com/sirupsen/logrus"
)

//go:embed client/build
var assets embed.FS

type WebAddon struct {
	proxy.BaseAddon

	server   *http.Server
	upgrader *websocket.Upgrader

	conns   []*concurrentConn
	connsMu sync.RWMutex

	flowMessageState map[*proxy.Flow]messageType
	flowMu           sync.Mutex
}

func NewWebAddon(addr string) *WebAddon {
	web := &WebAddon{
		flowMessageState: make(map[*proxy.Flow]messageType),
	}

	web.upgrader = &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	serverMux := new(http.ServeMux)
	serverMux.HandleFunc("/echo", web.echo)

	fsys, err := fs.Sub(assets, "client/build")
	if err != nil {
		panic(err)
	}
	serverMux.Handle("/", http.FileServer(http.FS(fsys)))

	web.server = &http.Server{Addr: addr, Handler: serverMux}
	web.conns = make([]*concurrentConn, 0)

	go func() {
		log.Infof("web interface start listen at %v\n", addr)
		err := web.server.ListenAndServe()
		log.Error(err)
	}()

	return web
}

func (web *WebAddon) echo(w http.ResponseWriter, r *http.Request) {
	c, err := web.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}

	conn := newConn(c)
	web.addConn(conn)
	defer func() {
		web.removeConn(conn)
		c.Close()
	}()

	conn.readloop()
}

func (web *WebAddon) addConn(c *concurrentConn) {
	web.connsMu.Lock()
	web.conns = append(web.conns, c)
	web.connsMu.Unlock()
}

func (web *WebAddon) removeConn(conn *concurrentConn) {
	web.connsMu.Lock()
	defer web.connsMu.Unlock()

	index := -1
	for i, c := range web.conns {
		if conn == c {
			index = i
			break
		}
	}

	if index == -1 {
		return
	}
	web.conns = append(web.conns[:index], web.conns[index+1:]...)
}

func (web *WebAddon) forEachConn(do func(c *concurrentConn)) bool {
	web.connsMu.RLock()
	conns := web.conns
	web.connsMu.RUnlock()
	if len(conns) == 0 {
		return false
	}
	for _, c := range conns {
		do(c)
	}
	return true
}

func (web *WebAddon) sendFlowMayWait(f *proxy.Flow, msgFn func() (*messageFlow, error)) {
	web.connsMu.RLock()
	conns := web.conns
	web.connsMu.RUnlock()

	if len(conns) == 0 {
		return
	}

	msg, err := msgFn()
	if err != nil {
		log.Error(fmt.Errorf("web addon gen msg: %w", err))
		return
	}
	for _, c := range conns {
		c.writeMessageMayWait(msg, f)
	}
}

func (web *WebAddon) Requestheaders(f *proxy.Flow) {
	web.flowMu.Lock()
	web.flowMessageState[f] = messageType(0)
	web.flowMu.Unlock()

	go func() {
		<-f.Done()
		web.sendMessageUntil(f, messageTypeResponseBody)

		web.flowMu.Lock()
		delete(web.flowMessageState, f)
		web.flowMu.Unlock()
	}()

	if f.ConnContext.ClientConn.Tls {
		web.forEachConn(func(c *concurrentConn) {
			c.trySendConnMessage(f)
		})
	}
}

func (web *WebAddon) Request(f *proxy.Flow) {
	if web.isIntercpt(f, messageTypeRequestBody) {
		web.sendFlowMayWait(f, func() (*messageFlow, error) {
			return newMessageFlow(messageTypeRequest, f)
		})
		web.sendFlowMayWait(f, func() (*messageFlow, error) {
			return newMessageFlow(messageTypeRequestBody, f)
		})
	}
}

func (web *WebAddon) Responseheaders(f *proxy.Flow) {
	if !f.ConnContext.ClientConn.Tls {
		web.forEachConn(func(c *concurrentConn) {
			c.trySendConnMessage(f)
		})
	}

	web.sendMessageUntil(f, messageTypeRequestBody)
}

func (web *WebAddon) Response(f *proxy.Flow) {
	if web.isIntercpt(f, messageTypeResponseBody) {
		web.sendFlowMayWait(f, func() (*messageFlow, error) {
			return newMessageFlow(messageTypeResponse, f)
		})
		web.sendFlowMayWait(f, func() (*messageFlow, error) {
			return newMessageFlow(messageTypeResponseBody, f)
		})
	}
}

func (web *WebAddon) ServerDisconnected(connCtx *proxy.ConnContext) {
	web.forEachConn(func(c *concurrentConn) {
		c.whenConnClose(connCtx)
	})
}

func (web *WebAddon) isIntercpt(f *proxy.Flow, mType messageType) bool {
	web.connsMu.RLock()
	conns := web.conns
	web.connsMu.RUnlock()

	if len(conns) == 0 {
		return false
	}

	for _, c := range conns {
		if c.isIntercpt(f, mType) {
			return true
		}
	}
	return false
}

func (web *WebAddon) sendFlow(msgFn func() (*messageFlow, error)) {
	web.connsMu.RLock()
	conns := web.conns
	web.connsMu.RUnlock()

	if len(conns) == 0 {
		return
	}

	msg, err := msgFn()
	if err != nil {
		log.Error(fmt.Errorf("web addon gen msg: %w", err))
		return
	}
	for _, c := range conns {
		c.writeMessage(msg)
	}
}

func (web *WebAddon) sendMessageUntil(f *proxy.Flow, mType messageType) {
	web.flowMu.Lock()
	if web.flowMessageState[f] >= mType {
		web.flowMu.Unlock()
		return
	}
	state := web.flowMessageState[f] + 1
	web.flowMu.Unlock()

	for ; state <= mType; state++ {
		web.sendFlow(func() (*messageFlow, error) {
			return newMessageFlow(state, f)
		})
	}

	web.flowMu.Lock()
	web.flowMessageState[f] = mType
	web.flowMu.Unlock()
}
