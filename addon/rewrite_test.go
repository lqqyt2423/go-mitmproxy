package addon

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

func TestRewriteFromFile(t *testing.T) {
	content := `{"Enable":true,"Items":[{"Enable":true,"Name":"test","From":{"Host":"*.example.com"},"Rules":[{"Type":"addHeader","Target":"request","Name":"X-Test","Value":"true"}]}]}`
	f, err := os.CreateTemp("", "rewrite-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	rw, err := NewRewriteFromFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !rw.Enable {
		t.Error("expected Enable to be true")
	}
	if len(rw.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(rw.Items))
	}
}

func TestRewriteMatch(t *testing.T) {
	item := &RewriteItem{
		Enable: true,
		From: RewriteMatch{
			Host: "*.example.com",
			Path: "/api/*",
		},
		Rules: []RewriteAction{},
	}

	tests := []struct {
		host  string
		path  string
		match bool
	}{
		{"sub.example.com", "/api/users", true},
		{"api.example.com", "/api/v1/data", true},
		{"example.com", "/api/data", false},  // no wildcard prefix
		{"sub.example.com", "/web/page", false}, // path doesn't match
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s%s", tt.host, tt.path), func(t *testing.T) {
			req := &proxy.Request{
				Method: "GET",
				URL:    &url.URL{Scheme: "https", Host: tt.host, Path: tt.path},
			}
			got := matchRewriteItem(item, req)
			if got != tt.match {
				t.Errorf("matchRewriteItem(%s, %s) = %v, want %v", tt.host, tt.path, got, tt.match)
			}
		})
	}
}

func TestRewriteMatchProtocol(t *testing.T) {
	item := &RewriteItem{
		Enable: true,
		From:   RewriteMatch{Protocol: "https", Host: "*"},
		Rules:  []RewriteAction{},
	}

	req := &proxy.Request{
		Method: "GET",
		URL:    &url.URL{Scheme: "http", Host: "example.com"},
	}
	if matchRewriteItem(item, req) {
		t.Error("should not match http when protocol filter is https")
	}

	req.URL.Scheme = "https"
	if !matchRewriteItem(item, req) {
		t.Error("should match https")
	}
}

func TestRewriteMatchMethod(t *testing.T) {
	item := &RewriteItem{
		Enable: true,
		From:   RewriteMatch{Host: "*", Method: []string{"POST", "PUT"}},
		Rules:  []RewriteAction{},
	}

	req := &proxy.Request{Method: "GET", URL: &url.URL{Host: "example.com"}}
	if matchRewriteItem(item, req) {
		t.Error("GET should not match POST/PUT filter")
	}

	req.Method = "POST"
	if !matchRewriteItem(item, req) {
		t.Error("POST should match")
	}
}

func TestRewriteAddHeader(t *testing.T) {
	rw := NewRewrite()
	rw.Items = append(rw.Items, &RewriteItem{
		Enable: true,
		From:   RewriteMatch{Host: "api.example.com"},
		Rules: []RewriteAction{
			{Type: "addHeader", Target: "request", Name: "X-Debug", Value: "true"},
		},
	})

	f := newTestFlow("api.example.com", "/v1/data", "GET")
	rw.Requestheaders(f)

	if f.Request.Header.Get("X-Debug") != "true" {
		t.Errorf("expected X-Debug: true, got: %s", f.Request.Header.Get("X-Debug"))
	}
}

func TestRewriteModifyHeader(t *testing.T) {
	rw := NewRewrite()
	rw.Items = append(rw.Items, &RewriteItem{
		Enable: true,
		From:   RewriteMatch{Host: "*"},
		Rules: []RewriteAction{
			{Type: "modifyHeader", Target: "request", Name: "User-Agent", Value: "OldAgent", Replace: "NewAgent", MatchMode: "text"},
		},
	})

	f := newTestFlow("example.com", "/", "GET")
	f.Request.Header.Set("User-Agent", "OldAgent/1.0")
	rw.Requestheaders(f)

	if f.Request.Header.Get("User-Agent") != "NewAgent/1.0" {
		t.Errorf("expected User-Agent modified, got: %s", f.Request.Header.Get("User-Agent"))
	}
}

func TestRewriteRemoveHeader(t *testing.T) {
	rw := NewRewrite()
	rw.Items = append(rw.Items, &RewriteItem{
		Enable: true,
		From:   RewriteMatch{Host: "*"},
		Rules: []RewriteAction{
			{Type: "removeHeader", Target: "request", Name: "X-Remove-Me"},
		},
	})

	f := newTestFlow("example.com", "/", "GET")
	f.Request.Header.Set("X-Remove-Me", "value")
	rw.Requestheaders(f)

	if f.Request.Header.Get("X-Remove-Me") != "" {
		t.Error("header should be removed")
	}
}

func TestRewriteHostPath(t *testing.T) {
	rw := NewRewrite()
	rw.Items = append(rw.Items, &RewriteItem{
		Enable: true,
		From:   RewriteMatch{Host: "old.example.com"},
		Rules: []RewriteAction{
			{Type: "host", Target: "request", Value: "old.example.com", Replace: "new.example.com", MatchMode: "text"},
			{Type: "path", Target: "request", Value: "/v1/", Replace: "/v2/", MatchMode: "text"},
		},
	})

	f := newTestFlow("old.example.com", "/v1/users", "GET")
	rw.Requestheaders(f)

	if f.Request.URL.Host != "new.example.com" {
		t.Errorf("expected host new.example.com, got: %s", f.Request.URL.Host)
	}
	if f.Request.URL.Path != "/v2/users" {
		t.Errorf("expected path /v2/users, got: %s", f.Request.URL.Path)
	}
}

