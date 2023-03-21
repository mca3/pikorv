// Most of the code here is lifted from pikonoded.

package ppwg

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"

	"github.com/mca3/pikorv/config"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var C = make(chan wgPeer, 1000)
var wg *wgctrl.Client

type wgPeer struct {
	Key    wgtypes.Key
	IP     string
	Remove bool
}

// nlWireguard implements netlink.Link, as there is no native way to do this in
// the netlink package yet.
type nlWireguard struct {
	netlink.LinkAttrs
}

func (w *nlWireguard) Attrs() *netlink.LinkAttrs {
	return &w.LinkAttrs
}

func (w *nlWireguard) Type() string {
	return "wireguard"
}

func (m *wgPeer) IPNet() *net.IPNet {
	_, ipn, err := net.ParseCIDR(m.IP + "/128")
	if err != nil {
		return nil
	}
	return ipn
}

// parseKey converts a base64 key into a WireGuard key.
func parseKey(key string) (wgtypes.Key, error) {
	dst := [32]byte{}

	if _, err := base64.StdEncoding.Decode(dst[:], []byte(key)); err != nil {
		return wgtypes.Key(dst), err
	}

	return wgtypes.Key(dst), nil
}

func handleWgMsg(link netlink.Link, wg *wgctrl.Client, pcfg wgPeer) {
	peer := wgtypes.PeerConfig{
		PublicKey: pcfg.Key,
		Remove:    pcfg.Remove,
	}

	ipn := pcfg.IPNet() // Never nil

	if pcfg.Remove {
		log.Printf("pikopunch: removing %s as WireGuard peer", pcfg.IP)
		rmRoute(link, ipn)
	} else {
		peer.AllowedIPs = []net.IPNet{*ipn}

		log.Printf("pikopunch: adding %s as WireGuard peer", pcfg.IP)
		addRoute(link, ipn)
	}

	wg.ConfigureDevice(link.Attrs().Name, wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peer},
	})
}

func goWireguard(ctx context.Context, link netlink.Link, wg *wgctrl.Client) {
	defer netlink.LinkDel(link)
	defer wg.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case e := <-C:
			handleWgMsg(link, wg, e)
		}
	}
}

func createWireguard(ctx context.Context) error {
	// Parse our IP
	ip, ipn, err := net.ParseCIDR(config.PunchIP + "/128")
	if err != nil {
		return err
	}
	ipn.IP = ip

	// Create the interface
	attrs := netlink.NewLinkAttrs()
	attrs.Name = config.PunchIface
	wga := nlWireguard{attrs}

	err = netlink.LinkAdd(&wga)
	if err != nil {
		return err
	}

	l, err := netlink.LinkByName(attrs.Name)
	if err != nil {
		// This shouldn't fail...
		log.Fatalf("pikopunch: cannot access the interface (%s) we just made: %v", attrs.Name, err)
	}

	// Unfortunately since this spawns a goroutine to handle messages being
	// passed to it since I'm not entirely sure if wgctrl-go will like
	// multiple goroutines using it at once, we cannot defer and must
	// instead cleanup whenever we would exit in a bad way.

	// Configure WireGuard
	wg, err = wgctrl.New()
	if err != nil {
		netlink.LinkDel(l)
		return err
	}

	wgkey, err := parseKey(config.PunchPrivateKey)
	if err != nil {
		wg.Close()
		netlink.LinkDel(l)
		return fmt.Errorf("couldn't parse private key: %w", err)
	}

	if err := wg.ConfigureDevice(attrs.Name, wgtypes.Config{
		PrivateKey: &wgkey,
		ListenPort: &config.PunchPort,
	}); err != nil {
		wg.Close()
		netlink.LinkDel(l)
		return err
	}

	// Tell the kernel where we live
	if err := netlink.AddrAdd(l, &netlink.Addr{IPNet: ipn}); err != nil {
		wg.Close()
		netlink.LinkDel(l)
		return err
	}

	// Set our IP
	if err := netlink.LinkSetUp(l); err != nil {
		wg.Close()
		netlink.LinkDel(l)
		return err
	}

	go goWireguard(ctx, l, wg)

	return nil
}

func addRoute(link netlink.Link, addr *net.IPNet) {
	r := netlink.Route{
		LinkIndex: link.Attrs().Index,
		Protocol:  6,
		Dst:       addr,
	}

	if err := netlink.RouteAdd(&r); err != nil {
		log.Printf("failed to add route for %s: %v", addr.IP, err)
	}
}

func rmRoute(link netlink.Link, addr *net.IPNet) {
	routes, err := netlink.RouteGet(addr.IP)
	if err != nil {
		return
	}

	for _, v := range routes {
		if v.LinkIndex == link.Attrs().Index && v.Dst.IP.Equal(addr.IP) {
			if err := netlink.RouteDel(&v); err != nil {
				log.Printf("failed to delete route for %s: %v", v.Dst, err)
			}

			break
		}
	}
}
