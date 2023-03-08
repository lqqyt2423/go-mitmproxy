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
	version bool // show go-mitmproxy version

	Addr        string   // proxy listen addr
	WebAddr     string   // web interface listen addr
	SslInsecure bool     // not verify upstream server SSL/TLS certificates.
	IgnoreHosts []string // a list of ignore hosts
	AllowHosts  []string // a list of allow hosts
	CertPath    string   // path of generate cert files
	Debug       int      // debug mode: 1 - print debug log, 2 - show debug from
	Dump        string   // dump filename
	DumpLevel   int      // dump level: 0 - header, 1 - header + body
	MapRemote   string   // map remote config filename
	MapLocal    string   // map local config filename

	filename string // read config from the filename
}

func main() {
	config := loadConfig()

	if config.Debug > 0 {
		rawLog.SetFlags(rawLog.LstdFlags | rawLog.Lshortfile)
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	if config.Debug == 2 {
		log.SetReportCaller(true)
	}
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	opts := &proxy.Options{
		Debug:             config.Debug,
		Addr:              config.Addr,
		StreamLargeBodies: 1024 * 1024 * 5,
		SslInsecure:       config.SslInsecure,
		CaRootPath:        config.CertPath,
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

	if len(config.IgnoreHosts) > 0 {
		p.SetShouldInterceptRule(func(address string) bool {
			return !matchHost(address, config.IgnoreHosts)
		})
	}
	if len(config.AllowHosts) > 0 {
		p.SetShouldInterceptRule(func(address string) bool {
			return matchHost(address, config.AllowHosts)
		})
	}

	p.AddAddon(&proxy.LogAddon{})
	p.AddAddon(web.NewWebAddon(config.WebAddr))

	if config.MapRemote != "" {
		mapRemote, err := addon.NewMapRemoteFromFile(config.MapRemote)
		if err != nil {
			log.Warnf("load map remote error: %v", err)
		} else {
			p.AddAddon(mapRemote)
		}
	}

	if config.MapLocal != "" {
		mapLocal, err := addon.NewMapLocalFromFile(config.MapLocal)
		if err != nil {
			log.Warnf("load map local error: %v", err)
		} else {
			p.AddAddon(mapLocal)
		}
	}

	if config.Dump != "" {
		dumper := addon.NewDumperWithFilename(config.Dump, config.DumpLevel)
		p.AddAddon(dumper)
	}

	log.Fatal(p.Start())
}

func matchHost(address string, hosts []string) bool {
	hostname, port := splitHostPort(address)
	for _, host := range hosts {
		h, p := splitHostPort(host)
		if matchHostname(hostname, h) && (p == "" || p == port) {
			return true
		}
	}
	return false
}

func matchHostname(hostname string, h string) bool {
	if h == "*" {
		return true
	}
	if strings.HasPrefix(h, "*.") {
		return hostname == h[2:] || strings.HasSuffix(hostname, h[1:])
	}
	return h == hostname
}

func splitHostPort(address string) (string, string) {
	index := strings.LastIndex(address, ":")
	if index == -1 {
		return address, ""
	}
	return address[:index], address[index+1:]
}
