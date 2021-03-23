package web

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/lqqyt2423/go-mitmproxy/flow"
)

type breakPointRule struct {
	Method string `json:"method"`
	URL    string `json:"url"`
	Action int    `json:"action"` // 1 - change request 2 - change response 3 - both
}

type concurrentConn struct {
	conn *websocket.Conn
	mu   sync.Mutex

	waitChans   map[string]chan interface{}
	waitChansMu sync.Mutex

	breakPointRules []*breakPointRule
}

func newConn(c *websocket.Conn) *concurrentConn {
	return &concurrentConn{
		conn:      c,
		waitChans: make(map[string]chan interface{}),
	}
}

func (c *concurrentConn) writeMessage(msg *messageFlow, f *flow.Flow) {
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

		if msgEdit, ok := msg.(*messageEdit); ok {
			ch := c.initWaitChan(msgEdit.id.String())
			go func(m *messageEdit, ch chan<- interface{}) {
				ch <- m
			}(msgEdit, ch)
		} else if msgMeta, ok := msg.(*messageMeta); ok {
			c.breakPointRules = msgMeta.breakPointRules
		} else {
			log.Warn("invalid message, skip")
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
func (c *concurrentConn) isIntercpt(f *flow.Flow, after *messageFlow) bool {
	if after.mType != messageTypeRequest {
		return false
	}

	return false
}

// 拦截
func (c *concurrentConn) waitIntercept(f *flow.Flow, after *messageFlow) {
	log.Infof("waiting Intercept: %s\n", f.Request.URL)
	ch := c.initWaitChan(f.Id.String())
	msg := (<-ch).(*messageEdit)
	log.Infof("waited Intercept: %s\n", f.Request.URL)

	// f.Request.Method = req.Method
	// f.Request.URL = req.URL
	// f.Request.Header = req.Header

	log.Infof("waitIntercept: %v", msg)
}
