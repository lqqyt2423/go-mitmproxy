package addon

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/lqqyt2423/go-mitmproxy/internal/helper"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"github.com/tidwall/match"
)

type RewriteAction struct {
	Type      string `json:"Type"`      // addHeader, modifyHeader, removeHeader, host, path, queryParam, addQueryParam, removeQueryParam, status, body
	Target    string `json:"Target"`    // request or response
	Name      string `json:"Name"`      // header name or param name
	Value     string `json:"Value"`     // match value or new value
	Replace   string `json:"Replace"`   // replacement value
	MatchMode string `json:"MatchMode"` // text or regex (default: text)
}

type RewriteMatch struct {
	Protocol string   `json:"Protocol"`
	Host     string   `json:"Host"`
	Port     string   `json:"Port"`
	Path     string   `json:"Path"`
	Method   []string `json:"Method"`
}

type RewriteItem struct {
	Enable bool           `json:"Enable"`
	Name   string         `json:"Name"`
	From   RewriteMatch   `json:"From"`
	Rules  []RewriteAction `json:"Rules"`
}

type RewriteRuleSet struct {
	proxy.BaseAddon
	Enable bool           `json:"Enable"`
	Items  []*RewriteItem `json:"Items"`
}

func NewRewriteFromFile(filename string) (*RewriteRuleSet, error) {
	ruleSet := new(RewriteRuleSet)
	if err := helper.NewStructFromFile(filename, ruleSet); err != nil {
		return nil, err
	}
	return ruleSet, nil
}

func NewRewrite() *RewriteRuleSet {
	return &RewriteRuleSet{
		Enable: true,
		Items:  make([]*RewriteItem, 0),
	}
}

func (rw *RewriteRuleSet) AddHeaderRule(urlPattern, target, actionType, headerName, headerValue string) {
	rw.Items = append(rw.Items, &RewriteItem{
		Enable: true,
		Name:   fmt.Sprintf("%s %s: %s", actionType, headerName, headerValue),
		From:   RewriteMatch{Host: urlPattern},
		Rules: []RewriteAction{
			{Type: actionType, Target: target, Name: headerName, Value: headerValue},
		},
	})
}

func matchRewriteItem(item *RewriteItem, req *proxy.Request) bool {
	from := item.From

	if from.Protocol != "" {
		scheme := req.URL.Scheme
		if scheme == "" {
			scheme = "http"
		}
		if !strings.EqualFold(from.Protocol, scheme) {
			return false
		}
	}

	if from.Host != "" && !match.Match(req.URL.Host, from.Host) {
		return false
	}

	if from.Port != "" {
		port := req.URL.Port()
		if port == "" {
			if req.URL.Scheme == "https" {
				port = "443"
			} else {
				port = "80"
			}
		}
		if from.Port != port {
			return false
		}
	}

	if from.Path != "" && !match.Match(req.URL.Path, from.Path) {
		return false
	}

	if len(from.Method) > 0 {
		found := false
		for _, m := range from.Method {
			if strings.EqualFold(m, req.Method) {
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

func (rw *RewriteRuleSet) Requestheaders(f *proxy.Flow) {
	if !rw.Enable {
		return
	}

	for _, item := range rw.Items {
		if !item.Enable {
			continue
		}
		if !matchRewriteItem(item, f.Request) {
			continue
		}
		for _, rule := range item.Rules {
			if rule.Target != "request" {
				continue
			}
			applyRewriteAction(&rule, f.Request, nil)
		}
	}
}

func (rw *RewriteRuleSet) Response(f *proxy.Flow) {
	if !rw.Enable || f.Response == nil {
		return
	}

	for _, item := range rw.Items {
		if !item.Enable {
			continue
		}
		if !matchRewriteItem(item, f.Request) {
			continue
		}
		for _, rule := range item.Rules {
			if rule.Target != "response" {
				continue
			}
			applyRewriteAction(&rule, f.Request, f.Response)
		}
	}
}

func applyRewriteAction(action *RewriteAction, req *proxy.Request, res *proxy.Response) {
	switch action.Type {
	case "addHeader":
		if action.Target == "request" {
			req.Header.Add(action.Name, action.Value)
		} else if res != nil {
			res.Header.Add(action.Name, action.Value)
		}

	case "modifyHeader":
		if action.Target == "request" {
			modifyHeader(req.Header, action)
		} else if res != nil {
			modifyHeader(res.Header, action)
		}

	case "removeHeader":
		if action.Target == "request" {
			req.Header.Del(action.Name)
		} else if res != nil {
			res.Header.Del(action.Name)
		}

	case "host":
		newHost := replaceValue(req.URL.Host, action.Value, action.Replace, action.MatchMode)
		req.URL.Host = newHost

	case "path":
		newPath := replaceValue(req.URL.Path, action.Value, action.Replace, action.MatchMode)
		req.URL.Path = newPath

	case "addQueryParam":
		q := req.URL.Query()
		q.Add(action.Name, action.Value)
		req.URL.RawQuery = q.Encode()

	case "queryParam":
		q := req.URL.Query()
		if q.Has(action.Name) {
			oldVal := q.Get(action.Name)
			newVal := replaceValue(oldVal, action.Value, action.Replace, action.MatchMode)
			q.Set(action.Name, newVal)
			req.URL.RawQuery = q.Encode()
		}

	case "removeQueryParam":
		q := req.URL.Query()
		q.Del(action.Name)
		req.URL.RawQuery = q.Encode()

	case "status":
		if res != nil {
			var code int
			fmt.Sscanf(action.Value, "%d", &code)
			if code > 0 {
				res.StatusCode = code
			}
		}

	case "body":
		if res != nil && res.Body != nil {
			bodyStr := string(res.Body)
			newBody := replaceValue(bodyStr, action.Value, action.Replace, action.MatchMode)
			res.Body = []byte(newBody)
			res.Header.Set("Content-Length", fmt.Sprintf("%d", len(res.Body)))
		}
	}
}

func modifyHeader(header http.Header, action *RewriteAction) {
	if vals := header.Values(action.Name); len(vals) > 0 {
		newVal := replaceValue(vals[0], action.Value, action.Replace, action.MatchMode)
		header.Set(action.Name, newVal)
	}
}

func replaceValue(s, pattern, replacement, mode string) string {
	if mode == "regex" {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return s
		}
		return re.ReplaceAllString(s, replacement)
	}
	// text mode: exact string replacement
	return strings.ReplaceAll(s, pattern, replacement)
}
