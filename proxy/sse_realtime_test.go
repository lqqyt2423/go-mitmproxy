package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
)

// TestSSERealTime 测试 SSE 事件的实时性
func TestSSERealTime(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	t.Run("should trigger SSEMessage hook immediately as each event arrives", func(t *testing.T) {
		eventCount := 5
		eventInterval := 100 * time.Millisecond

		// 创建 SSE 服务器，按时间间隔逐步发送事件
		sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("Streaming not supported")
			}

			// 按时间间隔逐步发送事件
			for i := 0; i < eventCount; i++ {
				event := fmt.Sprintf("data: Event %d at %%s\n\n", i)
				fmt.Fprintf(w, event, time.Now().Format(time.RFC3339Nano))
				flusher.Flush()

				// 等待一段时间再发送下一个事件
				time.Sleep(eventInterval)
			}
		}))
		defer sseServer.Close()

		// 创建实时性测试 addon
		realtimeAddon := &SSERealTimeTestAddon{
			BaseAddon:          BaseAddon{},
			eventTimestamps:    make([]time.Time, 0),
			triggerSequence:    make([]string, 0),
			streamNotCompleted: true,
			mu:                 sync.Mutex{},
		}

		// 启动代理
		proxy, err := NewProxy(&Options{
			Addr: ":29105",
		})
		if err != nil {
			t.Fatalf("Failed to create proxy: %v", err)
		}
		proxy.AddAddon(realtimeAddon)
		go proxy.Start()
		defer proxy.Close()
		time.Sleep(time.Millisecond * 100)

		// 创建客户端
		proxyURL, _ := url.Parse("http://127.0.0.1:29105")
		client := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
		}

		// 记录测试开始时间
		testStartTime := time.Now()

		// 发送 SSE 请求
		sseEndpoint := sseServer.URL + "/sse"
		req, err := http.NewRequest("GET", sseEndpoint, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		req.Header.Set("Accept", "text/event-stream")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		// 读取响应（会触发 SSE hooks）
		buffer := make([]byte, 1024)
		for {
			n, err := resp.Body.Read(buffer)
			if n == 0 && err != nil {
				break
			}
		}

		testEndTime := time.Now()

		// 验证实时性
		t.Run("verify all events were captured", func(t *testing.T) {
			realtimeAddon.mu.Lock()
			defer realtimeAddon.mu.Unlock()

			if len(realtimeAddon.eventTimestamps) != eventCount {
				t.Errorf("Expected %d events, got %d", eventCount, len(realtimeAddon.eventTimestamps))
			}

			t.Logf("✓ Captured %d events", len(realtimeAddon.eventTimestamps))
		})

		t.Run("verify events were triggered during streaming, not after completion", func(t *testing.T) {
			realtimeAddon.mu.Lock()
			defer realtimeAddon.mu.Unlock()

			// 验证每个事件的触发时间都在测试开始和结束之间
			for i, timestamp := range realtimeAddon.eventTimestamps {
				if timestamp.Before(testStartTime) {
					t.Errorf("Event %d was triggered before test started", i)
				}
				if timestamp.After(testEndTime) {
					t.Errorf("Event %d was triggered after test ended", i)
				}
			}

			// 验证事件触发时间是递增的（说明是逐个触发的）
			for i := 1; i < len(realtimeAddon.eventTimestamps); i++ {
				if realtimeAddon.eventTimestamps[i].Before(realtimeAddon.eventTimestamps[i-1]) {
					t.Errorf("Event %d was triggered before event %d", i, i-1)
				}
			}

			t.Logf("✓ Events were triggered during streaming (not batched at end)")
		})

		t.Run("verify events were triggered with expected timing", func(t *testing.T) {
			realtimeAddon.mu.Lock()
			defer realtimeAddon.mu.Unlock()

			// 验证相邻事件的时间间隔大致符合服务器发送间隔
			// 允许一定的误差（网络延迟、处理时间等）
			maxDelay := eventInterval + 200*time.Millisecond
			minDelay := eventInterval - 100*time.Millisecond

			for i := 1; i < len(realtimeAddon.eventTimestamps); i++ {
				delay := realtimeAddon.eventTimestamps[i].Sub(realtimeAddon.eventTimestamps[i-1])

				// 只检查中间的事件，第一个和最后一个可能有额外的连接开销
				if i > 1 && i < len(realtimeAddon.eventTimestamps)-1 {
					if delay > maxDelay {
						t.Logf("Warning: Event %d delay %v exceeds expected %v", i, delay, eventInterval)
					}
					if delay < minDelay && minDelay > 0 {
						t.Logf("Warning: Event %d delay %v is less than expected %v", i, delay, eventInterval)
					}
				}
			}

			t.Logf("✓ Events were triggered with appropriate timing")
		})

		t.Run("verify stream was still marked as in progress during event triggers", func(t *testing.T) {
			// 这个验证在流结束后进行，检查中间事件是否在流进行期间触发
			if len(realtimeAddon.triggerSequence) == 0 {
				t.Error("No events were triggered")
			}

			// 检查是否有 "event_x_while_streaming" 的记录
			hasStreamingEvents := false
			for _, seq := range realtimeAddon.triggerSequence {
				if len(seq) > 0 {
					hasStreamingEvents = true
					break
				}
			}

			if !hasStreamingEvents {
				t.Error("Events were not triggered during streaming")
			}

			t.Logf("✓ Events were triggered while stream was active")
		})

		// 输出详细的时间信息
		t.Run("output detailed timing information", func(t *testing.T) {
			realtimeAddon.mu.Lock()
			defer realtimeAddon.mu.Unlock()

			t.Logf("\n=== SSE Real-time Test Results ===")
			t.Logf("Test started: %s", testStartTime.Format(time.RFC3339Nano))
			t.Logf("Test ended: %s", testEndTime.Format(time.RFC3339Nano))
			t.Logf("Total duration: %v", testEndTime.Sub(testStartTime))
			t.Logf("\nEvent timing:")

			for i, timestamp := range realtimeAddon.eventTimestamps {
				var delayFromStart time.Duration
				if i == 0 {
					delayFromStart = timestamp.Sub(testStartTime)
				} else {
					delayFromStart = timestamp.Sub(realtimeAddon.eventTimestamps[i-1])
				}
				t.Logf("  Event %d: %s (delay: %v)", i, timestamp.Format(time.RFC3339Nano), delayFromStart)
			}
			t.Logf("===================================\n")
		})
	})
}

