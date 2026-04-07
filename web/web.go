package web

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lqqyt2423/go-mitmproxy/addon"
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

	// In-memory flow store for repeat/session/statistics
	flowStore    []*proxy.Flow
	flowStoreMap map[string]*proxy.Flow
	flowStoreMu  sync.RWMutex
	flowStoreMax int

	// Annotation storage
	annotations   map[string]*proxy.FlowAnnotation
	annotationsMu sync.RWMutex
}

func NewWebAddon(addr string) *WebAddon {
	web := &WebAddon{
		flowMessageState: make(map[*proxy.Flow]messageType),
		flowStore:        make([]*proxy.Flow, 0),
		flowStoreMap:     make(map[string]*proxy.Flow),
		flowStoreMax:     5000,
		annotations:      make(map[string]*proxy.FlowAnnotation),
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
	conn.onAction = web.handleAction
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

	if f.ConnContext.ClientConn.Tls {
		web.forEachConn(func(c *concurrentConn) {
			c.trySendConnMessage(f)
		})
	}
}

func (web *WebAddon) storeFlow(f *proxy.Flow) {
	web.flowStoreMu.Lock()
	defer web.flowStoreMu.Unlock()

	key := f.Id.String()
	if _, exists := web.flowStoreMap[key]; !exists {
		web.flowStore = append(web.flowStore, f)
		web.flowStoreMap[key] = f

		// LRU eviction
		if len(web.flowStore) > web.flowStoreMax {
			oldest := web.flowStore[0]
			web.flowStore = web.flowStore[1:]
			delete(web.flowStoreMap, oldest.Id.String())
		}
	}
}

func (web *WebAddon) GetFlow(id string) *proxy.Flow {
	web.flowStoreMu.RLock()
	defer web.flowStoreMu.RUnlock()
	return web.flowStoreMap[id]
}

func (web *WebAddon) GetAllFlows() []*proxy.Flow {
	web.flowStoreMu.RLock()
	defer web.flowStoreMu.RUnlock()
	result := make([]*proxy.Flow, len(web.flowStore))
	copy(result, web.flowStore)
	return result
}

func (web *WebAddon) SetAnnotation(flowId string, annotation *proxy.FlowAnnotation) {
	web.annotationsMu.Lock()
	web.annotations[flowId] = annotation
	web.annotationsMu.Unlock()

	// Also set on the flow if it exists in store
	if f := web.GetFlow(flowId); f != nil {
		f.Annotation = annotation
	}
}

func (web *WebAddon) Request(f *proxy.Flow) {
	web.storeFlow(f)

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
	web.storeFlow(f) // update stored flow with response data
	web.sendMessageUntil(f, messageTypeResponseBody)

	if web.isIntercpt(f, messageTypeResponseBody) {
		web.sendFlowMayWait(f, func() (*messageFlow, error) {
			return newMessageFlow(messageTypeResponse, f)
		})
		web.sendFlowMayWait(f, func() (*messageFlow, error) {
			return newMessageFlow(messageTypeResponseBody, f)
		})
	}

	web.flowMu.Lock()
	delete(web.flowMessageState, f)
	web.flowMu.Unlock()
}

func (web *WebAddon) ServerDisconnected(connCtx *proxy.ConnContext) {
	web.forEachConn(func(c *concurrentConn) {
		c.whenConnClose(connCtx)
	})
}

// WebSocketStart 发送 WebSocket 连接建立消息
func (web *WebAddon) WebSocketStart(f *proxy.Flow) {
	web.sendFlow(func() (*messageFlow, error) {
		return newMessageFlow(messageTypeWebSocketStart, f)
	})
}

// WebSocketMessage 发送 WebSocket 消息
func (web *WebAddon) WebSocketMessage(f *proxy.Flow) {
	web.sendFlow(func() (*messageFlow, error) {
		return newMessageFlow(messageTypeWebSocketMessage, f)
	})
}

// WebSocketEnd 发送 WebSocket 连接结束消息
func (web *WebAddon) WebSocketEnd(f *proxy.Flow) {
	web.sendFlow(func() (*messageFlow, error) {
		return newMessageFlow(messageTypeWebSocketEnd, f)
	})
}

// SSEStart 发送 SSE 连接建立消息
func (web *WebAddon) SSEStart(f *proxy.Flow) {
	// 对于 SSE 流，需要先发送 Response 消息（3），因为 SSE 不会触发 Response 事件
	// 确保前端能收到响应头信息
	web.sendMessageUntil(f, messageTypeResponse)

	// 然后发送 SSEStart 消息
	web.sendFlow(func() (*messageFlow, error) {
		return newMessageFlow(messageTypeSSEStart, f)
	})
}

// SSEMessage 发送 SSE 消息
func (web *WebAddon) SSEMessage(f *proxy.Flow) {
	web.sendFlow(func() (*messageFlow, error) {
		return newMessageFlow(messageTypeSSEMessage, f)
	})
}

// SSEEnd 发送 SSE 连接结束消息
func (web *WebAddon) SSEEnd(f *proxy.Flow) {
	// 发送 SSEEnd 消息
	web.sendFlow(func() (*messageFlow, error) {
		return newMessageFlow(messageTypeSSEEnd, f)
	})

	// 清理 flowMessageState，因为 SSE 不会触发 Response 事件来清理
	web.flowMu.Lock()
	delete(web.flowMessageState, f)
	web.flowMu.Unlock()
}

func (web *WebAddon) RequestError(f *proxy.Flow, err error) {
	web.flowMu.Lock()
	delete(web.flowMessageState, f)
	web.flowMu.Unlock()
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

func (web *WebAddon) handleAction(msg *messageAction) {
	switch msg.mType {
	case messageTypeSetAnnotation:
		var annotation proxy.FlowAnnotation
		if err := json.Unmarshal(msg.content, &annotation); err != nil {
			log.Warnf("parse annotation: %v", err)
			return
		}
		web.SetAnnotation(msg.id.String(), &annotation)

	case messageTypeRepeatRequest:
		go web.handleRepeat(msg)

	case messageTypeComposeRequest:
		go web.handleCompose(msg)

	case messageTypeSaveSession:
		go web.handleSaveSession(msg)

	case messageTypeLoadSession:
		go web.handleLoadSession(msg)

	case messageTypeExportHAR:
		go web.handleExportHAR(msg)

	case messageTypeImportHAR:
		go web.handleImportHAR(msg)

	default:
		log.Warnf("unhandled action type: %v", msg.mType)
	}
}

func (web *WebAddon) handleRepeat(msg *messageAction) {
	f := web.GetFlow(msg.id.String())
	if f == nil || f.Request == nil {
		log.Warnf("repeat: flow not found: %s", msg.id)
		return
	}

	req, err := http.NewRequest(f.Request.Method, f.Request.URL.String(), nil)
	if err != nil {
		log.Warnf("repeat: create request: %v", err)
		return
	}
	req.Header = f.Request.Header.Clone()
	if len(f.Request.Body) > 0 {
		req.Body = io.NopCloser(bytes.NewReader(f.Request.Body))
		req.ContentLength = int64(len(f.Request.Body))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Warnf("repeat: send request: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Infof("repeat: %s %s → %d", f.Request.Method, f.Request.URL, resp.StatusCode)
	_ = body // Response captured; in full implementation would create a new Flow and push to clients
}

func (web *WebAddon) handleCompose(msg *messageAction) {
	if msg.content == nil {
		return
	}
	// Parse compose request from content
	var composeReq struct {
		Method string              `json:"method"`
		URL    string              `json:"url"`
		Header map[string][]string `json:"header"`
	}
	if err := json.Unmarshal(msg.content, &composeReq); err != nil {
		log.Warnf("compose: parse: %v", err)
		return
	}

	req, err := http.NewRequest(composeReq.Method, composeReq.URL, nil)
	if err != nil {
		log.Warnf("compose: create request: %v", err)
		return
	}
	for k, vals := range composeReq.Header {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Warnf("compose: send request: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Infof("compose: %s %s → %d (%d bytes)", composeReq.Method, composeReq.URL, resp.StatusCode, len(body))
}

func (web *WebAddon) handleSaveSession(msg *messageAction) {
	var req struct {
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(msg.content, &req); err != nil || req.Filename == "" {
		log.Warnf("save session: invalid request")
		return
	}
	flows := web.GetAllFlows()
	if err := addon.SaveSession(flows, req.Filename); err != nil {
		log.Warnf("save session: %v", err)
		return
	}
	log.Infof("session saved: %s (%d flows)", req.Filename, len(flows))
}

func (web *WebAddon) handleLoadSession(msg *messageAction) {
	var req struct {
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(msg.content, &req); err != nil || req.Filename == "" {
		log.Warnf("load session: invalid request")
		return
	}
	flows, err := addon.LoadSession(req.Filename)
	if err != nil {
		log.Warnf("load session: %v", err)
		return
	}
	for _, f := range flows {
		web.storeFlow(f)
	}
	log.Infof("session loaded: %s (%d flows)", req.Filename, len(flows))
}

func (web *WebAddon) handleExportHAR(msg *messageAction) {
	var req struct {
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(msg.content, &req); err != nil || req.Filename == "" {
		log.Warnf("export HAR: invalid request")
		return
	}
	flows := web.GetAllFlows()
	if err := addon.ExportHAR(flows, req.Filename); err != nil {
		log.Warnf("export HAR: %v", err)
		return
	}
	log.Infof("HAR exported: %s (%d flows)", req.Filename, len(flows))
}

func (web *WebAddon) handleImportHAR(msg *messageAction) {
	var req struct {
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(msg.content, &req); err != nil || req.Filename == "" {
		log.Warnf("import HAR: invalid request")
		return
	}
	flows, err := addon.ImportHAR(req.Filename)
	if err != nil {
		log.Warnf("import HAR: %v", err)
		return
	}
	for _, f := range flows {
		web.storeFlow(f)
	}
	log.Infof("HAR imported: %s (%d flows)", req.Filename, len(flows))
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
