package web

import (
	"embed"
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
	upgrader *websocket.Upgrader

	conns   []*concurrentConn
	connsMu sync.RWMutex
}

func NewWebAddon(addr string) *WebAddon {
	web := new(WebAddon)
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

	server := &http.Server{Addr: addr, Handler: serverMux}
	web.conns = make([]*concurrentConn, 0)

	go func() {
		log.Infof("web interface start listen at %v\n", addr)
		err := server.ListenAndServe()
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

func (web *WebAddon) sendFlow(f *proxy.Flow, msgFn func() *messageFlow) bool {
	web.connsMu.RLock()
	conns := web.conns
	web.connsMu.RUnlock()

	if len(conns) == 0 {
		return false
	}

	msg := msgFn()
	for _, c := range conns {
		c.writeMessage(msg, f)
	}

	return true
}

func (web *WebAddon) Requestheaders(f *proxy.Flow) {
	if f.ConnContext.ClientConn.Tls {
		web.forEachConn(func(c *concurrentConn) {
			c.trySendConnMessage(f)
		})
	}

	web.sendFlow(f, func() *messageFlow {
		return newMessageFlow(messageTypeRequest, f)
	})
}

func (web *WebAddon) Request(f *proxy.Flow) {
	web.sendFlow(f, func() *messageFlow {
		return newMessageFlow(messageTypeRequestBody, f)
	})
}

func (web *WebAddon) Responseheaders(f *proxy.Flow) {
	if !f.ConnContext.ClientConn.Tls {
		web.forEachConn(func(c *concurrentConn) {
			c.trySendConnMessage(f)
		})
	}

	web.sendFlow(f, func() *messageFlow {
		return newMessageFlow(messageTypeResponse, f)
	})
}

func (web *WebAddon) Response(f *proxy.Flow) {
	web.sendFlow(f, func() *messageFlow {
		return newMessageFlow(messageTypeResponseBody, f)
	})
}

func (web *WebAddon) ServerDisconnected(connCtx *proxy.ConnContext) {
	web.forEachConn(func(c *concurrentConn) {
		c.whenConnClose(connCtx)
	})
}
