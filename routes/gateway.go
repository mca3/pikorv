package routes

import (
	"context"
	"log"
	"net/http"
	"net/netip"
	"sync"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"github.com/mca3/mwr"
	"github.com/mca3/pikorv/db"
)

type gatewayMsgType int

type gatewayMsg struct {
	Type gatewayMsgType

	Device  *db.Device  `json:"device,omitempty"`
	Network *db.Network `json:"network,omitempty"`
	Remove  bool

	Port      int   `json:"port,omitempty"`
	DeviceID  int64 `json:"device_id,omitempty"`
	NetworkID int64 `json:"network_id,omitempty"`
}

type gatewayClient struct {
	u  *db.User
	c  *websocket.Conn
	d  int64
	ip netip.Addr
}

const (
	gatewayPing gatewayMsgType = iota
	gatewayPeer
)

var (
	gatewayClients = []*gatewayClient{}
	gwcMu          = sync.RWMutex{}
)

func (gc *gatewayClient) handle(ctx context.Context, msg *gatewayMsg) {
	switch msg.Type {
	case gatewayPing:
		if msg.DeviceID <= 0 || msg.Port < 1024 || msg.Port > 65535 {
			return
		}

		dev, err := db.DeviceID(ctx, msg.DeviceID)
		if err != nil || dev.Owner != gc.u.ID {
			return
		}

		// Racy!!!
		if gc.d == 0 {
			gc.d = dev.ID
		}

		dev.Endpoint = netip.AddrPortFrom(gc.ip, uint16(msg.Port)).String()
		if err := dev.Save(ctx); err != nil {
			return
		}

		notifyDeviceChange(dev)
	}
}

func (gc *gatewayClient) Send(ctx context.Context, msg gatewayMsg) {
	wsjson.Write(ctx, gc.c, msg)
}

// findGatewayDevice tries to find dev as a gateway client.
// gatewayClients is assumed to be locked.
//
// If dev is not a client, nil is returned.
func findGatewayDevice(dev int64) *gatewayClient {
	for _, v := range gatewayClients {
		if v.d == dev {
			return v
		}
	}
	return nil
}

// findAffectedByDelete finds all devices that should know that a device is
// leaving the network.
func findAffectedByDelete(dev int64, leavenw int64) ([]db.Device, error) {
	// Find all devices this device knows.
	// Count how many times we see them.
	devs := map[int64]int{}
	devst := map[int64]db.Device{}

	nws, err := db.DeviceNetworks(context.Background(), dev)
	if err != nil {
		return nil, err
	}

	for _, nw := range nws {
		ndevs, err := db.NetworkDevices(context.Background(), nw.ID)
		if err != nil {
			return nil, err
		}

		for _, d := range ndevs {
			if d.ID == dev {
				continue
			}

			devst[d.ID] = d
			c, _ := devs[d.ID]

			if nw.ID == leavenw || leavenw == 0 {
				devs[d.ID] = c - 1
			} else {
				devs[d.ID] = c + 1
			}
		}
	}

	var l []db.Device

	for d, c := range devs {
		if c > 0 {
			continue
		}

		l = append(l, devst[d])
	}

	return l, nil
}

// findAffectedDevices finds all devices that should know about an update to
// dev.
//
// For devices that should know about a delete, see findAffectedByDelete.
func findAffectedDevices(dev int64) ([]db.Device, error) {
	var devs []db.Device

	nws, err := db.DeviceNetworks(context.Background(), dev)
	if err != nil {
		return nil, err
	}

	for _, nw := range nws {
		nwdevs, err := db.NetworkDevices(context.Background(), nw.ID)
		if err != nil {
			return nil, err
		}

		for _, d := range nwdevs {
			if d.ID == dev {
				continue
			}

			ok := true
			for _, v := range devs {
				if v.ID == d.ID {
					ok = false
					break
				}
			}

			if ok {
				devs = append(devs, d)
			}
		}
	}

	return devs, nil
}

