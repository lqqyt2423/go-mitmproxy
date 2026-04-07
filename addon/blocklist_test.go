package addon

import (
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

func TestBlockListFromFile(t *testing.T) {
	content := `{"Enable":true,"Items":[{"Enable":true,"Host":"ads.example.com","StatusCode":403,"Body":"Blocked"}]}`
	f, err := os.CreateTemp("", "blocklist-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	bl, err := NewBlockListFromFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !bl.Enable {
		t.Error("expected Enable to be true")
	}
	if len(bl.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(bl.Items))
	}
	if bl.Items[0].Host != "ads.example.com" {
		t.Errorf("expected host ads.example.com, got %s", bl.Items[0].Host)
	}
}

func TestBlockListAddRule(t *testing.T) {
	bl := NewBlockList()
	bl.AddRule("test.com", "/api/*", 503, "Service Down")
	if len(bl.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(bl.Items))
	}
	if bl.Items[0].StatusCode != 503 {
		t.Errorf("expected status 503, got %d", bl.Items[0].StatusCode)
	}
}

func newTestFlow(host, path, method string) *proxy.Flow {
	return &proxy.Flow{
		Request: &proxy.Request{
			Method: method,
			URL: &url.URL{
				Scheme: "https",
				Host:   host,
				Path:   path,
			},
			Header: make(http.Header),
		},
	}
}

func TestBlockListRequestheaders(t *testing.T) {
	bl := NewBlockList()
	bl.AddRule("ads.example.com", "", 403, "Blocked")

	t.Run("matching host is blocked", func(t *testing.T) {
		f := newTestFlow("ads.example.com", "/banner", "GET")
		bl.Requestheaders(f)
		if f.Response == nil {
			t.Fatal("expected response to be set (blocked)")
		}
		if f.Response.StatusCode != 403 {
			t.Errorf("expected status 403, got %d", f.Response.StatusCode)
		}
		if string(f.Response.Body) != "Blocked" {
			t.Errorf("expected body 'Blocked', got '%s'", string(f.Response.Body))
		}
	})

	t.Run("non-matching host passes through", func(t *testing.T) {
		f := newTestFlow("api.example.com", "/data", "GET")
		bl.Requestheaders(f)
		if f.Response != nil {
			t.Error("expected response to be nil (not blocked)")
		}
	})

	t.Run("disabled blocklist does nothing", func(t *testing.T) {
		bl2 := NewBlockList()
		bl2.Enable = false
		bl2.AddRule("*", "", 403, "Blocked")
		f := newTestFlow("anything.com", "/", "GET")
		bl2.Requestheaders(f)
		if f.Response != nil {
			t.Error("expected response to be nil when disabled")
		}
	})
}

func TestBlockListGlobPattern(t *testing.T) {
	bl := NewBlockList()
	bl.AddRule("*.ads.com", "/track*", 403, "Blocked")

	t.Run("wildcard host matches", func(t *testing.T) {
		f := newTestFlow("sub.ads.com", "/track/click", "GET")
		bl.Requestheaders(f)
		if f.Response == nil {
			t.Fatal("expected blocked")
		}
	})

	t.Run("wildcard host no match", func(t *testing.T) {
		f := newTestFlow("ads.com", "/track/click", "GET")
		bl.Requestheaders(f)
		if f.Response != nil {
			t.Error("expected not blocked (no wildcard prefix)")
		}
	})
}

func TestBlockListMethodFilter(t *testing.T) {
	bl := NewBlockList()
	bl.Items = append(bl.Items, &BlockRule{
		Enable:     true,
		Host:       "api.example.com",
		Method:     []string{"DELETE"},
		StatusCode: 405,
		Body:       "Method Not Allowed",
	})

	t.Run("matching method is blocked", func(t *testing.T) {
		f := newTestFlow("api.example.com", "/resource", "DELETE")
		bl.Requestheaders(f)
		if f.Response == nil {
			t.Fatal("expected blocked")
		}
		if f.Response.StatusCode != 405 {
			t.Errorf("expected 405, got %d", f.Response.StatusCode)
		}
	})

	t.Run("non-matching method passes", func(t *testing.T) {
		f := newTestFlow("api.example.com", "/resource", "GET")
		bl.Requestheaders(f)
		if f.Response != nil {
			t.Error("expected not blocked (wrong method)")
		}
	})
}

func TestBlockListCustomStatusCode(t *testing.T) {
	bl := NewBlockList()
	bl.AddRule("down.example.com", "", 503, "Service Unavailable")

	f := newTestFlow("down.example.com", "/", "GET")
	bl.Requestheaders(f)
	if f.Response == nil {
		t.Fatal("expected blocked")
	}
	if f.Response.StatusCode != 503 {
		t.Errorf("expected 503, got %d", f.Response.StatusCode)
	}
}
