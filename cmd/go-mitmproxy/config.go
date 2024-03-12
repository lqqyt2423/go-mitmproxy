package main

import (
	"flag"
	"fmt"

	"github.com/lqqyt2423/go-mitmproxy/internal/helper"
	log "github.com/sirupsen/logrus"
)

func loadConfigFromFile(filename string) (*Config, error) {
	var config Config
	if err := helper.NewStructFromFile(filename, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func loadConfigFromCli() *Config {
	config := new(Config)

	flag.BoolVar(&config.version, "version", false, "show go-mitmproxy version")
	flag.StringVar(&config.Addr, "addr", ":9080", "proxy listen addr")
	flag.StringVar(&config.WebAddr, "web_addr", ":9081", "web interface listen addr")
	flag.BoolVar(&config.SslInsecure, "ssl_insecure", false, "not verify upstream server SSL/TLS certificates.")
	flag.Var((*arrayValue)(&config.IgnoreHosts), "ignore_hosts", "a list of ignore hosts")
	flag.Var((*arrayValue)(&config.AllowHosts), "allow_hosts", "a list of allow hosts")
	flag.StringVar(&config.CertPath, "cert_path", "", "path of generate cert files")
	flag.IntVar(&config.Debug, "debug", 0, "debug mode: 1 - print debug log, 2 - show debug from")
	flag.StringVar(&config.Dump, "dump", "", "dump filename")
	flag.IntVar(&config.DumpLevel, "dump_level", 0, "dump level: 0 - header, 1 - header + body")
	flag.StringVar(&config.Upstream, "upstream", "", "upstream proxy")
	flag.BoolVar(&config.UpstreamCert, "upstream_cert", true, "connect to upstream server to look up certificate details")
	flag.StringVar(&config.MapRemote, "map_remote", "", "map remote config filename")
	flag.StringVar(&config.MapLocal, "map_local", "", "map local config filename")
	flag.StringVar(&config.filename, "f", "", "read config from the filename")
	flag.Parse()

	return config
}

func mergeConfigs(fileConfig, cliConfig *Config) *Config {
	config := new(Config)
	*config = *fileConfig
	if cliConfig.Addr != "" {
		config.Addr = cliConfig.Addr
	}
	if cliConfig.WebAddr != "" {
		config.WebAddr = cliConfig.WebAddr
	}
	if cliConfig.SslInsecure {
		config.SslInsecure = cliConfig.SslInsecure
	}
	if len(cliConfig.IgnoreHosts) > 0 {
		config.IgnoreHosts = cliConfig.IgnoreHosts
	}
	if len(cliConfig.AllowHosts) > 0 {
		config.AllowHosts = cliConfig.AllowHosts
	}
	if cliConfig.CertPath != "" {
		config.CertPath = cliConfig.CertPath
	}
	if cliConfig.Debug != 0 {
		config.Debug = cliConfig.Debug
	}
	if cliConfig.Dump != "" {
		config.Dump = cliConfig.Dump
	}
	if cliConfig.DumpLevel != 0 {
		config.DumpLevel = cliConfig.DumpLevel
	}
	if cliConfig.Upstream != "" {
		config.Upstream = cliConfig.Upstream
	}
	if !cliConfig.UpstreamCert {
		config.UpstreamCert = cliConfig.UpstreamCert
	}
	if cliConfig.MapRemote != "" {
		config.MapRemote = cliConfig.MapRemote
	}
	if cliConfig.MapLocal != "" {
		config.MapLocal = cliConfig.MapLocal
	}
	return config
}

func loadConfig() *Config {
	cliConfig := loadConfigFromCli()
	if cliConfig.version {
		return cliConfig
	}
	if cliConfig.filename == "" {
		return cliConfig
	}

	fileConfig, err := loadConfigFromFile(cliConfig.filename)
	if err != nil {
		log.Warnf("read config from %v error %v", cliConfig.filename, err)
		return cliConfig
	}
	return mergeConfigs(fileConfig, cliConfig)
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
