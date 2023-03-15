package config

import (
	"encoding/json"
	"net"
	"os"
)

var (
	ConfPath    = "./rendezvous.json"
	DatabaseUrl = ""
	HttpAddr    = ":8080"
	Subnet      = "fd00::/32"
	SubnetIp    *net.IPNet
)

func Load() error {
	cfg := struct {
		Dburl  string `json:"database"`
		Http   string `json:"http"`
		Subnet string `json:"subnet"`
	}{}

	f, err := os.Open(ConfPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return err
	}

	if cfg.Dburl != "" {
		DatabaseUrl = cfg.Dburl
	}
	if cfg.Http != "" {
		HttpAddr = cfg.Http
	}
	if cfg.Subnet != "" {
		Subnet = cfg.Subnet
	}

	_, SubnetIp, err = net.ParseCIDR(Subnet)
	return err
}
