package addon

import (
	"fmt"
	"path"
	"strings"

	"github.com/lqqyt2423/go-mitmproxy/internal/helper"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/match"
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
	Method   []string
	Path     string
}

func (mf *mapFrom) match(req *proxy.Request) bool {
	if mf.Protocol != "" && mf.Protocol != req.URL.Scheme {
		return false
	}
	if mf.Host != "" && mf.Host != req.URL.Host {
		return false
	}
	if len(mf.Method) > 0 && !lo.Contains(mf.Method, req.Method) {
		return false
	}
	if mf.Path != "" && !match.Match(req.URL.Path, mf.Path) {
		return false
	}
	return true
}

type mapRemoteTo struct {
	Protocol string
	Host     string
	Path     string
}

type mapRemoteItem struct {
	From   *mapFrom
	To     *mapRemoteTo
	Enable bool
}

func (item *mapRemoteItem) match(req *proxy.Request) bool {
	if !item.Enable {
		return false
	}
	return item.From.match(req)
}

func (item *mapRemoteItem) replace(req *proxy.Request) *proxy.Request {
	if item.To.Protocol != "" {
		req.URL.Scheme = item.To.Protocol
	}
	if item.To.Host != "" {
		req.URL.Host = item.To.Host
	}
	if item.To.Path != "" {
		if item.From.Path != "" && strings.HasSuffix(item.From.Path, "/*") {
			subPath := req.URL.Path[len(item.From.Path)-2:]
			req.URL.Path = path.Join("/", item.To.Path, subPath)
		} else {
			req.URL.Path = path.Join("/", item.To.Path)
		}
	}
	return req
}

type MapRemote struct {
	proxy.BaseAddon
	Items  []*mapRemoteItem
	Enable bool
}

func (mr *MapRemote) Requestheaders(f *proxy.Flow) {
	if !mr.Enable {
		return
	}
	for _, item := range mr.Items {
		if item.match(f.Request) {
			aurl := f.Request.URL.String()
			f.Request = item.replace(f.Request)
			f.UseSeparateClient = true
			burl := f.Request.URL.String()
			log.Infof("map remote %v to %v", aurl, burl)
			return
		}
	}
}

func (mr *MapRemote) validate() error {
	for i, item := range mr.Items {
		if item.From == nil {
			return fmt.Errorf("%v no item.From", i)
		}
		if item.From.Protocol != "" && item.From.Protocol != "http" && item.From.Protocol != "https" {
			return fmt.Errorf("%v invalid item.From.Protocol %v", i, item.From.Protocol)
		}
		if item.To == nil {
			return fmt.Errorf("%v no item.To", i)
		}
		if item.To.Protocol == "" && item.To.Host == "" && item.To.Path == "" {
			return fmt.Errorf("%v empty item.To", i)
		}
		if item.To.Protocol != "" && item.To.Protocol != "http" && item.To.Protocol != "https" {
			return fmt.Errorf("%v invalid item.To.Protocol %v", i, item.To.Protocol)
		}
	}
	return nil
}

func NewMapRemoteFromFile(filename string) (*MapRemote, error) {
	var mapRemote MapRemote
	if err := helper.NewStructFromFile(filename, &mapRemote); err != nil {
		return nil, err
	}
	if err := mapRemote.validate(); err != nil {
		return nil, err
	}
	return &mapRemote, nil
}
