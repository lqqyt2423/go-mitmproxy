package web

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/lqqyt2423/go-mitmproxy/addon"
	"github.com/lqqyt2423/go-mitmproxy/flow"
	_log "github.com/sirupsen/logrus"
)

var log = _log.WithField("at", "web addon")

func (web *WebAddon) echo(w http.ResponseWriter, r *http.Request) {
	c, err := web.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}

	web.addConn(c)
	defer func() {
		web.removeConn(c)
		c.Close()
	}()

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

type concurrentConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

type WebAddon struct {
	addon.Base
	addr      string
	upgrader  *websocket.Upgrader
	serverMux *http.ServeMux
	server    *http.Server

	conns   []*concurrentConn
	connsMu sync.RWMutex
}

func NewWebAddon() *WebAddon {
	web := new(WebAddon)
	web.addr = ":9081"
	web.upgrader = &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	web.serverMux = new(http.ServeMux)
	web.serverMux.HandleFunc("/echo", web.echo)
	web.serverMux.Handle("/", http.FileServer(http.Dir("addon/web/client/build")))

	web.server = &http.Server{Addr: web.addr, Handler: web.serverMux}
	log = log.WithField("in", "WebAddon")
	web.conns = make([]*concurrentConn, 0)

	go func() {
		log.Infof("server start listen at %v\n", web.addr)
		err := web.server.ListenAndServe()
		log.Error(err)
	}()

	return web
}

func (web *WebAddon) addConn(c *websocket.Conn) {
	web.connsMu.Lock()
	web.conns = append(web.conns, &concurrentConn{conn: c})
	web.connsMu.Unlock()
}

func (web *WebAddon) removeConn(conn *websocket.Conn) {
	web.connsMu.Lock()
	defer web.connsMu.Unlock()

	index := -1
	for i, c := range web.conns {
		if conn == c.conn {
			index = i
			break
		}
	}

	if index == -1 {
		return
	}
	web.conns = append(web.conns[:index], web.conns[index+1:]...)
}

func (web *WebAddon) sendFlow(msgFn func() *message) {
	web.connsMu.RLock()
	conns := web.conns
	web.connsMu.RUnlock()

	if len(conns) == 0 {
		return
	}

	msg := msgFn()
	for _, c := range conns {
		c.mu.Lock()
		c.conn.WriteMessage(websocket.BinaryMessage, msg.bytes())
		c.mu.Unlock()
	}
}

func (web *WebAddon) Request(f *flow.Flow) {
	web.sendFlow(func() *message {
		return newMessageRequest(f)
	})
}

func (web *WebAddon) Responseheaders(f *flow.Flow) {
	web.sendFlow(func() *message {
		return newMessageResponse(f)
	})
}

func (web *WebAddon) Response(f *flow.Flow) {
	web.sendFlow(func() *message {
		return newMessageResponseBody(f)
	})
}
