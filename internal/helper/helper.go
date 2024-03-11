package helper

import (
	"encoding/json"
	"net"
	"net/url"
	"os"
)

func NewStructFromFile(filename string, v interface{}) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return err
	}
	return nil
}

var portMap = map[string]string{
	"http":   "80",
	"https":  "443",
	"socks5": "1080",
}

// CanonicalAddr returns url.Host but always with a ":port" suffix.
func CanonicalAddr(url *url.URL) string {
	port := url.Port()
	if port == "" {
		port = portMap[url.Scheme]
	}
	return net.JoinHostPort(url.Hostname(), port)
}

// https://github.com/mitmproxy/mitmproxy/blob/main/mitmproxy/net/tls.py is_tls_record_magic
func IsTls(buf []byte) bool {
	if buf[0] == 0x16 && buf[1] == 0x03 && buf[2] <= 0x03 {
		return true
	} else {
		return false
	}
}
