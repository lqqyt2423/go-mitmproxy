package addon

import (
	"net/url"
	"testing"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

func TestMapItemMatch(t *testing.T) {
	req := &proxy.Request{
		Method: "GET",
		URL: &url.URL{
			Scheme: "https",
			Host:   "example.com",
			Path:   "/path/to/resource",
		},
	}

	// test match

	item := &mapRemoteItem{
		From: &mapFrom{
			Protocol: "https",
			Host:     "example.com",
			Method:   []string{"GET", "POST"},
			Path:     "/path/to/resource",
		},
		To:     nil,
		Enable: true,
	}
	result := item.match(req)
	if !result {
		t.Errorf("Expected true, but got false")
	}

	// empty Protocol and empty Method match
	item.From = &mapFrom{
		Protocol: "",
		Host:     "example.com",
		Method:   []string{},
		Path:     "/path/to/resource",
	}
	result = item.match(req)
	if !result {
		t.Errorf("Expected true, but got false")
	}

	// empty Host match
	item.From = &mapFrom{
		Protocol: "",
		Host:     "",
		Method:   []string{},
		Path:     "/path/to/*",
	}
	result = item.match(req)
	if !result {
		t.Errorf("Expected true, but got false")
	}

	// all empty match
	item.From = &mapFrom{
		Protocol: "",
		Host:     "",
		Method:   []string{},
		Path:     "",
	}
	result = item.match(req)
	if !result {
		t.Errorf("Expected true, but got false")
	}

	// test not match

	// diff Protocol
	item.From = &mapFrom{
		Protocol: "http",
		Host:     "example.com",
		Method:   []string{},
		Path:     "/path/to/resource",
	}
	result = item.match(req)
	if result {
		t.Errorf("Expected true, but got false")
	}

	// diff Host
	item.From = &mapFrom{
		Protocol: "https",
		Host:     "hello.com",
		Method:   []string{},
		Path:     "/path/to/resource",
	}
	result = item.match(req)
	if result {
		t.Errorf("Expected true, but got false")
	}

	// diff Method
	item.From = &mapFrom{
		Protocol: "https",
		Host:     "example.com",
		Method:   []string{"PUT"},
		Path:     "/path/to/resource",
	}
	result = item.match(req)
	if result {
		t.Errorf("Expected true, but got false")
	}

	// diff Path
	item.From = &mapFrom{
		Protocol: "http",
		Host:     "example.com",
		Method:   []string{},
		Path:     "/hello/world",
	}
	result = item.match(req)
	if result {
		t.Errorf("Expected true, but got false")
	}
}

func TestMapItemReplace(t *testing.T) {
	rawreq := func() *proxy.Request {
		return &proxy.Request{
			Method: "GET",
			URL: &url.URL{
				Scheme: "https",
				Host:   "example.com",
				Path:   "/path/to/resource",
			},
		}
	}

	item := &mapRemoteItem{
		From: &mapFrom{
			Protocol: "https",
			Host:     "example.com",
			Method:   []string{"GET", "POST"},
			Path:     "/path/to/resource",
		},
		To: &mapRemoteTo{
			Protocol: "http",
			Host:     "hello.com",
			Path:     "",
		},
		Enable: true,
	}
	req := item.replace(rawreq())
	should := "http://hello.com/path/to/resource"
	if req.URL.String() != should {
		t.Errorf("Expected %v, but got %v", should, req.URL.String())
	}

	item = &mapRemoteItem{
		From: &mapFrom{
			Protocol: "https",
			Host:     "example.com",
			Method:   []string{"GET", "POST"},
			Path:     "/path/to/resource",
		},
		To: &mapRemoteTo{
			Protocol: "http",
			Host:     "hello.com",
			Path:     "/path/to/resource",
		},
		Enable: true,
	}
	req = item.replace(rawreq())
	should = "http://hello.com/path/to/resource"
	if req.URL.String() != should {
		t.Errorf("Expected %v, but got %v", should, req.URL.String())
	}

	item = &mapRemoteItem{
		From: &mapFrom{
			Protocol: "https",
			Host:     "example.com",
			Method:   []string{"GET", "POST"},
			Path:     "/path/to/resource",
		},
		To: &mapRemoteTo{
			Protocol: "http",
			Host:     "hello.com",
			Path:     "/path/to/world",
		},
		Enable: true,
	}
	req = item.replace(rawreq())
	should = "http://hello.com/path/to/world"
	if req.URL.String() != should {
		t.Errorf("Expected %v, but got %v", should, req.URL.String())
	}

	item = &mapRemoteItem{
		From: &mapFrom{
			Protocol: "https",
			Host:     "example.com",
			Method:   []string{"GET", "POST"},
			Path:     "/path/to/*",
		},
		To: &mapRemoteTo{
			Protocol: "http",
			Host:     "hello.com",
			Path:     "",
		},
		Enable: true,
	}
	req = item.replace(rawreq())
	should = "http://hello.com/path/to/resource"
	if req.URL.String() != should {
		t.Errorf("Expected %v, but got %v", should, req.URL.String())
	}

	item = &mapRemoteItem{
		From: &mapFrom{
			Protocol: "https",
			Host:     "example.com",
			Method:   []string{"GET", "POST"},
			Path:     "/path/to/*",
		},
		To: &mapRemoteTo{
			Protocol: "http",
			Host:     "hello.com",
			Path:     "/world",
		},
		Enable: true,
	}
	req = item.replace(rawreq())
	should = "http://hello.com/world/resource"
	if req.URL.String() != should {
		t.Errorf("Expected %v, but got %v", should, req.URL.String())
	}
}
