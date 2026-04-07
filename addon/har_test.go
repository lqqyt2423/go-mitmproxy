package addon

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
	uuid "github.com/satori/go.uuid"
)

func TestExportHAR(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "export-*.har")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	flows := []*proxy.Flow{
		{
			Id:        uuid.NewV4(),
			StartTime: time.Now(),
			Request: &proxy.Request{
				Method: "GET",
				URL:    &url.URL{Scheme: "https", Host: "example.com", Path: "/api/users", RawQuery: "page=1&limit=10"},
				Proto:  "HTTP/1.1",
				Header: http.Header{"Accept": []string{"application/json"}},
			},
			Response: &proxy.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       []byte(`[{"id":1}]`),
			},
			Timing: &proxy.TimingData{
				DnsMs: 10, ConnectMs: 20, TlsMs: 30,
				SendMs: 5, WaitMs: 100, ReceiveMs: 15,
			},
		},
	}

	if err := ExportHAR(flows, tmpFile.Name()); err != nil {
		t.Fatalf("ExportHAR error: %v", err)
	}

	// Read and parse
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	var har HarRoot
	if err := json.Unmarshal(data, &har); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if har.Log.Version != "1.2" {
		t.Errorf("expected HAR version 1.2, got %s", har.Log.Version)
	}
	if len(har.Log.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(har.Log.Entries))
	}

	entry := har.Log.Entries[0]
	if entry.Request.Method != "GET" {
		t.Errorf("request method: got %s", entry.Request.Method)
	}
	if entry.Response.Status != 200 {
		t.Errorf("response status: got %d", entry.Response.Status)
	}
	if entry.Timings.DNS != 10 {
		t.Errorf("DNS timing: got %f, want 10", entry.Timings.DNS)
	}
	if entry.Time != 180 { // 10+20+30+5+100+15
		t.Errorf("total time: got %f, want 180", entry.Time)
	}

	// Verify query string parsed
	if len(entry.Request.QueryString) != 2 {
		t.Errorf("expected 2 query params, got %d", len(entry.Request.QueryString))
	}
}

func TestExportHARBinaryBody(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "export-binary-*.har")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	flows := []*proxy.Flow{
		{
			Id:        uuid.NewV4(),
			StartTime: time.Now(),
			Request: &proxy.Request{
				Method: "POST",
				URL:    &url.URL{Scheme: "https", Host: "example.com", Path: "/upload"},
				Proto:  "HTTP/1.1",
				Header: http.Header{"Content-Type": []string{"application/octet-stream"}},
				Body:   []byte{0x00, 0x01, 0x02, 0xFF},
			},
			Response: &proxy.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"image/png"}},
				Body:       []byte{0x89, 0x50, 0x4E, 0x47},
			},
		},
	}

	if err := ExportHAR(flows, tmpFile.Name()); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	var har HarRoot
	json.Unmarshal(data, &har)

	entry := har.Log.Entries[0]
	if entry.Response.Content.Encoding != "base64" {
		t.Errorf("binary body should be base64 encoded, got encoding: %s", entry.Response.Content.Encoding)
	}
	if entry.Request.PostData == nil {
		t.Fatal("post data should exist for request body")
	}
	if entry.Request.PostData.Encoding != "base64" {
		t.Errorf("request body should be base64 encoded")
	}
}

func TestExportHAREmptyFlows(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "export-empty-*.har")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	if err := ExportHAR([]*proxy.Flow{}, tmpFile.Name()); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(tmpFile.Name())
	var har HarRoot
	json.Unmarshal(data, &har)

	if len(har.Log.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(har.Log.Entries))
	}
}

func TestImportHAR(t *testing.T) {
	// Create a minimal HAR file
	har := HarRoot{
		Log: HarLog{
			Version: "1.2",
			Creator: HarCreator{Name: "test", Version: "1.0"},
			Entries: []HarEntry{
				{
					StartedDateTime: time.Now().Format(time.RFC3339Nano),
					Request: HarRequest{
						Method:      "GET",
						URL:         "https://example.com/test",
						HTTPVersion: "HTTP/1.1",
						Headers:     []HarHeader{{Name: "Accept", Value: "*/*"}},
						QueryString: []HarQuery{},
					},
					Response: HarResponse{
						Status:      200,
						HTTPVersion: "HTTP/1.1",
						Headers:     []HarHeader{{Name: "Content-Type", Value: "text/plain"}},
						Content: HarContent{
							Size:     5,
							MimeType: "text/plain",
							Text:     "hello",
						},
					},
					Timings: HarTimings{DNS: -1, Connect: -1, SSL: -1, Send: -1, Wait: -1, Receive: -1},
				},
			},
		},
	}

	tmpFile, err := os.CreateTemp("", "import-*.har")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	data, _ := json.Marshal(har)
	os.WriteFile(tmpFile.Name(), data, 0644)

	flows, err := ImportHAR(tmpFile.Name())
	if err != nil {
		t.Fatalf("ImportHAR error: %v", err)
	}

	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}

	f := flows[0]
	if f.Request.Method != "GET" {
		t.Errorf("method: got %s", f.Request.Method)
	}
	if f.Request.URL.String() != "https://example.com/test" {
		t.Errorf("URL: got %s", f.Request.URL.String())
	}
	if f.Response.StatusCode != 200 {
		t.Errorf("status: got %d", f.Response.StatusCode)
	}
	if string(f.Response.Body) != "hello" {
		t.Errorf("response body: got %s", string(f.Response.Body))
	}
}

func TestImportHAREmptyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "import-empty-*.har")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	har := HarRoot{Log: HarLog{Version: "1.2", Entries: []HarEntry{}}}
	data, _ := json.Marshal(har)
	os.WriteFile(tmpFile.Name(), data, 0644)

	flows, err := ImportHAR(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if len(flows) != 0 {
		t.Errorf("expected 0 flows, got %d", len(flows))
	}
}