func TestRewriteResponseBody(t *testing.T) {
	rw := NewRewrite()
	rw.Items = append(rw.Items, &RewriteItem{
		Enable: true,
		From:   RewriteMatch{Host: "*"},
		Rules: []RewriteAction{
			{Type: "body", Target: "response", Value: "oldText", Replace: "newText", MatchMode: "text"},
		},
	})

	f := newTestFlow("example.com", "/", "GET")
	f.Response = &proxy.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Length": []string{"20"}},
		Body:       []byte("prefix oldText suffix"),
	}

	rw.Response(f)

	expected := "prefix newText suffix"
	if string(f.Response.Body) != expected {
		t.Errorf("expected body '%s', got '%s'", expected, string(f.Response.Body))
	}
	// Content-Length should be recalculated
	if f.Response.Header.Get("Content-Length") != fmt.Sprintf("%d", len(expected)) {
		t.Errorf("Content-Length should be recalculated, got: %s", f.Response.Header.Get("Content-Length"))
	}
}

func TestRewriteResponseBodyRegex(t *testing.T) {
	rw := NewRewrite()
	rw.Items = append(rw.Items, &RewriteItem{
		Enable: true,
		From:   RewriteMatch{Host: "*"},
		Rules: []RewriteAction{
			{Type: "body", Target: "response", Value: `\d{4}-\d{2}-\d{2}`, Replace: "REDACTED", MatchMode: "regex"},
		},
	})

	f := newTestFlow("example.com", "/", "GET")
	f.Response = &proxy.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       []byte("date: 2026-04-06 and 2025-01-01"),
	}

	rw.Response(f)

	expected := "date: REDACTED and REDACTED"
	if string(f.Response.Body) != expected {
		t.Errorf("expected body '%s', got '%s'", expected, string(f.Response.Body))
	}
}

func TestRewriteResponseStatus(t *testing.T) {
	rw := NewRewrite()
	rw.Items = append(rw.Items, &RewriteItem{
		Enable: true,
		From:   RewriteMatch{Host: "*"},
		Rules: []RewriteAction{
			{Type: "status", Target: "response", Value: "200"},
		},
	})

	f := newTestFlow("example.com", "/", "GET")
	f.Response = &proxy.Response{
		StatusCode: 500,
		Header:     make(http.Header),
	}

	rw.Response(f)

	if f.Response.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", f.Response.StatusCode)
	}
}

func TestRewriteMultipleRules(t *testing.T) {
	rw := NewRewrite()
	rw.Items = append(rw.Items, &RewriteItem{
		Enable: true,
		From:   RewriteMatch{Host: "*"},
		Rules: []RewriteAction{
			{Type: "addHeader", Target: "request", Name: "X-First", Value: "1"},
		},
	})
	rw.Items = append(rw.Items, &RewriteItem{
		Enable: true,
		From:   RewriteMatch{Host: "*"},
		Rules: []RewriteAction{
			{Type: "addHeader", Target: "request", Name: "X-Second", Value: "2"},
		},
	})

	f := newTestFlow("example.com", "/", "GET")
	rw.Requestheaders(f)

	if f.Request.Header.Get("X-First") != "1" {
		t.Error("first rule should apply")
	}
	if f.Request.Header.Get("X-Second") != "2" {
		t.Error("second rule should also apply")
	}
}

func TestRewriteDisabledRule(t *testing.T) {
	rw := NewRewrite()
	rw.Items = append(rw.Items, &RewriteItem{
		Enable: false, // disabled
		From:   RewriteMatch{Host: "*"},
		Rules: []RewriteAction{
			{Type: "addHeader", Target: "request", Name: "X-Should-Not-Exist", Value: "yes"},
		},
	})

	f := newTestFlow("example.com", "/", "GET")
	rw.Requestheaders(f)

	if f.Request.Header.Get("X-Should-Not-Exist") != "" {
		t.Error("disabled rule should not apply")
	}
}

func TestRewriteQueryParam(t *testing.T) {
	rw := NewRewrite()
	rw.Items = append(rw.Items, &RewriteItem{
		Enable: true,
		From:   RewriteMatch{Host: "*"},
		Rules: []RewriteAction{
			{Type: "addQueryParam", Target: "request", Name: "debug", Value: "true"},
			{Type: "removeQueryParam", Target: "request", Name: "token"},
		},
	})

	f := newTestFlow("example.com", "/api", "GET")
	f.Request.URL.RawQuery = "token=secret&page=1"
	rw.Requestheaders(f)

	q := f.Request.URL.Query()
	if q.Get("debug") != "true" {
		t.Error("debug param should be added")
	}
	if q.Has("token") {
		t.Error("token param should be removed")
	}
	if q.Get("page") != "1" {
		t.Error("page param should be preserved")
	}
}

func TestRewriteResponseAddHeader(t *testing.T) {
	rw := NewRewrite()
	rw.Items = append(rw.Items, &RewriteItem{
		Enable: true,
		From:   RewriteMatch{Host: "*"},
		Rules: []RewriteAction{
			{Type: "addHeader", Target: "response", Name: "X-Proxy", Value: "go-mitmproxy"},
		},
	})

	f := newTestFlow("example.com", "/", "GET")
	f.Response = &proxy.Response{
		StatusCode: 200,
		Header:     make(http.Header),
	}
	rw.Response(f)

	if f.Response.Header.Get("X-Proxy") != "go-mitmproxy" {
		t.Error("response header should be added")
	}
}
