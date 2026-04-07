package addon

import (
	"net/http"

	"github.com/lqqyt2423/go-mitmproxy/internal/helper"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"github.com/tidwall/match"
)

type BlockRule struct {
	Enable     bool     `json:"Enable"`
	Host       string   `json:"Host"`
	Path       string   `json:"Path"`
	Method     []string `json:"Method"`
	StatusCode int      `json:"StatusCode"`
	Body       string   `json:"Body"`
}

type BlockListConfig struct {
	proxy.BaseAddon
	Enable bool        `json:"Enable"`
	Items  []*BlockRule `json:"Items"`
}

func NewBlockListFromFile(filename string) (*BlockListConfig, error) {
	config := new(BlockListConfig)
	if err := helper.NewStructFromFile(filename, config); err != nil {
		return nil, err
	}
	return config, nil
}

func NewBlockList() *BlockListConfig {
	return &BlockListConfig{
		Enable: true,
		Items:  make([]*BlockRule, 0),
	}
}

func (bl *BlockListConfig) AddRule(host, path string, statusCode int, body string) {
	if statusCode == 0 {
		statusCode = 403
	}
	if body == "" {
		body = "Blocked by go-mitmproxy"
	}
	bl.Items = append(bl.Items, &BlockRule{
		Enable:     true,
		Host:       host,
		Path:       path,
		StatusCode: statusCode,
		Body:       body,
	})
}

func (bl *BlockListConfig) Requestheaders(f *proxy.Flow) {
	if !bl.Enable {
		return
	}

	for _, rule := range bl.Items {
		if !rule.Enable {
			continue
		}
		if !matchBlockRule(rule, f.Request) {
			continue
		}

		statusCode := rule.StatusCode
		if statusCode == 0 {
			statusCode = 403
		}
		body := rule.Body
		if body == "" {
			body = "Blocked by go-mitmproxy"
		}

		f.Response = &proxy.Response{
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       []byte(body),
		}
		f.Response.Header.Set("Content-Type", "text/plain; charset=utf-8")
		return
	}
}

func matchBlockRule(rule *BlockRule, req *proxy.Request) bool {
	// match host
	if rule.Host != "" && !match.Match(req.URL.Host, rule.Host) {
		return false
	}

	// match path
	if rule.Path != "" && !match.Match(req.URL.Path, rule.Path) {
		return false
	}

	// match method
	if len(rule.Method) > 0 {
		found := false
		for _, m := range rule.Method {
			if m == req.Method {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
