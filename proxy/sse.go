package proxy

import (
	"bufio"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// sseReader wraps an io.Reader to parse SSE events and trigger hooks
type sseReader struct {
	flow     *Flow
	proxy    *Proxy
	reader   *bufio.Reader
	buffer   []byte      // 当前事件累积的数据
	started  bool
	ended    bool
	mu       sync.Mutex
}

// newSSEReader creates a new SSE reader wrapper
func newSSEReader(f *Flow, r io.Reader) io.Reader {
	return &sseReader{
		flow:   f,
		proxy:  f.ConnContext.proxy,
		reader: bufio.NewReader(r),
		buffer: make([]byte, 0, 1024),
	}
}

// Read implements io.Reader, parsing SSE events as data is read
func (sr *sseReader) Read(p []byte) (n int, err error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if sr.ended {
		return 0, io.EOF
	}

	// 从底层 reader 读取数据
	n, err = sr.reader.Read(p)
	if n > 0 {
		// 将新数据添加到 buffer
		sr.buffer = append(sr.buffer, p[:n]...)

		// 解析 buffer 中的所有完整事件
		sr.parseEvents()
	}

	if err != nil {
		// 流结束，触发 SSEEnd
		if sr.started {
			sr.flushEvent() // 处理最后一个可能不完整的事件
			sr.triggerEnd()
		}
		return n, err
	}

	if !sr.started {
		sr.started = true
	}

	return n, nil
}

// parseEvents parses SSE events from the buffer
func (sr *sseReader) parseEvents() {
	// 在 buffer 中查找事件分隔符 \n\n
	for {
		// 查找下一个事件结束位置
		eventEnd := sr.findEventEnd()
		if eventEnd == -1 {
			// 没有找到完整事件，等待更多数据
			break
		}

		// 提取一个完整的事件
		eventData := string(sr.buffer[:eventEnd])
		if len(eventData) > 0 {
			// 解析并触发事件
			sr.parseAndFlushEvent(eventData)
		}

		// 移除已处理的数据（包括 \n\n）
		sr.buffer = sr.buffer[eventEnd+2:]
	}
}

// findEventEnd 在 buffer 中查找 \n\n 的位置
func (sr *sseReader) findEventEnd() int {
	for i := 0; i < len(sr.buffer)-1; i++ {
		if sr.buffer[i] == '\n' && sr.buffer[i+1] == '\n' {
			return i
		}
	}
	return -1
}

// parseAndFlushEvent 解析并触发一个事件
func (sr *sseReader) parseAndFlushEvent(eventData string) {
	event := sr.parseEvent(eventData)

	// 只处理有 data 字段的事件
	if event.Data != "" {
		// 保存到 Flow 中
		sr.flow.SSE.addEvent(event)

		// 触发 SSEMessage hook（通过 f.SSE.Events 访问最新事件）
		for _, addon := range sr.proxy.Addons {
			addon.SSEMessage(sr.flow)
		}
	}
}

// flushEvent 解析并触发当前缓冲区中的事件
func (sr *sseReader) flushEvent() {
	if len(sr.buffer) == 0 {
		return
	}

	// 解析事件
	event := sr.parseEvent(string(sr.buffer))

	// 只处理有 data 字段的事件
	if event.Data != "" {
		// 保存到 Flow 中
		sr.flow.SSE.addEvent(event)

		// 触发 SSEMessage hook（通过 f.SSE.Events 访问最新事件）
		for _, addon := range sr.proxy.Addons {
			addon.SSEMessage(sr.flow)
		}
	}

	// 清空 buffer
	sr.buffer = sr.buffer[:0]
}

// parseEvent 解析 SSE 事件文本
func (sr *sseReader) parseEvent(text string) *SSEEvent {
	event := &SSEEvent{
		Event: "message",
		Data:  "",
		Raw:   text,
		Time:  time.Now(),
	}

	// 按行解析
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 跳过注释行
		if strings.HasPrefix(line, ":") {
			continue
		}

		// 解析 field: value 格式
		if idx := strings.Index(line, ":"); idx != -1 {
			field := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])

			switch field {
			case "data":
				// 多个 data 行用换行符连接
				if event.Data != "" {
					event.Data += "\n"
				}
				event.Data += value
			case "event":
				event.Event = value
			case "id":
				event.ID = value
			case "retry":
				if retry, err := strconv.Atoi(value); err == nil {
					event.Retry = retry
				}
			}
		}
	}

	return event
}

// triggerEnd 触发 SSE 结束 hook
func (sr *sseReader) triggerEnd() {
	if sr.ended {
		return
	}

	sr.ended = true

	for _, addon := range sr.proxy.Addons {
		addon.SSEEnd(sr.flow)
	}

	log.Debugf("SSE stream ended for %s", sr.flow.Request.URL.String())
}
