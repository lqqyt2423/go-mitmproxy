package addon

import (
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
	uuid "github.com/satori/go.uuid"
)

func makeTestFlows() []*proxy.Flow {
	return []*proxy.Flow{
		{
			Id:        uuid.NewV4(),
			StartTime: time.Now(),
			Request: &proxy.Request{
				Method: "GET",
				URL:    &url.URL{Scheme: "https", Host: "example.com", Path: "/api/data"},
				Proto:  "HTTP/1.1",
				Header: http.Header{"Accept": []string{"application/json"}},
				Body:   nil,
			},
			Response: &proxy.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       []byte(`{"key":"value"}`),
			},
		},
		{
			Id:        uuid.NewV4(),
			StartTime: time.Now(),
			Request: &proxy.Request{
				Method: "POST",
				URL:    &url.URL{Scheme: "https", Host: "api.example.com", Path: "/submit"},
				Proto:  "HTTP/2.0",
				Header: http.Header{"Content-Type": []string{"application/json"}},
				Body:   []byte(`{"name":"test"}`),
			},
			Response: &proxy.Response{
				StatusCode: 201,
				Header:     http.Header{"Location": []string{"/submit/123"}},
				Body:       []byte(`{"id":123}`),
			},
			Annotation: &proxy.FlowAnnotation{Color: "red", Comment: "important"},
		},
	}
}

func TestSessionSaveLoad(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "session-*.gmps")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	original := makeTestFlows()

	// Save
	if err := SaveSession(original, tmpFile.Name()); err != nil {
		t.Fatalf("SaveSession error: %v", err)
	}

	// Verify file exists and is non-empty
	info, err := os.Stat(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Error("session file should not be empty")
	}

	// Load
	loaded, err := LoadSession(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadSession error: %v", err)
	}

	if len(loaded) != len(original) {
		t.Fatalf("expected %d flows, got %d", len(original), len(loaded))
	}

	// Verify first flow
	f0 := loaded[0]
	if f0.Id.String() != original[0].Id.String() {
		t.Errorf("flow ID mismatch: got %s, want %s", f0.Id, original[0].Id)
	}
	if f0.Request.Method != "GET" {
		t.Errorf("request method: got %s, want GET", f0.Request.Method)
	}
	if f0.Request.URL.String() != "https://example.com/api/data" {
		t.Errorf("request URL: got %s", f0.Request.URL.String())
	}
	if f0.Response.StatusCode != 200 {
		t.Errorf("response status: got %d, want 200", f0.Response.StatusCode)
	}
	if string(f0.Response.Body) != `{"key":"value"}` {
		t.Errorf("response body: got %s", string(f0.Response.Body))
	}

	// Verify second flow with annotation
	f1 := loaded[1]
	if f1.Request.Method != "POST" {
		t.Errorf("second flow method: got %s, want POST", f1.Request.Method)
	}
	if string(f1.Request.Body) != `{"name":"test"}` {
		t.Errorf("request body: got %s", string(f1.Request.Body))
	}
	if f1.Annotation == nil {
		t.Fatal("annotation should be preserved")
	}
	if f1.Annotation.Color != "red" {
		t.Errorf("annotation color: got %s, want red", f1.Annotation.Color)
	}
	if f1.Annotation.Comment != "important" {
		t.Errorf("annotation comment: got %s", f1.Annotation.Comment)
	}
}

func TestSessionEmptyFlows(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "session-empty-*.gmps")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	if err := SaveSession([]*proxy.Flow{}, tmpFile.Name()); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadSession(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected 0 flows, got %d", len(loaded))
	}
}

func TestSessionNilResponse(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "session-nilresp-*.gmps")
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
				URL:    &url.URL{Scheme: "https", Host: "example.com", Path: "/"},
				Proto:  "HTTP/1.1",
				Header: make(http.Header),
			},
			// Response is nil
		},
	}

	if err := SaveSession(flows, tmpFile.Name()); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadSession(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(loaded))
	}
	if loaded[0].Response != nil {
		t.Error("response should be nil")
	}
}

func TestSessionCorruptedFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "session-corrupt-*.gmps")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.WriteString("this is not gzip data")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	_, err = LoadSession(tmpFile.Name())
	if err == nil {
		t.Error("expected error for corrupted file")
	}
}
