package main

import "C"
import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"github.com/lqqyt2423/go-mitmproxy/web"
	uuid "github.com/satori/go.uuid"
)

func main() {}

//export GoStartProxy
func GoStartProxy() {
	opts := &proxy.Options{
		Addr: ":9080",
	}
	p, err := proxy.NewProxy(opts)
	if err != nil {
		log.Fatal(err)
	}
	globalProxy = p

	fmt.Println("go-mitmproxy: " + p.Version)

	p.AddAddon(&proxy.LogAddon{})
	p.AddAddon(&NodejsAddon{})
	p.AddAddon(web.NewWebAddon(":9081"))

	go func() {
		loopAckMessage()
	}()
	go func() {
		log.Fatal(p.Start())
	}()
}

//export GoCloseProxy
func GoCloseProxy() {
	close(nodejsFlowChan)
	close(ackMessageChan)
	if globalProxy != nil {
		globalProxy.Close()
	}
}

//export GoAcceptFlow
func GoAcceptFlow() *C.char {
	nf := <-nodejsFlowChan
	return nf
}

//export GoAckMessage
func GoAckMessage(rawStr *C.char) {
	payload := C.GoString(rawStr)
	am := &AckMessage{}
	err := json.Unmarshal([]byte(payload), am)
	if err != nil {
		log.Printf("GoAckMessage error: %v\n", err)
		return
	}
	ackMessageChan <- am
}

var globalProxy *proxy.Proxy
var nodejsFlowChan = make(chan *C.char)
var ackMessageChan = make(chan *AckMessage)
var waitAckMap = make(map[string]func(*AckMessage))
var waitAckMapLock = &sync.Mutex{}

type NodejsAddon struct {
	proxy.BaseAddon
}

func (a *NodejsAddon) Requestheaders(f *proxy.Flow) {
	toNodejs(f, FlowHookRequestheaders)
}

func (a *NodejsAddon) Request(f *proxy.Flow) {
	toNodejs(f, FlowHookRequest)
}

func (a *NodejsAddon) Responseheaders(f *proxy.Flow) {
	toNodejs(f, FlowHookResponseheaders)
}

func (a *NodejsAddon) Response(f *proxy.Flow) {
	f.Response.ReplaceToDecodedBody()
	toNodejs(f, FlowHookResponse)
}

type FlowHook string

const (
	FlowHookRequestheaders  FlowHook = "Requestheaders"
	FlowHookRequest         FlowHook = "Request"
	FlowHookResponseheaders FlowHook = "Responseheaders"
	FlowHookResponse        FlowHook = "Response"
)

type NodejsFlow struct {
	HookAt FlowHook `json:"hookAt"`
	Flow   *NFlow   `json:"flow"`
}

type NFlow struct {
	Id       uuid.UUID  `json:"id"`
	Request  *NRequest  `json:"request"`
	Response *NResponse `json:"response"`
}
type NRequest struct {
	Method string      `json:"method"`
	URL    string      `json:"url"`
	Proto  string      `json:"proto"`
	Header http.Header `json:"header"`
	Body   []byte      `json:"body"`
}
type NResponse struct {
	StatusCode int         `json:"statusCode"`
	Header     http.Header `json:"header"`
	Body       []byte      `json:"body"`
}

func toNodejs(f *proxy.Flow, at FlowHook) {
	nf, err := getNodejsFlow(f, at)
	if err != nil {
		log.Printf("getNodejsFlow error: %v\n", err)
		return
	}
	nodejsFlowChan <- nf

	// wait ack
	waitChan := make(chan *AckMessage)
	waitAckMapLock.Lock()
	waitAckMap[f.Id.String()+string(at)] = func(am *AckMessage) {
		waitChan <- am
	}
	waitAckMapLock.Unlock()
	am := <-waitChan

	// not change flow
	if am.Action == AckMessageActionNoChange {
		return
	}

	// change flow
	f.Request.Method = am.Flow.Request.Method
	if u, err := url.Parse(am.Flow.Request.URL); err == nil {
		f.Request.URL = u
	}
	f.Request.Proto = am.Flow.Request.Proto
	f.Request.Header = am.Flow.Request.Header
	f.Request.Body = am.Flow.Request.Body

	if am.Flow.Response != nil {
		if f.Response == nil {
			f.Response = &proxy.Response{}
		}
		f.Response.StatusCode = am.Flow.Response.StatusCode
		f.Response.Header = am.Flow.Response.Header
		f.Response.Body = am.Flow.Response.Body
	}
}

func getNodejsFlow(f *proxy.Flow, at FlowHook) (*C.char, error) {
	nf := &NodejsFlow{
		HookAt: at,
		Flow: &NFlow{
			Id: f.Id,
			Request: &NRequest{
				Method: f.Request.Method,
				URL:    f.Request.URL.String(),
				Proto:  f.Request.Proto,
				Header: f.Request.Header,
				Body:   f.Request.Body,
			},
		},
	}
	if f.Response != nil {
		nf.Flow.Response = &NResponse{
			StatusCode: f.Response.StatusCode,
			Header:     f.Response.Header,
			Body:       f.Response.Body,
		}
	}
	content, err := json.Marshal(nf)
	if err != nil {
		return nil, err
	}
	return C.CString(string(content)), nil
}

type AckMessageAction string

const (
	AckMessageActionNoChange AckMessageAction = "noChange"
	AckMessageActionChange   AckMessageAction = "change"
)

type AckMessage struct {
	Action AckMessageAction `json:"action"`
	HookAt FlowHook         `json:"hookAt"`
	Id     uuid.UUID        `json:"id"`
	Flow   *NFlow           `json:"flow"`
}

func loopAckMessage() {
	for am := range ackMessageChan {
		waitAckMapLock.Lock()
		key := am.Id.String() + string(am.HookAt)
		f, ok := waitAckMap[key]
		if ok {
			f(am)
			delete(waitAckMap, key)
		}
		waitAckMapLock.Unlock()
	}
}
