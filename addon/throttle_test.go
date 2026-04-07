package addon

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"
)

func TestThrottleFromFile(t *testing.T) {
	content := `{"Enable":true,"Hosts":["*.example.com"],"Profile":{"Name":"3G","DownloadKbps":750,"UploadKbps":250,"LatencyMs":200}}`
	f, err := os.CreateTemp("", "throttle-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	tc, err := NewThrottleFromFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !tc.Enable {
		t.Error("expected Enable to be true")
	}
	if tc.Profile.Name != "3G" {
		t.Errorf("expected profile name 3G, got %s", tc.Profile.Name)
	}
	if tc.Profile.DownloadKbps != 750 {
		t.Errorf("expected download 750, got %d", tc.Profile.DownloadKbps)
	}
}

func TestThrottlePresets(t *testing.T) {
	if Preset3G.DownloadKbps != 750 {
		t.Errorf("3G preset download should be 750, got %d", Preset3G.DownloadKbps)
	}
	if Preset3G.UploadKbps != 250 {
		t.Errorf("3G preset upload should be 250, got %d", Preset3G.UploadKbps)
	}
	if Preset4G.DownloadKbps != 12000 {
		t.Errorf("4G preset download should be 12000, got %d", Preset4G.DownloadKbps)
	}
	if PresetFullLoss.PacketLossPercent != 100 {
		t.Errorf("Full loss should be 100%%, got %f", PresetFullLoss.PacketLossPercent)
	}
}

func TestThrottleShouldThrottle(t *testing.T) {
	tc := NewThrottle(Preset3G)

	t.Run("empty hosts matches all", func(t *testing.T) {
		if !tc.shouldThrottle("anything.com") {
			t.Error("empty hosts should match all")
		}
	})

	t.Run("disabled returns false", func(t *testing.T) {
		tc2 := NewThrottle(Preset3G)
		tc2.Enable = false
		if tc2.shouldThrottle("anything.com") {
			t.Error("disabled should not throttle")
		}
	})

	t.Run("glob pattern matching", func(t *testing.T) {
		tc3 := NewThrottle(Preset3G)
		tc3.Hosts = []string{"*.example.com"}

		if !tc3.shouldThrottle("api.example.com") {
			t.Error("should match *.example.com")
		}
		if tc3.shouldThrottle("other.com") {
			t.Error("should not match other.com")
		}
	})
}

func TestRateLimitedReader(t *testing.T) {
	// Create a 10KB data source
	data := make([]byte, 10*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	reader := bytes.NewReader(data)

	// Rate limit to 100 Kbps = 12800 bytes/sec
	rlr := newRateLimitedReader(reader, 100)

	start := time.Now()
	buf := make([]byte, 0, len(data))
	tmp := make([]byte, 1024)
	for {
		n, err := rlr.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(start)

	// Verify all data was read
	if len(buf) != len(data) {
		t.Errorf("expected %d bytes read, got %d", len(data), len(buf))
	}

	// Verify it took at least some time (rate limiting worked)
	// 10KB at 100Kbps should take ~0.6s, but we allow wide tolerance
	if elapsed < 200*time.Millisecond {
		t.Errorf("rate limiting should slow reads, elapsed: %v", elapsed)
	}
}

func TestRateLimitedReaderEOF(t *testing.T) {
	data := []byte("hello")
	reader := bytes.NewReader(data)
	rlr := newRateLimitedReader(reader, 1000) // fast enough to not delay

	buf, err := io.ReadAll(rlr)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "hello" {
		t.Errorf("expected 'hello', got '%s'", string(buf))
	}
}

func TestThrottleInvalidFile(t *testing.T) {
	_, err := NewThrottleFromFile("/nonexistent/file.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
