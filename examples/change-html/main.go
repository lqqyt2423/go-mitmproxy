package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

var titleRegexp = regexp.MustCompile("(<title>)(.*?)(</title>)")

type ChangeHtml struct {
	proxy.BaseAddon
}

func (c *ChangeHtml) StreamResponseModifier(f *proxy.Flow, in io.Reader) io.Reader {
	contentType := f.Response.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		return in
	}
	log.Println("开始修改了", f.Request.URL.String())
	body, err := ioutil.ReadAll(in)
	if err != nil {
		log.Error("ioutil.ReadAll err=", err)
		return nil
	}
	enc := f.Response.Header.Get("Content-Encoding")
	if enc != "" && enc != "identity" {
		body, err = proxy.Decode(enc, body)
		if err != nil {
			log.Error("proxy.Decode err=", err)
			return nil
		}
	}
	body = bytes.ReplaceAll(body, []byte("百度"), []byte("就要改你怎么地"))
	f.Response.Header.Del("Content-Encoding")
	f.Response.Header.Del("Transfer-Encoding")
	f.Response.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return bytes.NewReader(body)
}

func main() {
	opts := &proxy.Options{
		Addr:        ":62222",
		SslInsecure: false,
		CaRootPath:  "./",
	}

	p, err := proxy.NewProxy(opts)
	if err != nil {
		log.Fatal(err)
	}
	p.AddAddon(&ChangeHtml{})

	log.Fatal(p.Start())
}
