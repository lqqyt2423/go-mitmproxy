package main

import (
	"fmt"
	rawLog "log"
	"os"
	"strings"

	"github.com/lqqyt2423/go-mitmproxy/addon"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"github.com/lqqyt2423/go-mitmproxy/web"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	debug    int
	version  bool
	certPath string

	addr         string
	webAddr      string
	ssl_insecure bool

	dump      string // dump filename
	dumpLevel int    // dump level

	mapperDir string

	ignoreHosts []string
	allowHosts  []string
}

func main() {
	config := loadConfig()

	if config.debug > 0 {
		rawLog.SetFlags(rawLog.LstdFlags | rawLog.Lshortfile)
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	if config.debug == 2 {
		log.SetReportCaller(true)
	}
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	opts := &proxy.Options{
		Debug:             config.debug,
		Addr:              config.addr,
		StreamLargeBodies: 1024 * 1024 * 5,
		SslInsecure:       config.ssl_insecure,
		CaRootPath:        config.certPath,
	}

	p, err := proxy.NewProxy(opts)
	if err != nil {
		log.Fatal(err)
	}

	if config.version {
		fmt.Println("go-mitmproxy: " + p.Version)
		os.Exit(0)
	}

	log.Infof("go-mitmproxy version %v\n", p.Version)

	if len(config.ignoreHosts) > 0 {
		p.SetShouldInterceptRule(func(address string) bool {
			return !matchHost(address, config.ignoreHosts)
		})
	}
	if len(config.allowHosts) > 0 {
		p.SetShouldInterceptRule(func(address string) bool {
			return matchHost(address, config.allowHosts)
		})
	}

	p.AddAddon(&proxy.LogAddon{})
	p.AddAddon(web.NewWebAddon(config.webAddr))

	if config.dump != "" {
		dumper := addon.NewDumperWithFilename(config.dump, config.dumpLevel)
		p.AddAddon(dumper)
	}

	if config.mapperDir != "" {
		mapper := addon.NewMapper(config.mapperDir)
		p.AddAddon(mapper)
	}

	log.Fatal(p.Start())
}

func matchHost(address string, hosts []string) bool {
	hostname, port := splitHostPort(address)
	for _, host := range hosts {
		h, p := splitHostPort(host)
		if h == hostname && (p == "" || p == port) {
			return true
		}
	}
	return false
}

func splitHostPort(address string) (string, string) {
	index := strings.LastIndex(address, ":")
	if index == -1 {
		return address, ""
	}
	return address[:index], address[index+1:]
}
