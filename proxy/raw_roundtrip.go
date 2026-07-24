package proxy

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// hop-by-hop headers must not be forwarded (RFC 7230).
var hopByHopHeaders = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailers":            true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
	"Proxy-Connection":    true,
}

func writeHeaderLine(buf *bytes.Buffer, key, value string) {
	buf.WriteString(key)
	buf.WriteString(": ")
	buf.WriteString(value)
	buf.WriteString("\r\n")
}

func hostWithoutDefaultPort(hostport string) string {
	if h, p, err := net.SplitHostPort(hostport); err == nil {
		if p == "443" || p == "80" {
			return h
		}
		return hostport
	}
	return hostport
}

// upstreamNegotiatedHTTP11 reports whether the upstream TLS conn speaks HTTP/1.1.
// Raw forwarding must not run on h2 connections.
func upstreamNegotiatedHTTP11(sc *ServerConn) bool {
	if sc == nil || sc.tlsState == nil {
		return true
	}
	p := sc.tlsState.NegotiatedProtocol
	return p == "" || p == "http/1.1"
}

func isHopByHop(name string) bool {
	return hopByHopHeaders[http.CanonicalHeaderKey(name)]
}

// roundTripRawHTTP1 writes an HTTP/1.1 request with original header names when available.
func roundTripRawHTTP1(conn net.Conn, method, requestURI, host string, header http.Header, raw []RawHeaderField, body []byte) (*http.Response, error) {
	if conn == nil {
		return nil, fmt.Errorf("raw roundtrip: nil conn")
	}
	if requestURI == "" {
		requestURI = "/"
	}
	host = hostWithoutDefaultPort(host)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s %s HTTP/1.1\r\n", method, requestURI)
	writeHeaderLine(&buf, "Host", host)

	wroteCL := false
	if len(raw) > 0 {
		for _, h := range raw {
			if h.Name == "" || strings.EqualFold(h.Name, "Host") || isHopByHop(h.Name) {
				continue
			}
			if strings.EqualFold(h.Name, "Content-Length") {
				continue
			}
			writeHeaderLine(&buf, h.Name, h.Value)
		}
	} else {
		for key, vals := range header {
			if key == "Host" || isHopByHop(key) || key == "Content-Length" {
				continue
			}
			for _, v := range vals {
				writeHeaderLine(&buf, key, v)
			}
		}
	}

	if body != nil {
		writeHeaderLine(&buf, "Content-Length", fmt.Sprintf("%d", len(body)))
		wroteCL = true
	} else if vals := header.Values("Content-Length"); len(vals) > 0 {
		writeHeaderLine(&buf, "Content-Length", vals[0])
		wroteCL = true
	}
	_ = wroteCL

	writeHeaderLine(&buf, "Connection", "close")
	buf.WriteString("\r\n")

	if _, err := conn.Write(buf.Bytes()); err != nil {
		return nil, err
	}
	if len(body) > 0 {
		if _, err := conn.Write(body); err != nil {
			return nil, err
		}
	}

	return http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: method})
}

// drainAndClose helps callers discard unread response bodies.
func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}
