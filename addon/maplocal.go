package addon

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/lqqyt2423/go-mitmproxy/internal/helper"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	log "github.com/sirupsen/logrus"
)

type mapLocalTo struct {
	Path string
}

type mapLocalItem struct {
	From   *mapFrom
	To     *mapLocalTo
	Enable bool
}

func (item *mapLocalItem) match(req *proxy.Request) bool {
	if !item.Enable {
		return false
	}
	return item.From.match(req)
}

func (item *mapLocalItem) response(req *proxy.Request) (string, *proxy.Response) {
	getStat := func(filepath string) (fs.FileInfo, *proxy.Response) {
		stat, err := os.Stat(filepath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, &proxy.Response{
					StatusCode: 404,
				}
			}
			log.Errorf("map local %v os.Stat error", filepath)
			return nil, &proxy.Response{
				StatusCode: 500,
			}
		}
		return stat, nil
	}

	respFile := func(filepath string) *proxy.Response {
		file, err := os.Open(filepath)
		if err != nil {
			log.Errorf("map local %v os.Open error", filepath)
			return &proxy.Response{
				StatusCode: 500,
			}
		}
		return &proxy.Response{
			StatusCode: 200,
			BodyReader: file,
		}
	}

	stat, resp := getStat(item.To.Path)
	if resp != nil {
		return item.To.Path, resp
	}

	if !stat.IsDir() {
		return item.To.Path, respFile(item.To.Path)
	}

	// is dir
	subPath := req.URL.Path
	if item.From.Path != "" && strings.HasSuffix(item.From.Path, "/*") {
		subPath = req.URL.Path[len(item.From.Path)-2:]
	}
	filepath := path.Join(item.To.Path, subPath)

	stat, resp = getStat(filepath)
	if resp != nil {
		return filepath, resp
	}

	if !stat.IsDir() {
		return filepath, respFile(filepath)
	} else {
		log.Errorf("map local %v should be file", filepath)
		return filepath, &proxy.Response{
			StatusCode: 500,
		}
	}
}

type MapLocal struct {
	proxy.BaseAddon
	Items  []*mapLocalItem
	Enable bool
}

func (ml *MapLocal) Requestheaders(f *proxy.Flow) {
	if !ml.Enable {
		return
	}
	for _, item := range ml.Items {
		if item.match(f.Request) {
			aurl := f.Request.URL.String()
			localfile, resp := item.response(f.Request)
			log.Infof("map local %v to %v", aurl, localfile)
			f.Response = resp
			return
		}
	}
}

func (ml *MapLocal) validate() error {
	for i, item := range ml.Items {
		if item.From == nil {
			return fmt.Errorf("%v no item.From", i)
		}
		if item.From.Protocol != "" && item.From.Protocol != "http" && item.From.Protocol != "https" {
			return fmt.Errorf("%v invalid item.From.Protocol %v", i, item.From.Protocol)
		}
		if item.To == nil {
			return fmt.Errorf("%v no item.To", i)
		}
		if item.To.Path == "" {
			return fmt.Errorf("%v empty item.To.Path", i)
		}
	}
	return nil
}

func NewMapLocalFromFile(filename string) (*MapLocal, error) {
	var mapLocal MapLocal
	if err := helper.NewStructFromFile(filename, &mapLocal); err != nil {
		return nil, err
	}
	if err := mapLocal.validate(); err != nil {
		return nil, err
	}
	return &mapLocal, nil
}
