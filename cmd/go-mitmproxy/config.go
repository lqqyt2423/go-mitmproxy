package main

import (
	"flag"
	"fmt"
)

func loadConfig() *Config {
	config := new(Config)

	flag.IntVar(&config.debug, "debug", 0, "debug mode: 1 - print debug log, 2 - show debug from")
	flag.BoolVar(&config.version, "version", false, "show version")
	flag.StringVar(&config.addr, "addr", ":9080", "proxy listen addr")
	flag.StringVar(&config.webAddr, "web_addr", ":9081", "web interface listen addr")
	flag.BoolVar(&config.ssl_insecure, "ssl_insecure", false, "not verify upstream server SSL/TLS certificates.")
	flag.StringVar(&config.dump, "dump", "", "dump filename")
	flag.IntVar(&config.dumpLevel, "dump_level", 0, "dump level: 0 - header, 1 - header + body")
	flag.StringVar(&config.mapperDir, "mapper_dir", "", "mapper files dirpath")
	flag.StringVar(&config.certPath, "cert_path", "", "path of generate cert files")
	flag.Var((*arrayValue)(&config.ignoreHosts), "ignore_hosts", "a list of ignore hosts")
	flag.Var((*arrayValue)(&config.allowHosts), "allow_hosts", "a list of allow hosts")
	flag.Parse()

	return config
}

// arrayValue 实现了 flag.Value 接口
type arrayValue []string

func (a *arrayValue) String() string {
	return fmt.Sprint(*a)
}

func (a *arrayValue) Set(value string) error {
	*a = append(*a, value)
	return nil
}