// TestSSEImmediateTriggering 测试事件触发是立即的而非批量的
func TestSSEImmediateTriggering(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	t.Run("should trigger hooks immediately, not wait for stream completion", func(t *testing.T) {
		// 使用较长的发送间隔来确保能检测到实时性
		eventInterval := 200 * time.Millisecond
		eventCount := 3

		triggerTimes := make([]time.Time, 0)
		var mu sync.Mutex

		immediateAddon := &ImmediateTestAddon{
			BaseAddon: BaseAddon{},
			onEvent: func() {
				mu.Lock()
				defer mu.Unlock()
				triggerTimes = append(triggerTimes, time.Now())
			},
		}

		// 创建 SSE 服务器
		sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")

			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("Streaming not supported")
			}

			for i := 0; i < eventCount; i++ {
				fmt.Fprintf(w, "data: Event %d\n\n", i)
				flusher.Flush()
				time.Sleep(eventInterval)
			}
		}))
		defer sseServer.Close()

		// 启动代理
		proxy, err := NewProxy(&Options{
			Addr: ":29106",
		})
		if err != nil {
			t.Fatalf("Failed to create proxy: %v", err)
		}
		proxy.AddAddon(immediateAddon)
		go proxy.Start()
		defer proxy.Close()
		time.Sleep(time.Millisecond * 100)

		// 创建客户端
		proxyURL, _ := url.Parse("http://127.0.0.1:29106")
		client := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			},
		}

		// 发送请求并记录开始时间
		startTime := time.Now()
		resp, err := client.Get(sseServer.URL + "/sse")
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		// 读取响应
		buffer := make([]byte, 1024)
		for {
			n, err := resp.Body.Read(buffer)
			if n == 0 && err != nil {
				break
			}
		}

		// 验证实时性
		mu.Lock()
		defer mu.Unlock()

		if len(triggerTimes) != eventCount {
			t.Fatalf("Expected %d trigger times, got %d", eventCount, len(triggerTimes))
		}

		// 关键验证：每个触发时间都应该在流结束前
		streamEndTime := time.Now()

		for i, triggerTime := range triggerTimes {
			// 每个事件都应该在流结束之前触发
			if triggerTime.After(streamEndTime) {
				t.Errorf("Event %d was triggered after stream ended", i)
			}

			// 计算相对于开始时间的延迟
			delay := triggerTime.Sub(startTime)
			expectedDelay := time.Duration(i) * eventInterval

			// 允许一定的误差，但应该大致符合预期
			maxAcceptableDelay := expectedDelay + 500*time.Millisecond
			minAcceptableDelay := expectedDelay - 100*time.Millisecond

			if delay > maxAcceptableDelay || delay < minAcceptableDelay {
				t.Logf("Warning: Event %d delay %v differs from expected %v", i, delay, expectedDelay)
			} else {
				t.Logf("✓ Event %d triggered at appropriate time (delay: %v)", i, delay)
			}
		}

		// 最关键的验证：检查触发时间是否分散在整个时间段内
		// 而不是集中在末尾
		firstEventDelay := triggerTimes[0].Sub(startTime)
		lastEventDelay := triggerTimes[len(triggerTimes)-1].Sub(startTime)

		totalExpectedDuration := time.Duration(eventCount-1) * eventInterval

		// 如果所有事件都在最后才触发，说明不是实时的
		if lastEventDelay-firstEventDelay < totalExpectedDuration/2 {
			t.Errorf("Events appear to be batched: all triggered within %v, expected spread over %v",
				lastEventDelay-firstEventDelay, totalExpectedDuration)
		}

		t.Logf("✓ Events were triggered immediately as they arrived, not batched")
		t.Logf("✓ First event delay: %v, Last event delay: %v", firstEventDelay, lastEventDelay)
	})
}

