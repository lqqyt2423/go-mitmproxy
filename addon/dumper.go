package addon

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/lqqyt2423/go-mitmproxy/flow"
)

type Dumper struct {
	Base
	Out io.Writer
}

func NewDumperWithFile(file string) *Dumper {
	out, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	return &Dumper{Out: out}
}

func (d *Dumper) Requestheaders(f *flow.Flow) {
	log := log.WithField("in", "Dumper")

	go func() {
		<-f.Done()

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

		if f.Response != nil {
			fmt.Fprintf(buf, "%v %v %v\r\n", f.Request.Proto, f.Response.StatusCode, http.StatusText(f.Response.StatusCode))
			err = f.Response.Header.WriteSubset(buf, nil)
			if err != nil {
				log.Error(err)
			}
			buf.WriteString("\r\n")
		}

		_, err = d.Out.Write(buf.Bytes())
		if err != nil {
			log.Error(err)
		}
	}()
}
