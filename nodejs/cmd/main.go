package main

import "C"
import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"github.com/lqqyt2423/go-mitmproxy/web"
)

func main() {}

//export StartProxy
func StartProxy() {
	opts := &proxy.Options{
		Addr: ":9080",
	}
	p, err := proxy.NewProxy(opts)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("go-mitmproxy: " + p.Version)

	p.AddAddon(&proxy.LogAddon{})
	p.AddAddon(&NodejsAddon{})
	p.AddAddon(web.NewWebAddon(":9081"))

	go func() {
		log.Fatal(p.Start())
	}()
}

//export AcceptFlow
func AcceptFlow() *C.char {
	nf := <-nodejsFlowChan
	return nf
}

var nodejsFlowChan = make(chan *C.char)

type NodejsAddon struct {
	proxy.BaseAddon
}

func (a *NodejsAddon) Requestheaders(f *proxy.Flow) {
	nf, err := getNodejsFlow(f, FlowHookRequestheaders)
	if err != nil {
		log.Printf("getNodejsFlow error: %v\n", err)
		return
	}
	nodejsFlowChan <- nf
}

type FlowHook string

const (
	FlowHookRequestheaders  FlowHook = "Requestheaders"
	FlowHookRequest         FlowHook = "Request"
	FlowHookResponseheaders FlowHook = "Responseheaders"
	FlowHookResponse        FlowHook = "Response"
)

type NodejsFlow struct {
	HookAt FlowHook    `json:"hookAt"`
	Flow   *proxy.Flow `json:"flow"`
}

func getNodejsFlow(f *proxy.Flow, at FlowHook) (*C.char, error) {
	nf := &NodejsFlow{
		HookAt: at,
		Flow:   f,
	}
	content, err := json.Marshal(nf)
	if err != nil {
		return nil, err
	}
	return C.CString(string(content)), nil
}
