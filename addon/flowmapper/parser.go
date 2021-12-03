package flowmapper

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/lqqyt2423/go-mitmproxy/flow"
)

type Parser struct {
	lines    []string
	url      string
	request  *flow.Request
	response *flow.Response
}

func NewParserFromFile(filename string) (*Parser, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return NewParserFromString(string(bytes))
}

func NewParserFromString(content string) (*Parser, error) {
	content = strings.TrimSpace(content)
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil, errors.New("no lines")
	}

	return &Parser{
		lines: lines,
	}, nil
}

func (p *Parser) Parse() (*flow.Flow, error) {
	if err := p.parseRequest(); err != nil {
		return nil, err
	}

	if err := p.parseResponse(); err != nil {
		return nil, err
	}

	return &flow.Flow{
		Request:  p.request,
		Response: p.response,
	}, nil
}

func (p *Parser) parseRequest() error {
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

func (p *Parser) parseReqHead() error {
	line, _ := p.getLine()
	re := regexp.MustCompile(`^(GET|POST|PUT|DELETE)\s+?(.+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) == 0 {
		return errors.New("request head parse error")
	}

	p.request = &flow.Request{
		Method: matches[1],
	}
	p.url = matches[2]

	return nil
}

func (p *Parser) parseHeader() (http.Header, error) {
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

func (p *Parser) parseReqBody() {
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

func (p *Parser) parseResponse() error {
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

func (p *Parser) parseResHead() error {
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
	p.response = &flow.Response{
		StatusCode: code,
	}

	return nil
}

func (p *Parser) getLine() (string, bool) {
	if len(p.lines) == 0 {
		return "", false
	}

	line := p.lines[0]
	p.lines = p.lines[1:]
	return line, true
}
