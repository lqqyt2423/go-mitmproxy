package helper

import (
	"io"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"
)

// Wireshark 解析 https 设置
var tlsKeyLogWriter io.Writer
var tlsKeyLogOnce sync.Once

func GetTlsKeyLogWriter() io.Writer {
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
