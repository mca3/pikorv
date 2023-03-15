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
	"github.com/mca3/pikorv/db"
	"github.com/mca3/pikorv/routes"
	"github.com/mca3/pikorv/config"
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

		return c.Next()
	})

	// New routes
	srvh.Post("/api/new/user", routes.NewUser)
	srvh.Post("/api/new/device", routes.NewDevice)

	// User stuff
	srvh.Get("/api/list/devices", routes.ListDevices)

	// Delete routes
	srvh.Post("/api/del/user", routes.DeleteUser)
	srvh.Post("/api/del/device", routes.DeleteDevice)

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
