package proxy

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
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

// wireHeaderName emits curl-style lowercase for x-* headers.
// Go's http.Header / Transport always use CanonicalMIMEHeaderKey (X-Token),
// which breaks gateways that treat custom header names as case-sensitive.
func wireHeaderName(canonicalKey string) string {
	if strings.HasPrefix(strings.ToLower(canonicalKey), "x-") {
		return strings.ToLower(canonicalKey)
	}
	return canonicalKey
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

// roundTripRawHTTP1 writes an HTTP/1.1 request with case-preserving header names
// directly to conn (typically the upstream TLS connection), then reads the response.
// Call only when upstreamNegotiatedHTTP11 is true; HTTP/2 paths stay on http2.Transport.
func roundTripRawHTTP1(conn net.Conn, method, requestURI, host string, header http.Header, body []byte) (*http.Response, error) {
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

	keys := make([]string, 0, len(header))
	for key := range header {
		if hopByHopHeaders[key] || key == "Host" || key == "Content-Length" {
			continue
		}
		keys = append(keys, key)
	}
	// Deterministic order only (not product-specific).
	sort.Slice(keys, func(i, j int) bool {
		return strings.ToLower(keys[i]) < strings.ToLower(keys[j])
	})

	for _, key := range keys {
		name := wireHeaderName(key)
		for _, v := range header.Values(key) {
			writeHeaderLine(&buf, name, v)
		}
	}

	// Content-Length from buffered body is authoritative.
	if body != nil {
		writeHeaderLine(&buf, "Content-Length", fmt.Sprintf("%d", len(body)))
	} else if vals := header.Values("Content-Length"); len(vals) > 0 {
		writeHeaderLine(&buf, "Content-Length", vals[0])
	}

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
