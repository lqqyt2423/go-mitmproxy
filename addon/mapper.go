package addon

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
	log "github.com/sirupsen/logrus"
)

var httpsRegexp = regexp.MustCompile(`^https://`)

type Mapper struct {
	proxy.BaseAddon
	reqResMap map[string]*proxy.Response
}

func NewMapper(dirname string) *Mapper {
	infos, err := ioutil.ReadDir(dirname)
	if err != nil {
		panic(err)
	}

	filenames := make([]string, 0)

	for _, info := range infos {
		if info.IsDir() {
			continue
		}
		if !strings.HasSuffix(info.Name(), ".map.txt") {
			continue
		}

		filenames = append(filenames, filepath.Join(dirname, info.Name()))
	}

	if len(filenames) == 0 {
		return &Mapper{
			reqResMap: make(map[string]*proxy.Response),
		}
	}

	ch := make(chan interface{}, len(filenames))
	for _, filename := range filenames {
		go func(filename string, ch chan<- interface{}) {
			f, err := parseFlowFromFile(filename)
			if err != nil {
				log.Error(err)
				ch <- err
				return
			}
			ch <- f
		}(filename, ch)
	}

	reqResMap := make(map[string]*proxy.Response)

	for i := 0; i < len(filenames); i++ {
		flowOrErr := <-ch
		if f, ok := flowOrErr.(*proxy.Flow); ok {
			key := buildReqKey(f.Request)
			log.Infof("add request mapper: %v", key)
			reqResMap[key] = f.Response
		}
	}

	return &Mapper{
		reqResMap: reqResMap,
	}
}

func parseFlowFromFile(filename string) (*proxy.Flow, error) {
	p, err := newMapperParserFromFile(filename)
	if err != nil {
		return nil, err
	}
	return p.parse()
}

func (c *Mapper) Request(f *proxy.Flow) {
	key := buildReqKey(f.Request)
	if resp, ok := c.reqResMap[key]; ok {
		f.Response = resp
	}
}

func buildReqKey(req *proxy.Request) string {
	url := req.URL.String()
	url = httpsRegexp.ReplaceAllString(url, "http://")
	key := req.Method + " " + url
	return key
}

type mapperParser struct {
	lines    []string
	url      string
	request  *proxy.Request
	response *proxy.Response
}

func newMapperParserFromFile(filename string) (*mapperParser, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return newMapperParserFromString(string(bytes))
}

func newMapperParserFromString(content string) (*mapperParser, error) {
	content = strings.TrimSpace(content)
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil, errors.New("no lines")
	}

	return &mapperParser{
		lines: lines,
	}, nil
}

func (p *mapperParser) parse() (*proxy.Flow, error) {
	if err := p.parseRequest(); err != nil {
		return nil, err
	}

	if err := p.parseResponse(); err != nil {
		return nil, err
	}

	return &proxy.Flow{
		Request:  p.request,
		Response: p.response,
	}, nil
}

func (p *mapperParser) parseRequest() error {
	if err := p.parseReqHead(); err != nil {
		return err
	}

	if header, err := p.parseHeader(); err != nil {
		return err
	} else {
		p.request.Header = header
	}

	// parse url
	if !strings.HasPrefix(p.url, "http") {
		host := p.request.Header.Get("host")
		if host == "" {
			return errors.New("no request host")
		}
		p.url = "http://" + host + p.url
	}
	url, err := url.Parse(p.url)
	if err != nil {
		return err
	}
	p.request.URL = url

	p.parseReqBody()

	return nil
}

func (p *mapperParser) parseReqHead() error {
	line, _ := p.getLine()
	re := regexp.MustCompile(`^(GET|POST|PUT|DELETE)\s+?(.+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) == 0 {
		return errors.New("request head parse error")
	}

	p.request = &proxy.Request{
		Method: matches[1],
	}
	p.url = matches[2]

	return nil
}

func (p *mapperParser) parseHeader() (http.Header, error) {
	header := make(http.Header)
	re := regexp.MustCompile(`^([\w-]+?):\s*(.+)$`)

	for {
		line, ok := p.getLine()
		if !ok {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		matches := re.FindStringSubmatch(line)
		if len(matches) == 0 {
			return nil, errors.New("request header parse error")
		}

		key := matches[1]
		val := matches[2]
		header.Add(key, val)
	}

	return header, nil
}

func (p *mapperParser) parseReqBody() {
	bodyLines := make([]string, 0)

	for {
		line, ok := p.getLine()
		if !ok {
			break
		}

		if len(bodyLines) == 0 {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
		}

		if strings.HasPrefix(line, "HTTP/1.1 ") {
			p.lines = append([]string{line}, p.lines...)
			break
		}
		bodyLines = append(bodyLines, line)
	}

	body := strings.Join(bodyLines, "\n")
	body = strings.TrimSpace(body)
	p.request.Body = []byte(body)
}

func (p *mapperParser) parseResponse() error {
	if err := p.parseResHead(); err != nil {
		return err
	}

	if header, err := p.parseHeader(); err != nil {
		return err
	} else {
		p.response.Header = header
	}

	// all left content
	body := strings.Join(p.lines, "\n")
	body = strings.TrimSpace(body)
	p.response.Body = []byte(body)
	p.response.Header.Set("Content-Length", strconv.Itoa(len(p.response.Body)))

	return nil
}

func (p *mapperParser) parseResHead() error {
	line, ok := p.getLine()
	if !ok {
		return errors.New("response no head line")
	}

	re := regexp.MustCompile(`^HTTP/1\.1\s+?(\d+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) == 0 {
		return errors.New("response head parse error")
	}

	code, _ := strconv.Atoi(matches[1])
	p.response = &proxy.Response{
		StatusCode: code,
	}

	return nil
}

func (p *mapperParser) getLine() (string, bool) {
	if len(p.lines) == 0 {
		return "", false
	}

	line := p.lines[0]
	p.lines = p.lines[1:]
	return line, true
}
