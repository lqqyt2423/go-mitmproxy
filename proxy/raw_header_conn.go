package proxy

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
)

// RawHeaderField is one header line exactly as received on the wire (name casing preserved).
type RawHeaderField struct {
	Name  string
	Value string
}

type http1HeaderSplicer struct {
	br       *bufio.Reader
	pending  []byte
	onCapture func([]RawHeaderField)
}

func (s *http1HeaderSplicer) Read(p []byte) (int, error) {
	if len(s.pending) > 0 {
		n := copy(p, s.pending)
		s.pending = s.pending[n:]
		return n, nil
	}

	peek, err := s.br.Peek(8)
	if err != nil && len(peek) == 0 {
		return 0, err
	}
	if looksLikeHTTP1Request(peek) {
		block, err := readHTTP1HeaderBlock(s.br)
		if err != nil {
			return 0, err
		}
		if s.onCapture != nil {
			s.onCapture(parseRawHeaderFields(block))
		}
		s.pending = block
		n := copy(p, s.pending)
		s.pending = s.pending[n:]
		return n, nil
	}
	return s.br.Read(p)
}

func looksLikeHTTP1Request(peek []byte) bool {
	methods := []string{
		"GET ", "POST ", "PUT ", "HEAD ", "DELETE ",
		"OPTIONS ", "PATCH ", "CONNECT ", "TRACE ",
	}
	for _, m := range methods {
		if len(peek) >= len(m) && string(peek[:len(m)]) == m {
			return true
		}
	}
	return false
}

func readHTTP1HeaderBlock(br *bufio.Reader) ([]byte, error) {
	var buf bytes.Buffer
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			buf.Write(line)
		}
		if err != nil {
			if buf.Len() > 0 {
				return buf.Bytes(), err
			}
			return nil, err
		}
		b := buf.Bytes()
		if bytes.HasSuffix(b, []byte("\r\n\r\n")) || bytes.HasSuffix(b, []byte("\n\n")) {
			return b, nil
		}
		if buf.Len() > 1<<20 {
			return nil, fmt.Errorf("http header block too large")
		}
	}
}

func parseRawHeaderFields(block []byte) []RawHeaderField {
	// Split on \n; tolerate \r\n
	lines := bytes.Split(block, []byte{'\n'})
	out := make([]RawHeaderField, 0, len(lines))
	for i, line := range lines {
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if i == 0 {
			continue // request line
		}
		if len(line) == 0 {
			break
		}
		idx := bytes.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		name := string(bytes.TrimSpace(line[:idx]))
		val := string(bytes.TrimLeft(line[idx+1:], " \t"))
		if name == "" {
			continue
		}
		out = append(out, RawHeaderField{Name: name, Value: val})
	}
	return out
}

// rawHeaderCaptureConn wraps a conn (e.g. TLS) and records original HTTP/1.1 header names.
type rawHeaderCaptureConn struct {
	net.Conn
	splicer http1HeaderSplicer
}

func wrapRawHeaderCapture(c net.Conn, connCtx *ConnContext) net.Conn {
	if c == nil {
		return nil
	}
	if _, ok := c.(*rawHeaderCaptureConn); ok {
		return c
	}
	rc := &rawHeaderCaptureConn{Conn: c}
	rc.splicer = http1HeaderSplicer{
		br: bufio.NewReader(c),
		onCapture: func(fields []RawHeaderField) {
			if connCtx != nil {
				connCtx.setRawRequestHeaders(fields)
			}
		},
	}
	return rc
}

func (c *rawHeaderCaptureConn) Read(p []byte) (int, error) {
	return c.splicer.Read(p)
}

func (c *ConnContext) setRawRequestHeaders(fields []RawHeaderField) {
	c.rawHeadersMu.Lock()
	c.rawRequestHeaders = fields
	c.rawHeadersMu.Unlock()
}

func (c *ConnContext) takeRawRequestHeaders() []RawHeaderField {
	c.rawHeadersMu.Lock()
	h := c.rawRequestHeaders
	c.rawRequestHeaders = nil
	c.rawHeadersMu.Unlock()
	return h
}