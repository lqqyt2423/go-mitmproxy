package addon

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"unicode"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
	log "github.com/sirupsen/logrus"
)

type Dumper struct {
	proxy.BaseAddon
	out   io.Writer
	level int // 0: header 1: header + body
}

func NewDumper(out io.Writer, level int) *Dumper {
	if level != 0 && level != 1 {
		level = 0
	}
	return &Dumper{out: out, level: level}
}

func NewDumperWithFilename(filename string, level int) *Dumper {
	out, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	return NewDumper(out, level)
}

func (d *Dumper) Requestheaders(f *proxy.Flow) {
	go func() {
		<-f.Done()
		d.dump(f)
	}()
}

// call when <-f.Done()
func (d *Dumper) dump(f *proxy.Flow) {
	// 参考 httputil.DumpRequest

	buf := bytes.NewBuffer(make([]byte, 0))
	fmt.Fprintf(buf, "%s %s %s\r\n", f.Request.Method, f.Request.URL.RequestURI(), f.Request.Proto)
	fmt.Fprintf(buf, "Host: %s\r\n", f.Request.URL.Host)
	if len(f.Request.Raw().TransferEncoding) > 0 {
		fmt.Fprintf(buf, "Transfer-Encoding: %s\r\n", strings.Join(f.Request.Raw().TransferEncoding, ","))
	}
	if f.Request.Raw().Close {
		fmt.Fprintf(buf, "Connection: close\r\n")
	}

	err := f.Request.Header.WriteSubset(buf, nil)
	if err != nil {
		log.Error(err)
	}
	buf.WriteString("\r\n")

	if d.level == 1 && f.Request.Body != nil && len(f.Request.Body) > 0 && canPrint(f.Request.Body) {
		buf.Write(f.Request.Body)
		buf.WriteString("\r\n\r\n")
	}

	if f.Response != nil {
		fmt.Fprintf(buf, "%v %v %v\r\n", f.Request.Proto, f.Response.StatusCode, http.StatusText(f.Response.StatusCode))
		err = f.Response.Header.WriteSubset(buf, nil)
		if err != nil {
			log.Error(err)
		}
		buf.WriteString("\r\n")

		if d.level == 1 && f.Response.Body != nil && len(f.Response.Body) > 0 && f.Response.IsTextContentType() {
			body, err := f.Response.DecodedBody()
			if err == nil && body != nil && len(body) > 0 {
				buf.Write(body)
				buf.WriteString("\r\n\r\n")
			}
		}
	}

	buf.WriteString("\r\n\r\n")

	_, err = d.out.Write(buf.Bytes())
	if err != nil {
		log.Error(err)
	}
}

func canPrint(content []byte) bool {
	for _, c := range string(content) {
		if !unicode.IsPrint(c) && !unicode.IsSpace(c) {
			return false
		}
	}
	return true
}
