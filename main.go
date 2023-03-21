package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/mca3/mwr"
	"github.com/mca3/pikorv/config"
	"github.com/mca3/pikorv/db"
	"github.com/mca3/pikorv/internal/ppwg"
	"github.com/mca3/pikorv/routes"
	"github.com/mca3/pikorv/routes/gateway"
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
			log.Printf("%s %s ERROR %v", c.IP(), c.Path(), err)
		} else {
			log.Printf("%s %s", c.IP(), c.Path())
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
	srvh.Get("/api/device/info", routes.DeviceInfo)
	srvh.Post("/api/device/join", routes.DeviceJoin)
	srvh.Post("/api/device/leave", routes.DeviceLeave)

	// Network stuff
	srvh.Get("/api/network/info", routes.NetworkInfo)

	// Auth stuff
	srvh.Get("/api/auth", routes.Auth)
	srvh.Post("/api/auth", routes.Auth)

	// Misc
	srvh.Get("/api/gateway", routes.Gateway)
	srvh.Get("/api/punch", routes.Punch)

	log.Fatal(srv.ListenAndServe())
}

func stopHttp() {
	srv.Shutdown(context.Background())
}

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config.ConfPath = *cfgPath

	if err := config.Load(); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if err := db.Connect(config.DatabaseUrl); err != nil {
		log.Fatalf("failed to connect to the database: %v", err)
	}

	defer db.Disconnect()

	go func() {
		if err := ppwg.Listen(ctx); err != nil {
			log.Fatalf("pikopunch failed to listen: %v", err)
			cancel()
		}
	}()

	// The gateway has a couple of workers that send out WebSocket
	// messages, to prevent spawning many goroutines.
	gateway.InitWorkers(runtime.GOMAXPROCS(0), 1<<12) // 4096

	go startHttp()
	defer stopHttp()

	log.Println("Running.")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	select {
	case <-ctx.Done():
	case <-c:
	}

	log.Println("Exiting.")

	cancel()

	// Let all gateway workers finish up what they need to do.
	gateway.JoinWorkers()

	// TODO: This is really terrible!
	// We're waiting for WireGuard to finish up.
	time.Sleep(time.Second)
}