// notifyDeviceChange notifies all devices that know dev about changes to dev.
func notifyDeviceChange(dev db.Device) {
	gwcMu.RLock()
	defer gwcMu.RUnlock()

	devs, err := findAffectedDevices(dev.ID)
	if err != nil {
		return
	}

	for _, d := range devs {
		gc := findGatewayDevice(d.ID)
		if gc == nil {
			continue
		}

		// TODO: I don't think this is proper.
		go gc.Send(context.Background(), gatewayMsg{
			Type:   gatewayPeer,
			Device: &dev,
		})
	}
}

// notifyDeviceJoin notifies all devices that know dev about deletes.
func notifyDeviceJoin(dev db.Device, nw int64) {
	gwcMu.RLock()
	defer gwcMu.RUnlock()

	devs, err := db.NetworkDevices(context.Background(), nw)
	if err != nil {
		return
	}

	for _, d := range devs {
		gc := findGatewayDevice(d.ID)
		if gc == nil {
			continue
		}

		// TODO: I don't think this is proper.
		go gc.Send(context.Background(), gatewayMsg{
			Type:   gatewayPeer,
			Device: &dev,
		})
	}

	// Also tell the device itself
	gc := findGatewayDevice(dev.ID)
	if gc == nil {
		return
	}

	for _, d := range devs {
		if d.ID == dev.ID {
			continue
		}

		// TODO: I don't think this is proper.
		go func(d db.Device) {
			gc.Send(context.Background(), gatewayMsg{
				Type:   gatewayPeer,
				Device: &d,
			})
		}(d)
	}
}

// notifyDeviceDelete notifies all devices that know dev about deletes.
func notifyDeviceDelete(dev db.Device, nw int64) {
	gwcMu.RLock()
	defer gwcMu.RUnlock()

	devs, err := findAffectedByDelete(dev.ID, nw)
	if err != nil {
		return
	}

	for _, d := range devs {
		gc := findGatewayDevice(d.ID)
		if gc == nil {
			continue
		}

		// TODO: I don't think this is proper.
		go gc.Send(context.Background(), gatewayMsg{
			Type:   gatewayPeer,
			Device: &dev,
			Remove: true,
		})
	}

	// Tell the device itself
	gc := findGatewayDevice(dev.ID)
	if gc == nil {
		return
	}

	for _, d := range devs {
		if d.ID == dev.ID {
			continue
		}

		// TODO: I don't think this is proper.
		go func(d db.Device) {
			gc.Send(context.Background(), gatewayMsg{
				Type:   gatewayPeer,
				Device: &d,
				Remove: true,
			})
		}(d)
	}
}

func Gateway(c *mwr.Ctx) error {
	user, ok := isAuthed(c)
	if !ok {
		return api403(c, errNoAuth)
	}

	return c.Hijack(func(w http.ResponseWriter, r *http.Request) {
		addr := netip.MustParseAddrPort(r.RemoteAddr)

		c, err := websocket.Accept(w, r, nil)
		if err != nil { // fail
			return
		}
		defer c.Close(websocket.StatusInternalError, "Websocket error")

		gc := &gatewayClient{
			u:  user,
			c:  c,
			ip: addr.Addr(),
		}

		gwcMu.Lock()
		gatewayClients = append(gatewayClients, gc)
		gwcMu.Unlock()

		// Clean up after ourselves
		defer func() {
			gwcMu.Lock()
			for i, v := range gatewayClients {
				if v == gc {
					gatewayClients[i], gatewayClients[len(gatewayClients)-1] = gatewayClients[len(gatewayClients)-1], gatewayClients[i]
					gatewayClients = gatewayClients[:len(gatewayClients)-1]
					break
				}
			}
			gwcMu.Unlock()
		}()

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		msg := gatewayMsg{}
		for {
			err = wsjson.Read(ctx, c, &msg)
			if err != nil {
				log.Println(err)
				return
			}

			gc.handle(ctx, &msg)
		}

		c.Close(websocket.StatusNormalClosure, "")
	})
}
