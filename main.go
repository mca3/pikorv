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
	"github.com/mca3/pikorv/config"
	"github.com/mca3/pikorv/db"
	"github.com/mca3/pikorv/routes"
)

var (
	cfgPath = flag.String("conf", "./rendezvous.json", "path to configuration file")
)

var srv *http.Server
var srvh *mwr.Handler

func startHttp() {
	srvh = &mwr.Handler{}
	srv = &http.Server{
		Addr:    config.HttpAddr,
		Handler: srvh,
	}

	srvh.Use(func(c *mwr.Ctx) error {
		defer func() {
			if v := recover(); v != nil {
				log.Println(v)
				log.Println(string(debug.Stack()))
			}
		}()

		err := c.Next()
		if err != nil {
			log.Printf("%s ERROR %v", c.Path(), err)
		}
		return err
	})

	// New routes
	srvh.Post("/api/new/user", routes.NewUser)
	srvh.Post("/api/new/device", routes.NewDevice)
	srvh.Post("/api/new/network", routes.NewNetwork)

	// User stuff
	srvh.Get("/api/list/devices", routes.ListDevices)
	srvh.Get("/api/list/networks", routes.ListNetworks)

	// Delete routes
	srvh.Post("/api/del/user", routes.DeleteUser)
	srvh.Post("/api/del/device", routes.DeleteDevice)
	srvh.Post("/api/del/network", routes.DeleteNetwork)

	// Device stuff
	srvh.Post("/api/device/ping", routes.DevicePing)
	srvh.Post("/api/device/join", routes.DeviceJoin)
	srvh.Post("/api/device/leave", routes.DeviceLeave)

	// Auth stuff
	srvh.Get("/api/auth", routes.Auth)
	srvh.Post("/api/auth", routes.Auth)

	log.Fatal(srv.ListenAndServe())
}

func stopHttp() {
	srv.Shutdown(context.Background())
}

func main() {
	flag.Parse()

	config.ConfPath = *cfgPath

	if err := config.Load(); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if err := db.Connect(config.DatabaseUrl); err != nil {
		log.Fatalf("failed to connect to the database: %v", err)
	}

	defer db.Disconnect()

	startHttp()
	defer stopHttp()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}
