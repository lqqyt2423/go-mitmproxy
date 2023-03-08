package addon

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

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

type mapTo struct {
	Protocol string
	Host     string
	Path     string
}

type mapItem struct {
	From   *mapFrom
	To     *mapTo
	Enable bool
}

func (item *mapItem) match(req *proxy.Request) bool {
	if !item.Enable {
		return false
	}
	if item.From.Protocol != "" && item.From.Protocol != req.URL.Scheme {
		return false
	}
	if item.From.Host != "" && item.From.Host != req.URL.Host {
		return false
	}
	if len(item.From.Method) > 0 && !lo.Contains(item.From.Method, req.Method) {
		return false
	}
	if item.From.Path != "" && !match.Match(req.URL.Path, item.From.Path) {
		return false
	}
	return true
}

func (item *mapItem) replace(req *proxy.Request) *proxy.Request {
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
	Items  []*mapItem
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
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var mapRemote MapRemote
	if err := json.Unmarshal(data, &mapRemote); err != nil {
		return nil, err
	}
	if err := mapRemote.validate(); err != nil {
		return nil, err
	}
	return &mapRemote, nil
}
