package main

import (
	"flag"
	"os"

	"github.com/lqqyt2423/go-mitmproxy/addon"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	addr string
	dump string // dump filename
}

func loadConfig() *Config {
	config := new(Config)

	flag.StringVar(&config.addr, "addr", ":9080", "proxy listen addr")
	flag.StringVar(&config.dump, "dump", "", "dump filename")
	flag.Parse()

	return config
}

func main() {
	log.SetLevel(log.InfoLevel)
	log.SetReportCaller(false)
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	config := loadConfig()

	opts := &proxy.Options{
		Addr:              config.addr,
		StreamLargeBodies: 1024 * 1024 * 5,
	}

	p, err := proxy.NewProxy(opts)
	if err != nil {
		log.Fatal(err)
	}

	if config.dump != "" {
		dumper := addon.NewDumperWithFile(config.dump)
		p.AddAddon(dumper)
	}

	log.Fatal(p.Start())
}
