package addon

import (
	"io"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/internal/helper"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"github.com/tidwall/match"
)

type ThrottleProfile struct {
	Name              string  `json:"Name"`
	DownloadKbps      int64   `json:"DownloadKbps"`
	UploadKbps        int64   `json:"UploadKbps"`
	LatencyMs         int     `json:"LatencyMs"`
	PacketLossPercent float64 `json:"PacketLossPercent"`
}

// Built-in presets
var (
	Preset3G = ThrottleProfile{
		Name: "3G", DownloadKbps: 750, UploadKbps: 250, LatencyMs: 200,
	}
	Preset4G = ThrottleProfile{
		Name: "4G/LTE", DownloadKbps: 12000, UploadKbps: 5000, LatencyMs: 50,
	}
	PresetWiFiLossy = ThrottleProfile{
		Name: "WiFi (lossy)", DownloadKbps: 30000, UploadKbps: 15000,
		LatencyMs: 5, PacketLossPercent: 1,
	}
	PresetFullLoss = ThrottleProfile{
		Name: "100% Loss", PacketLossPercent: 100,
	}
)

type ThrottleConfig struct {
	proxy.BaseAddon
	Enable  bool            `json:"Enable"`
	Hosts   []string        `json:"Hosts"`
	Profile ThrottleProfile `json:"Profile"`
}

func NewThrottleFromFile(filename string) (*ThrottleConfig, error) {
	config := new(ThrottleConfig)
	if err := helper.NewStructFromFile(filename, config); err != nil {
		return nil, err
	}
	return config, nil
}

func NewThrottle(profile ThrottleProfile) *ThrottleConfig {
	return &ThrottleConfig{
		Enable:  true,
		Profile: profile,
	}
}

func (t *ThrottleConfig) shouldThrottle(host string) bool {
	if !t.Enable {
		return false
	}
	if len(t.Hosts) == 0 {
		return true
	}
	for _, pattern := range t.Hosts {
		if match.Match(host, pattern) {
			return true
		}
	}
	return false
}

func (t *ThrottleConfig) Requestheaders(f *proxy.Flow) {
	if !t.shouldThrottle(f.Request.URL.Host) {
		return
	}
	if t.Profile.LatencyMs > 0 {
		time.Sleep(time.Duration(t.Profile.LatencyMs) * time.Millisecond)
	}
}

func (t *ThrottleConfig) StreamRequestModifier(f *proxy.Flow, in io.Reader) io.Reader {
	if !t.shouldThrottle(f.Request.URL.Host) {
		return in
	}
	if t.Profile.UploadKbps <= 0 {
		return in
	}
	return newRateLimitedReader(in, t.Profile.UploadKbps)
}

func (t *ThrottleConfig) StreamResponseModifier(f *proxy.Flow, in io.Reader) io.Reader {
	if !t.shouldThrottle(f.Request.URL.Host) {
		return in
	}
	if t.Profile.DownloadKbps <= 0 {
		return in
	}
	return newRateLimitedReader(in, t.Profile.DownloadKbps)
}

// rateLimitedReader wraps an io.Reader with bandwidth limiting
type rateLimitedReader struct {
	reader    io.Reader
	bytesPerSec int64
	lastTime    time.Time
	bytesRead   int64
}

func newRateLimitedReader(r io.Reader, kbps int64) *rateLimitedReader {
	return &rateLimitedReader{
		reader:      r,
		bytesPerSec: kbps * 1024 / 8, // convert Kbps to bytes/sec
		lastTime:    time.Now(),
	}
}

func (r *rateLimitedReader) Read(p []byte) (int, error) {
	// Calculate how many bytes we're allowed to read
	now := time.Now()
	elapsed := now.Sub(r.lastTime).Seconds()
	if elapsed <= 0 {
		elapsed = 0.001
	}

	allowedBytes := int64(elapsed * float64(r.bytesPerSec))
	if allowedBytes <= 0 {
		allowedBytes = 1
	}

	// Limit the read size
	readSize := len(p)
	if int64(readSize) > allowedBytes {
		readSize = int(allowedBytes)
	}
	if readSize <= 0 {
		readSize = 1
	}

	n, err := r.reader.Read(p[:readSize])
	r.bytesRead += int64(n)

	// Sleep to maintain rate
	if n > 0 && r.bytesPerSec > 0 {
		expectedDuration := time.Duration(float64(n) / float64(r.bytesPerSec) * float64(time.Second))
		actualDuration := time.Since(now)
		if expectedDuration > actualDuration {
			time.Sleep(expectedDuration - actualDuration)
		}
	}

	r.lastTime = time.Now()
	return n, err
}
