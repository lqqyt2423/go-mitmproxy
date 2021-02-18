package web

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/lqqyt2423/go-mitmproxy/flow"
)

type concurrentConn struct {
	conn *websocket.Conn
	mu   sync.Mutex

	waitChans   map[string]chan interface{}
	waitChansMu sync.Mutex

	interceptUri string
}

func newConn(c *websocket.Conn) *concurrentConn {
	return &concurrentConn{
		conn:      c,
		waitChans: make(map[string]chan interface{}),
	}
}

func (c *concurrentConn) writeMessage(msg *message, f *flow.Flow) {
	if c.isIntercpt(f, msg) {
		msg.waitIntercept = 1
	}

	c.mu.Lock()
	err := c.conn.WriteMessage(websocket.BinaryMessage, msg.bytes())
	c.mu.Unlock()
	if err != nil {
		log.Error(err)
		return
	}

	if msg.waitIntercept == 1 {
		c.waitIntercept(f, msg)
	}
}

func (c *concurrentConn) readloop() {
	for {
		mt, data, err := c.conn.ReadMessage()
		if err != nil {
			log.Error(err)
			break
		}

		if mt != websocket.BinaryMessage {
			log.Warn("not BinaryMessage, skip")
			continue
		}

		msg := parseMessage(data)
		if msg == nil {
			log.Warn("parseMessage error, skip")
			continue
		}

		if msg.mType == messageTypeChangeInterceptUri {
			interceptUri := ""
			if len(msg.content) > 0 {
				interceptUri = string(msg.content)
			}
			c.interceptUri = interceptUri
			continue
		}

		if msg.mType == messageTypeChangeRequest {
			req := new(flow.Request)
			err := json.Unmarshal(msg.content, req)
			if err != nil {
				log.Error(err)
				continue
			}

			ch := c.initWaitChan(msg.id.String())
			go func(req *flow.Request, ch chan<- interface{}) {
				ch <- req
			}(req, ch)
		}
	}
}

func (c *concurrentConn) initWaitChan(key string) chan interface{} {
	c.waitChansMu.Lock()
	defer c.waitChansMu.Unlock()

	if ch, ok := c.waitChans[key]; ok {
		return ch
	}
	ch := make(chan interface{})
	c.waitChans[key] = ch
	return ch
}

// 是否拦截
func (c *concurrentConn) isIntercpt(f *flow.Flow, after *message) bool {
	if after.mType != messageTypeRequest {
		return false
	}

	if c.interceptUri == "" {
		return false
	}
	if strings.Contains(f.Request.URL.String(), c.interceptUri) {
		return true
	}
	return false
}

// 拦截
func (c *concurrentConn) waitIntercept(f *flow.Flow, after *message) {
	log.Infof("waiting Intercept: %s\n", f.Request.URL)
	ch := c.initWaitChan(f.Id.String())
	req := (<-ch).(*flow.Request)
	log.Infof("waited Intercept: %s\n", f.Request.URL)

	f.Request.Method = req.Method
	f.Request.URL = req.URL
	f.Request.Header = req.Header
}
