package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"

	"github.com/mca3/mwr"
)

var (
	cfgPath = flag.String("conf", "./rendezvous.json", "path to configuration file")
)

var srv *http.Server
var srvh *mwr.Handler

func startHttp() {
	srvh = &mwr.Handler{}
	srv = &http.Server{
		Addr:    httpAddr,
		Handler: srvh,
	}

	srvh.Use(func(c *mwr.Ctx) error {
		defer func() {
			if v := recover(); v != nil {
				log.Println(v)
				log.Println(string(debug.Stack()))
			}
		}()

		return c.Next()
	})

	// New routes
	srvh.Post("/api/new/user", apiNewUser)
	srvh.Post("/api/new/device", apiNewDevice)

	// User stuff
	srvh.Get("/api/list/devices", apiListDevices)

	// Delete routes
	srvh.Post("/api/del/user", apiDeleteUser)
	srvh.Post("/api/del/device", apiDeleteDevice)

	// Auth stuff
	srvh.Get("/api/auth", apiAuth)
	srvh.Post("/api/auth", apiAuth)

	log.Fatal(srv.ListenAndServe())
}

func stopHttp() {
	srv.Shutdown(context.Background())
}

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

	startHttp()
	defer stopHttp()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}