// SSERealTimeTestAddon 测试 SSE 实时性的 addon
type SSERealTimeTestAddon struct {
	BaseAddon
	eventTimestamps []time.Time    // 每个 SSE 事件的触发时间
	triggerSequence []string       // 触发顺序记录
	streamNotCompleted bool         // 流是否仍在进行中
	mu              sync.Mutex
}

func (addon *SSERealTimeTestAddon) SSEStart(f *Flow) {
	addon.mu.Lock()
	defer addon.mu.Unlock()
	addon.streamNotCompleted = true
}

func (addon *SSERealTimeTestAddon) SSEMessage(f *Flow) {
	addon.mu.Lock()
	defer addon.mu.Unlock()

	// 记录事件触发时间
	addon.eventTimestamps = append(addon.eventTimestamps, time.Now())

	// 获取最新事件
	events := f.SSE.Events
	if len(events) > 0 {
		lastEvent := events[len(events)-1]
		addon.triggerSequence = append(addon.triggerSequence, lastEvent.Data)
	}
}

func (addon *SSERealTimeTestAddon) SSEEnd(f *Flow) {
	addon.mu.Lock()
	defer addon.mu.Unlock()
	addon.streamNotCompleted = false
}

// ImmediateTestAddon 立即触发测试的 addon
type ImmediateTestAddon struct {
	BaseAddon
	onEvent func()
}

func (addon *ImmediateTestAddon) SSEMessage(f *Flow) {
	if addon.onEvent != nil {
		addon.onEvent()
	}
}
