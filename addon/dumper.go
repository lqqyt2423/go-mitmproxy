package addon

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"unicode"

	"github.com/lqqyt2423/go-mitmproxy/flow"
)

type Dumper struct {
	Base
	level int // 0: header 1: header + body
	Out   io.Writer
}

func NewDumper(file string, level int) *Dumper {
	out, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	if level != 0 && level != 1 {
		level = 0
	}

	return &Dumper{Out: out, level: level}
}

// call when <-f.Done()
func (d *Dumper) dump(f *flow.Flow) {
	// 参考 httputil.DumpRequest

	log := log.WithField("in", "Dumper")

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

	if d.level == 1 && f.Request.Body != nil && len(f.Request.Body) > 0 && CanPrint(f.Request.Body) {
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

		if d.level == 1 && f.Response.Body != nil && len(f.Response.Body) > 0 {
			body, _ := f.Response.DecodedBody()
			if len(body) > 0 && CanPrint(body) {
				buf.Write(body)
				buf.WriteString("\r\n\r\n")
			}
		}
	}

	buf.WriteString("\r\n\r\n")

	_, err = d.Out.Write(buf.Bytes())
	if err != nil {
		log.Error(err)
	}
}

func (d *Dumper) Requestheaders(f *flow.Flow) {
	go func() {
		<-f.Done()
		d.dump(f)
	}()
}

func CanPrint(content []byte) bool {
	for _, c := range string(content) {
		if !unicode.IsPrint(c) && !unicode.IsSpace(c) {
			return false
		}
	}
	return true
}
