package flowmapper

import "testing"

func TestParser(t *testing.T) {
	content := `
GET /index.html
Host: www.baidu.com
Accept: */*

hello world

HTTP/1.1 200

ok
`
	p, err := NewParserFromString(content)
	if err != nil {
		t.Fatal(err)
	}
	f, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}

	if f.Request.Method != "GET" {
		t.Fatal("request method error")
	}
	if f.Request.URL.String() != "http://www.baidu.com/index.html" {
		t.Fatal("request url error")
	}
	if f.Response.StatusCode != 200 {
		t.Fatal("response status code error")
	}
	if string(f.Response.Body) != "ok" {
		t.Fatal("response body error")
	}
	if f.Response.Header.Get("Content-Length") != "2" {
		t.Fatal("response header content-length error")
	}
}
