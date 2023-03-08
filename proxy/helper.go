package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

var normalErrMsgs []string = []string{
	"read: connection reset by peer",
	"write: broken pipe",
	"i/o timeout",
	"net/http: TLS handshake timeout",
	"io: read/write on closed pipe",
	"connect: connection refused",
	"connect: connection reset by peer",
	"use of closed network connection",
}

// 仅打印预料之外的错误信息
func logErr(log *log.Entry, err error) (loged bool) {
	msg := err.Error()

	for _, str := range normalErrMsgs {
		if strings.Contains(msg, str) {
			log.Debug(err)
			return
		}
	}

	log.Error(err)
	loged = true
	return
}

// 转发流量
func transfer(log *log.Entry, server, client io.ReadWriteCloser) {
	done := make(chan struct{})
	defer close(done)

	errChan := make(chan error)
	go func() {
		_, err := io.Copy(server, client)
		log.Debugln("client copy end", err)
		client.Close()
		select {
		case <-done:
			return
		case errChan <- err:
			return
		}
	}()
	go func() {
		_, err := io.Copy(client, server)
		log.Debugln("server copy end", err)
		server.Close()

		if clientConn, ok := client.(*wrapClientConn); ok {
			err := clientConn.Conn.(*net.TCPConn).CloseRead()
			log.Debugln("clientConn.Conn.(*net.TCPConn).CloseRead()", err)
		}

		select {
		case <-done:
			return
		case errChan <- err:
			return
		}
	}()

	for i := 0; i < 2; i++ {
		if err := <-errChan; err != nil {
			logErr(log, err)
			return // 如果有错误，直接返回
		}
	}
}

// 尝试将 Reader 读取至 buffer 中
// 如果未达到 limit，则成功读取进入 buffer
// 否则 buffer 返回 nil，且返回新 Reader，状态为未读取前
func readerToBuffer(r io.Reader, limit int64) ([]byte, io.Reader, error) {
	buf := bytes.NewBuffer(make([]byte, 0))
	lr := io.LimitReader(r, limit)

	_, err := io.Copy(buf, lr)
	if err != nil {
		return nil, nil, err
	}

	// 达到上限
	if int64(buf.Len()) == limit {
		// 返回新的 Reader
		return nil, io.MultiReader(bytes.NewBuffer(buf.Bytes()), r), nil
	}

	// 返回 buffer
	return buf.Bytes(), nil, nil
}

// Wireshark 解析 https 设置
var tlsKeyLogWriter io.Writer
var tlsKeyLogOnce sync.Once

func getTlsKeyLogWriter() io.Writer {
	tlsKeyLogOnce.Do(func() {
		logfile := os.Getenv("SSLKEYLOGFILE")
		if logfile == "" {
			return
		}

		writer, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Debugf("getTlsKeyLogWriter OpenFile error: %v", err)
			return
		}

		tlsKeyLogWriter = writer
	})
	return tlsKeyLogWriter
}

func clientProxy(upstream string) func(*http.Request) (*url.URL, error) {
	var useProxy func(*http.Request) (*url.URL, error)
	if len(upstream) > 0 {
		upstreamUrl, _ := url.Parse(upstream)
		useProxy = http.ProxyURL(upstreamUrl)
	} else {
		useProxy = http.ProxyFromEnvironment
	}
	return useProxy
}

func NewStructFromFile[T any](filename string) (*T, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var item T
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, err
	}
	return &item, nil
}
