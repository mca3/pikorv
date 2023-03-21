package ppwg

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/mca3/pikorv/config"
	"github.com/mca3/pikorv/db"
	"github.com/mca3/pikorv/punch"
)

var errNotFound = errors.New("not found")

func punchLookup(_ context.Context, addr *net.UDPAddr) (string, error) {
	ipstr := addr.IP.String() + "/128"
	ip, _, err := net.ParseCIDR(ipstr)
	if err != nil {
		return "", err
	}

	dev, err := wg.Device(config.PunchIface)
	if err != nil {
		return "", err
	}

	for _, p := range dev.Peers {
		for _, i := range p.AllowedIPs {
			if i.IP.Equal(ip) {
				return p.Endpoint.String(), nil
			}
		}
	}

	return "", errNotFound
}

func configurePeers(ctx context.Context) error {
	devs, err := db.AllDevices(ctx)
	if err != nil {
		return err
	}

	for _, dev := range devs {
		k, err := parseKey(dev.PublicKey)
		if err != nil {
			// shouldn't happen
			continue
		}

		C <- wgPeer{
			IP:  dev.IP,
			Key: k,
		}
	}

	return nil
}

func Listen(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := createWireguard(ctx); err != nil {
		return err
	}

	if err := configurePeers(ctx); err != nil {
		return err
	}

	s := punch.Server{
		Lookup: punchLookup,
	}

	addr, _ := net.ResolveUDPAddr("udp6", fmt.Sprintf("[%s]:8743", config.PunchIP))
	srv, err := net.ListenUDP("udp6", addr)
	if err != nil {
		return err
	}
	defer srv.Close()

	return s.Listen(srv)
}
