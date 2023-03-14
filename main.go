package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
)

var (
	cfgPath = flag.String("conf", "./rendezvous.json", "path to configuration file")
)

func main() {
	flag.Parse()

	confPath = *cfgPath

	if err := loadConfig(); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if err := connect(); err != nil {
		log.Fatalf("failed to connect to the database: %v", err)
	}

	defer disconnect()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}
