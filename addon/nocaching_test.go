package addon

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

func TestNoCachingRequestheaders(t *testing.T) {
	nc := NewNoCaching()

	f := &proxy.Flow{
		Request: &proxy.Request{
			Method: "GET",
			URL:    &url.URL{Scheme: "https", Host: "example.com", Path: "/"},
			Header: http.Header{
				"If-Modified-Since": []string{"Thu, 01 Jan 2026 00:00:00 GMT"},
				"If-None-Match":     []string{`"abc123"`},
				"Cache-Control":     []string{"max-age=3600"},
				"Pragma":            []string{"no-cache"},
				"Accept":            []string{"text/html"},
			},
		},
	}

	nc.Requestheaders(f)

	if f.Request.Header.Get("If-Modified-Since") != "" {
		t.Error("If-Modified-Since should be removed")
	}
	if f.Request.Header.Get("If-None-Match") != "" {
		t.Error("If-None-Match should be removed")
	}
	if f.Request.Header.Get("Cache-Control") != "" {
		t.Error("Cache-Control should be removed")
	}
	if f.Request.Header.Get("Pragma") != "" {
		t.Error("Pragma should be removed")
	}
	if f.Request.Header.Get("Accept") != "text/html" {
		t.Error("Accept should be preserved")
	}
}

func TestNoCachingResponse(t *testing.T) {
	nc := NewNoCaching()

	f := &proxy.Flow{
		Request: &proxy.Request{
			Method: "GET",
			URL:    &url.URL{Scheme: "https", Host: "example.com", Path: "/"},
			Header: make(http.Header),
		},
		Response: &proxy.Response{
			StatusCode: 200,
			Header: http.Header{
				"Cache-Control": []string{"max-age=86400"},
				"Expires":       []string{"Thu, 01 Jan 2027 00:00:00 GMT"},
				"Etag":          []string{`"abc123"`},
				"Last-Modified": []string{"Thu, 01 Jan 2026 00:00:00 GMT"},
				"Content-Type":  []string{"text/html"},
			},
		},
	}

	nc.Response(f)

	if f.Response.Header.Get("Etag") != "" {
		t.Error("ETag should be removed")
	}
	if f.Response.Header.Get("Last-Modified") != "" {
		t.Error("Last-Modified should be removed")
	}
	if f.Response.Header.Get("Cache-Control") != "no-cache, no-store, must-revalidate" {
		t.Errorf("Cache-Control should be overridden, got: %s", f.Response.Header.Get("Cache-Control"))
	}
	if f.Response.Header.Get("Pragma") != "no-cache" {
		t.Errorf("Pragma should be set to no-cache, got: %s", f.Response.Header.Get("Pragma"))
	}
	if f.Response.Header.Get("Content-Type") != "text/html" {
		t.Error("Content-Type should be preserved")
	}
}

func TestNoCachingNilResponse(t *testing.T) {
	nc := NewNoCaching()

	f := &proxy.Flow{
		Request: &proxy.Request{
			Method: "GET",
			URL:    &url.URL{Scheme: "https", Host: "example.com", Path: "/"},
			Header: make(http.Header),
		},
	}

	// Should not panic with nil response
	nc.Response(f)
}
