package main

import (
	"flag"
	"fmt"

	// slog "log"

	"os"

	"github.com/lqqyt2423/go-mitmproxy/addon"
	"github.com/lqqyt2423/go-mitmproxy/addon/flowmapper"
	"github.com/lqqyt2423/go-mitmproxy/addon/web"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	version  bool
	certPath string

	addr         string
	webAddr      string
	ssl_insecure bool

	dump      string // dump filename
	dumpLevel int    // dump level

	mapperDir string
}

func loadConfig() *Config {
	config := new(Config)

	flag.BoolVar(&config.version, "version", false, "show version")
	flag.StringVar(&config.addr, "addr", ":9080", "proxy listen addr")
	flag.StringVar(&config.webAddr, "web_addr", ":9081", "web interface listen addr")
	flag.BoolVar(&config.ssl_insecure, "ssl_insecure", false, "not verify upstream server SSL/TLS certificates.")
	flag.StringVar(&config.dump, "dump", "", "dump filename")
	flag.IntVar(&config.dumpLevel, "dump_level", 0, "dump level: 0 - header, 1 - header + body")
	flag.StringVar(&config.mapperDir, "mapper_dir", "", "mapper files dirpath")
	flag.StringVar(&config.certPath, "cert_path", "", "path of generate cert files")
	flag.Parse()

	return config
}

func main() {
	config := loadConfig()

	// for debug
	// slog.SetFlags(slog.LstdFlags | slog.Lshortfile)
	// log.SetReportCaller(true)

	log.SetLevel(log.InfoLevel)
	log.SetReportCaller(false)
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	opts := &proxy.Options{
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

	p.AddAddon(&addon.Log{})
	p.AddAddon(web.NewWebAddon(config.webAddr))

	if config.dump != "" {
		dumper := addon.NewDumper(config.dump, config.dumpLevel)
		p.AddAddon(dumper)
	}

	if config.mapperDir != "" {
		mapper := flowmapper.NewMapper(config.mapperDir)
		p.AddAddon(mapper)
	}

	log.Fatal(p.Start())
}
