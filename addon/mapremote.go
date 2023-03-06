package addon

import (
	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

// Path map rule:
//   1. mapFrom.Path /hello and mapTo.Path /world
//     /hello => /world
//   2. mapFrom.Path /hello/* and mapTo.Path /world
//     /hello => /world
//     /hello/abc => /world/abc

type mapFrom struct {
	Protocol string
	Host     string
	Path     string
}

type mapTo struct {
	Protocol string
	Host     string
	Path     string
}

type mapItem struct {
	From         *mapFrom
	To           *mapTo
	PerserveHost bool
	Enable       bool
}

func (item *mapItem) match(req *proxy.Request) bool {
	if !item.Enable {
		return false
	}
	if item.From.Protocol != "" && item.From.Protocol != req.URL.Scheme {
		return false
	}
	// todo
	return false
}

func (item *mapItem) replace(req *proxy.Request) *proxy.Request {
	// todo
	return req
}

type MapRemote struct {
	proxy.BaseAddon
	Items  []*mapItem
	Enable bool
}

func (mr *MapRemote) Requestheaders(f *proxy.Flow) {
	if !mr.Enable {
		return
	}
	for _, item := range mr.Items {
		if item.match(f.Request) {
			f.Request = item.replace(f.Request)
			return
		}
	}
}
