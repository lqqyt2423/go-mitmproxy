package proxy

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/cert"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func TestAttackerHTTP2RSTStreamAfterQueuedResponseDoesNotPanic(t *testing.T) {
	proxy, err := NewProxy(&Options{NewCaFunc: cert.NewSelfSignCAMemory})
	handleError(t, err)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	serverDone := make(chan struct{})
	body := bytes.Repeat([]byte("a"), 256*1024)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	})
	go func() {
		defer close(serverDone)
		proxy.attacker.h2Server.ServeConn(serverConn, &http2.ServeConnOpts{
			Handler:    handler,
			BaseConfig: proxy.attacker.server,
		})
	}()
	defer func() {
		serverConn.Close()
		select {
		case <-serverDone:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for HTTP/2 server connection to close")
		}
	}()

	fr := http2.NewFramer(clientConn, clientConn)
	writeHTTP2Request(t, clientConn, fr)
	readResponseHeaders(t, clientConn, fr, 1)

	dataHeader := readNextDataFrameHeader(t, clientConn, 1)

	pingData := [8]byte{'r', 's', 't', '-', 'p', 'i', 'n', 'g'}
	setWriteDeadline(t, clientConn)
	if err := fr.WriteRSTStream(1, http2.ErrCodeCancel); err != nil {
		t.Fatal(err)
	}
	if err := fr.WritePing(false, pingData); err != nil {
		t.Fatal(err)
	}

	setReadDeadline(t, clientConn)
	if _, err := io.CopyN(io.Discard, clientConn, int64(dataHeader.Length)); err != nil {
		t.Fatal(err)
	}

	readPingAck(t, clientConn, fr, pingData)
}

func writeHTTP2Request(t *testing.T, conn net.Conn, fr *http2.Framer) {
	t.Helper()

	setWriteDeadline(t, conn)
	if _, err := conn.Write([]byte(http2.ClientPreface)); err != nil {
		t.Fatal(err)
	}
	if err := fr.WriteSettings(); err != nil {
		t.Fatal(err)
	}

	var headerBlock bytes.Buffer
	enc := hpack.NewEncoder(&headerBlock)
	for _, hf := range []hpack.HeaderField{
		{Name: ":method", Value: "GET"},
		{Name: ":scheme", Value: "https"},
		{Name: ":authority", Value: "example.test"},
		{Name: ":path", Value: "/"},
	} {
		if err := enc.WriteField(hf); err != nil {
			t.Fatal(err)
		}
	}

	if err := fr.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      1,
		EndStream:     true,
		EndHeaders:    true,
		BlockFragment: headerBlock.Bytes(),
	}); err != nil {
		t.Fatal(err)
	}
}

func readResponseHeaders(t *testing.T, conn net.Conn, fr *http2.Framer, streamID uint32) {
	t.Helper()

	for {
		f := readFrame(t, conn, fr)
		switch f := f.(type) {
		case *http2.SettingsFrame:
			if !f.IsAck() {
				setWriteDeadline(t, conn)
				if err := fr.WriteSettingsAck(); err != nil {
					t.Fatal(err)
				}
			}
		case *http2.HeadersFrame:
			if f.Header().StreamID == streamID {
				return
			}
		}
	}
}

func readNextDataFrameHeader(t *testing.T, conn net.Conn, streamID uint32) http2.FrameHeader {
	t.Helper()

	for {
		setReadDeadline(t, conn)
		fh, err := http2.ReadFrameHeader(conn)
		if err != nil {
			t.Fatal(err)
		}
		if fh.Type == http2.FrameData && fh.StreamID == streamID {
			return fh
		}
		setReadDeadline(t, conn)
		if _, err := io.CopyN(io.Discard, conn, int64(fh.Length)); err != nil {
			t.Fatal(err)
		}
	}
}

func readPingAck(t *testing.T, conn net.Conn, fr *http2.Framer, want [8]byte) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		f := readFrame(t, conn, fr)
		ping, ok := f.(*http2.PingFrame)
		if ok && ping.IsAck() && ping.Data == want {
			return
		}
	}
	t.Fatal("timed out waiting for PING ack")
}

func readFrame(t *testing.T, conn net.Conn, fr *http2.Framer) http2.Frame {
	t.Helper()

	setReadDeadline(t, conn)
	f, err := fr.ReadFrame()
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func setReadDeadline(t *testing.T, conn net.Conn) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
}

func setWriteDeadline(t *testing.T, conn net.Conn) {
	t.Helper()
	if err := conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
}
