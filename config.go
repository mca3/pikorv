package main

import (
	"encoding/json"
	"os"
)

var (
	confPath    = "./rendezvous.json"
	databaseUrl = ""
	httpAddr    = ":8080"
)

func loadConfig() error {
	cfg := struct {
		Dburl string `json:"database"`
		Http  string `json:"http"`
	}{}

	f, err := os.Open(confPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return err
	}

	databaseUrl = cfg.Dburl
	httpAddr = cfg.Http
	return nil
}
