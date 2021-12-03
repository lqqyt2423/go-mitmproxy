package flowmapper

import (
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lqqyt2423/go-mitmproxy/addon"
	"github.com/lqqyt2423/go-mitmproxy/flow"
	_log "github.com/sirupsen/logrus"
)

var log = _log.WithField("at", "changeflow addon")
var httpsRegexp = regexp.MustCompile(`^https://`)

type Mapper struct {
	addon.Base
	reqResMap map[string]*flow.Response
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
			reqResMap: make(map[string]*flow.Response),
		}
	}

	ch := make(chan interface{}, len(filenames))
	for _, filename := range filenames {
		go func(filename string, ch chan<- interface{}) {
			f, err := ParseFlowFromFile(filename)
			if err != nil {
				log.Error(err)
				ch <- err
				return
			}
			ch <- f
		}(filename, ch)
	}

	reqResMap := make(map[string]*flow.Response)

	for i := 0; i < len(filenames); i++ {
		flowOrErr := <-ch
		if f, ok := flowOrErr.(*flow.Flow); ok {
			key := buildReqKey(f.Request)
			log.Infof("add request mapper: %v", key)
			reqResMap[key] = f.Response
		}
	}

	return &Mapper{
		reqResMap: reqResMap,
	}
}

func ParseFlowFromFile(filename string) (*flow.Flow, error) {
	p, err := NewParserFromFile(filename)
	if err != nil {
		return nil, err
	}
	return p.Parse()
}

func (c *Mapper) Request(f *flow.Flow) {
	key := buildReqKey(f.Request)
	if resp, ok := c.reqResMap[key]; ok {
		f.Response = resp
	}
}

func buildReqKey(req *flow.Request) string {
	url := req.URL.String()
	url = httpsRegexp.ReplaceAllString(url, "http://")
	key := req.Method + " " + url
	return key
}
