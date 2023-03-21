package config

import (
	"encoding/json"
	"net"
	"os"
)

var (
	ConfPath        = "./rendezvous.json"
	DatabaseUrl     = ""
	HttpAddr        = ":8080"
	Subnet          = "fd00::/32"
	SubnetIp        *net.IPNet
	JWTSecret       = ""
	OurIP           = ""
	PunchIP         = "fd00::"
	PunchIface      = "pp0"
	PunchPort       = 18732
	PunchPrivateKey = ""
	PunchPublicKey  = ""
)

func Load() error {
	cfg := struct {
		Dburl           string `json:"database"`
		Http            string `json:"http"`
		Subnet          string `json:"subnet"`
		Jwt             string `json:"jwt_secret"`
		Punch           string `json:"punch_interface"`
		PunchIP         string `json:"punch_ip"`
		PunchPort       int    `json:"punch_port"`
		PunchPrivateKey string `json:"punch_private_key"`
		PunchPublicKey  string `json:"punch_public_key"`
		OurIP           string `json:"our_ip"`
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
	if cfg.Punch != "" {
		PunchIface = cfg.Punch
	}
	if cfg.PunchIP != "" {
		PunchIP = cfg.PunchIP
	}
	if cfg.PunchPort != 0 {
		PunchPort = cfg.PunchPort
	}
	if cfg.PunchPrivateKey == "" {
		panic("punch_private_key is empty")
	}
	PunchPrivateKey = cfg.PunchPrivateKey
	if cfg.PunchPublicKey == "" {
		panic("punch_public_key is empty")
	}
	PunchPublicKey = cfg.PunchPublicKey
	if cfg.OurIP == "" {
		panic("our_ip is empty")
	}
	OurIP = cfg.OurIP
	if cfg.Jwt == "" {
		panic("jwt secret is empty")
	}
	JWTSecret = cfg.Jwt

	_, SubnetIp, err = net.ParseCIDR(Subnet)
	return err
}
