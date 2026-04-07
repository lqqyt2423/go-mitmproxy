package addon

import (
	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

type NoCaching struct {
	proxy.BaseAddon
}

func NewNoCaching() *NoCaching {
	return &NoCaching{}
}

func (nc *NoCaching) Requestheaders(f *proxy.Flow) {
	f.Request.Header.Del("If-Modified-Since")
	f.Request.Header.Del("If-None-Match")
	f.Request.Header.Del("Cache-Control")
	f.Request.Header.Del("Pragma")
}

func (nc *NoCaching) Response(f *proxy.Flow) {
	if f.Response == nil {
		return
	}
	f.Response.Header.Del("ETag")
	f.Response.Header.Del("Last-Modified")
	f.Response.Header.Del("Expires")
	f.Response.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	f.Response.Header.Set("Pragma", "no-cache")
	f.Response.Header.Set("Expires", "0")
}
