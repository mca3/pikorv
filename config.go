package main

import (
	"encoding/json"
	"net"
	"os"
)

var (
	confPath    = "./rendezvous.json"
	databaseUrl = ""
	httpAddr    = ":8080"
	subnet      = "fd00::/32"
	subnetIp    *net.IPNet
)

func loadConfig() error {
	cfg := struct {
		Dburl  string `json:"database"`
		Http   string `json:"http"`
		Subnet string `json:"subnet"`
	}{}

	f, err := os.Open(confPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return err
	}

	if cfg.Dburl != "" {
		databaseUrl = cfg.Dburl
	}
	if cfg.Http != "" {
		httpAddr = cfg.Http
	}
	if cfg.Subnet != "" {
		subnet = cfg.Subnet
	}

	_, subnetIp, err = net.ParseCIDR(subnet)
	return err
}
