package helper

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GetProxyConn connect proxy
// ref: http/transport.go dialConn func
func GetProxyConn(ctx context.Context, proxyUrl *url.URL, address string, sslInsecure bool) (net.Conn, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", proxyUrl.Host)
	if err != nil {
		return nil, err
	}
	// 如果代理URL是HTTPS，则进行TLS握手
	if proxyUrl.Scheme == "https" {
		tlsConfig := &tls.Config{
			ServerName:         proxyUrl.Hostname(), // 设置TLS握手的服务器名称
			InsecureSkipVerify: sslInsecure,
			// 可以在这里添加其他TLS配置
		}
		// 包装原始连接为TLS连接
		tlsConn := tls.Client(conn, tlsConfig)
		// 执行TLS握手
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close() // 握手失败，关闭连接
			return nil, err
		}
		conn = tlsConn // 使用TLS连接替换原始连接
	}
	connectReq := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: address},
		Host:   address,
		Header: http.Header{},
	}
	if proxyUrl.User != nil {
		connectReq.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(proxyUrl.User.String())))
	}
	connectCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	didReadResponse := make(chan struct{}) // closed after CONNECT write+read is done or fails
	var resp *http.Response
	// Write the CONNECT request & read the response.
	go func() {
		defer close(didReadResponse)
		err = connectReq.Write(conn)
		if err != nil {
			return
		}
		// Okay to use and discard buffered reader here, because
		// TLS server will not speak until spoken to.
		br := bufio.NewReader(conn)
		resp, err = http.ReadResponse(br, connectReq)
	}()
	select {
	case <-connectCtx.Done():
		conn.Close()
		<-didReadResponse
		return nil, connectCtx.Err()
	case <-didReadResponse:
		// resp or err now set
	}
	if err != nil {
		conn.Close()
		return nil, err
	}
	if resp.StatusCode != 200 {
		_, text, ok := strings.Cut(resp.Status, " ")
		conn.Close()
		if !ok {
			return nil, errors.New("unknown status code")
		}
		return nil, errors.New(text)
	}
	return conn, nil
}
