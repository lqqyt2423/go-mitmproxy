package flow

import (
	"io"
	"os"
	"sync"
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
			log.WithField("in", "GetTlsKeyLogWriter").Debug(err)
			return
		}

		tlsKeyLogWriter = writer
	})
	return tlsKeyLogWriter
}
