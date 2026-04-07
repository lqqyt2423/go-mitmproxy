package addon

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
	uuid "github.com/satori/go.uuid"
)

// HAR 1.2 structs

type HarRoot struct {
	Log HarLog `json:"log"`
}

type HarLog struct {
	Version string     `json:"version"`
	Creator HarCreator `json:"creator"`
	Entries []HarEntry `json:"entries"`
}

type HarCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type HarEntry struct {
	StartedDateTime string      `json:"startedDateTime"`
	Time            float64     `json:"time"`
	Request         HarRequest  `json:"request"`
	Response        HarResponse `json:"response"`
	Timings         HarTimings  `json:"timings"`
}

type HarRequest struct {
	Method      string      `json:"method"`
	URL         string      `json:"url"`
	HTTPVersion string      `json:"httpVersion"`
	Headers     []HarHeader `json:"headers"`
	QueryString []HarQuery  `json:"queryString"`
	BodySize    int         `json:"bodySize"`
	PostData    *HarPostData `json:"postData,omitempty"`
}

type HarResponse struct {
	Status      int         `json:"status"`
	StatusText  string      `json:"statusText"`
	HTTPVersion string      `json:"httpVersion"`
	Headers     []HarHeader `json:"headers"`
	Content     HarContent  `json:"content"`
	BodySize    int         `json:"bodySize"`
}

type HarHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type HarQuery struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type HarPostData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
	Encoding string `json:"encoding,omitempty"`
}

type HarContent struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
	Encoding string `json:"encoding,omitempty"`
}

type HarTimings struct {
	DNS     float64 `json:"dns"`
	Connect float64 `json:"connect"`
	SSL     float64 `json:"ssl"`
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
}

func ExportHAR(flows []*proxy.Flow, filename string) error {
	har := HarRoot{
		Log: HarLog{
			Version: "1.2",
			Creator: HarCreator{
				Name:    "go-mitmproxy",
				Version: "1.0",
			},
			Entries: make([]HarEntry, 0, len(flows)),
		},
	}

	for _, f := range flows {
		if f.Request == nil {
			continue
		}

		entry := HarEntry{
			StartedDateTime: f.StartTime.Format(time.RFC3339Nano),
			Request:         flowRequestToHAR(f.Request),
		}

		if f.Response != nil {
			entry.Response = flowResponseToHAR(f.Response)
		} else {
			entry.Response = HarResponse{Status: 0, StatusText: "", Headers: []HarHeader{}, Content: HarContent{}}
		}

		if f.Timing != nil {
			entry.Timings = HarTimings{
				DNS:     float64(f.Timing.DnsMs),
				Connect: float64(f.Timing.ConnectMs),
				SSL:     float64(f.Timing.TlsMs),
				Send:    float64(f.Timing.SendMs),
				Wait:    float64(f.Timing.WaitMs),
				Receive: float64(f.Timing.ReceiveMs),
			}
			entry.Time = float64(f.Timing.DnsMs + f.Timing.ConnectMs + f.Timing.TlsMs +
				f.Timing.SendMs + f.Timing.WaitMs + f.Timing.ReceiveMs)
		} else {
			entry.Timings = HarTimings{DNS: -1, Connect: -1, SSL: -1, Send: -1, Wait: -1, Receive: -1}
		}

		har.Log.Entries = append(har.Log.Entries, entry)
	}

	data, err := json.MarshalIndent(har, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func flowRequestToHAR(req *proxy.Request) HarRequest {
	headers := make([]HarHeader, 0)
	for name, vals := range req.Header {
		for _, v := range vals {
			headers = append(headers, HarHeader{Name: name, Value: v})
		}
	}

	queryString := make([]HarQuery, 0)
	if req.URL != nil {
		for name, vals := range req.URL.Query() {
			for _, v := range vals {
				queryString = append(queryString, HarQuery{Name: name, Value: v})
			}
		}
	}

	harReq := HarRequest{
		Method:      req.Method,
		URL:         req.URL.String(),
		HTTPVersion: req.Proto,
		Headers:     headers,
		QueryString: queryString,
		BodySize:    len(req.Body),
	}

	if len(req.Body) > 0 {
		mimeType := ""
		if ct := req.Header.Get("Content-Type"); ct != "" {
			mimeType = ct
		}
		harReq.PostData = &HarPostData{
			MimeType: mimeType,
			Text:     base64.StdEncoding.EncodeToString(req.Body),
			Encoding: "base64",
		}
	}

	return harReq
}

func flowResponseToHAR(res *proxy.Response) HarResponse {
	headers := make([]HarHeader, 0)
	for name, vals := range res.Header {
		for _, v := range vals {
			headers = append(headers, HarHeader{Name: name, Value: v})
		}
	}

	mimeType := ""
	if ct := res.Header.Get("Content-Type"); ct != "" {
		mimeType = ct
	}

	content := HarContent{
		Size:     len(res.Body),
		MimeType: mimeType,
	}
	if len(res.Body) > 0 {
		content.Text = base64.StdEncoding.EncodeToString(res.Body)
		content.Encoding = "base64"
	}

	return HarResponse{
		Status:      res.StatusCode,
		StatusText:  "",
		HTTPVersion: "HTTP/1.1",
		Headers:     headers,
		Content:     content,
		BodySize:    len(res.Body),
	}
}

func ImportHAR(filename string) ([]*proxy.Flow, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var har HarRoot
	if err := json.Unmarshal(data, &har); err != nil {
		return nil, err
	}

	flows := make([]*proxy.Flow, 0, len(har.Log.Entries))
	for _, entry := range har.Log.Entries {
		flow := &proxy.Flow{
			Id: uuid.NewV4(),
		}

		if t, err := time.Parse(time.RFC3339Nano, entry.StartedDateTime); err == nil {
			flow.StartTime = t
		} else {
			flow.StartTime = time.Now()
		}

		// Parse request
		reqURL, err := parseURL(entry.Request.URL)
		if err != nil {
			continue
		}

		reqHeader := make(map[string][]string)
		for _, h := range entry.Request.Headers {
			reqHeader[h.Name] = append(reqHeader[h.Name], h.Value)
		}

		var reqBody []byte
		if entry.Request.PostData != nil && entry.Request.PostData.Text != "" {
			if entry.Request.PostData.Encoding == "base64" {
				reqBody, _ = base64.StdEncoding.DecodeString(entry.Request.PostData.Text)
			} else {
				reqBody = []byte(entry.Request.PostData.Text)
			}
		}

		flow.Request = &proxy.Request{
			Method: entry.Request.Method,
			URL:    reqURL,
			Proto:  entry.Request.HTTPVersion,
			Header: reqHeader,
			Body:   reqBody,
		}

		// Parse response
		if entry.Response.Status > 0 {
			resHeader := make(map[string][]string)
			for _, h := range entry.Response.Headers {
				resHeader[h.Name] = append(resHeader[h.Name], h.Value)
			}

			var resBody []byte
			if entry.Response.Content.Text != "" {
				if entry.Response.Content.Encoding == "base64" {
					resBody, _ = base64.StdEncoding.DecodeString(entry.Response.Content.Text)
				} else {
					resBody = []byte(entry.Response.Content.Text)
				}
			}

			flow.Response = &proxy.Response{
				StatusCode: entry.Response.Status,
				Header:     resHeader,
				Body:       resBody,
			}
		}

		flows = append(flows, flow)
	}

	return flows, nil
}

func parseURL(rawurl string) (*url.URL, error) {
	return url.Parse(rawurl)
}
