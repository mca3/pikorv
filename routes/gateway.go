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

// notifyDeviceChange notifies all devices that know dev about changes to dev.
func notifyDeviceChange(dev db.Device) {
	gwcMu.RLock()
	defer gwcMu.RUnlock()

	devs, err := dev.ConnectedTo(context.Background())
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
		if d.ID == dev.ID {
			continue
		}

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

	devs, err := dev.AffectedByLeave(context.Background(), nw)
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
