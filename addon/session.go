package addon

import (
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"os"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
	uuid "github.com/satori/go.uuid"
)

const sessionVersion = "1.0"

type SessionFile struct {
	Version      string        `json:"version"`
	CreatedAt    time.Time     `json:"createdAt"`
	ProxyVersion string        `json:"proxyVersion"`
	Flows        []SessionFlow `json:"flows"`
}

type SessionFlow struct {
	Id         string           `json:"id"`
	Request    SessionRequest   `json:"request"`
	Response   *SessionResponse `json:"response,omitempty"`
	Annotation *proxy.FlowAnnotation `json:"annotation,omitempty"`
	Timing     *proxy.TimingData     `json:"timing,omitempty"`
	StartTime  time.Time        `json:"startTime"`
}

type SessionRequest struct {
	Method string              `json:"method"`
	URL    string              `json:"url"`
	Proto  string              `json:"proto"`
	Header map[string][]string `json:"header"`
	Body   string              `json:"body"` // base64 encoded
}

type SessionResponse struct {
	StatusCode int                 `json:"statusCode"`
	Header     map[string][]string `json:"header"`
	Body       string              `json:"body"` // base64 encoded
}

func SaveSession(flows []*proxy.Flow, filename string) error {
	sf := SessionFile{
		Version:   sessionVersion,
		CreatedAt: time.Now(),
		Flows:     make([]SessionFlow, 0, len(flows)),
	}

	for _, f := range flows {
		flow := SessionFlow{
			Id:         f.Id.String(),
			Annotation: f.Annotation,
			Timing:     f.Timing,
			StartTime:  f.StartTime,
		}

		if f.Request != nil {
			flow.Request = SessionRequest{
				Method: f.Request.Method,
				URL:    f.Request.URL.String(),
				Proto:  f.Request.Proto,
				Header: f.Request.Header,
				Body:   base64.StdEncoding.EncodeToString(f.Request.Body),
			}
		}

		if f.Response != nil {
			flow.Response = &SessionResponse{
				StatusCode: f.Response.StatusCode,
				Header:     f.Response.Header,
				Body:       base64.StdEncoding.EncodeToString(f.Response.Body),
			}
		}

		sf.Flows = append(sf.Flows, flow)
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	encoder := json.NewEncoder(gw)
	return encoder.Encode(sf)
}

func LoadSession(filename string) ([]*proxy.Flow, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	var sf SessionFile
	decoder := json.NewDecoder(gr)
	if err := decoder.Decode(&sf); err != nil {
		return nil, err
	}

	flows := make([]*proxy.Flow, 0, len(sf.Flows))
	for _, sf := range sf.Flows {
		id, err := uuid.FromString(sf.Id)
		if err != nil {
			id = uuid.NewV4()
		}

		reqBody, _ := base64.StdEncoding.DecodeString(sf.Request.Body)

		flow := &proxy.Flow{
			Id:         id,
			Annotation: sf.Annotation,
			Timing:     sf.Timing,
			StartTime:  sf.StartTime,
		}

		reqURL, err := parseURL(sf.Request.URL)
		if err == nil {
			flow.Request = &proxy.Request{
				Method: sf.Request.Method,
				URL:    reqURL,
				Proto:  sf.Request.Proto,
				Header: sf.Request.Header,
				Body:   reqBody,
			}
		}

		if sf.Response != nil {
			resBody, _ := base64.StdEncoding.DecodeString(sf.Response.Body)
			flow.Response = &proxy.Response{
				StatusCode: sf.Response.StatusCode,
				Header:     sf.Response.Header,
				Body:       resBody,
			}
		}

		flows = append(flows, flow)
	}

	return flows, nil
}
