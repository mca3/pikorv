package routes

import (
	"context"
	"log"
	"net/http"
	"net/netip"
	"sync"
	"time"

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
		if msg.DeviceID == 0 || msg.Port < 1024 || msg.Port > 65535 {
			return
		}

		dev, err := db.DeviceID(ctx, msg.DeviceID)
		if err != nil || dev.Owner != gc.u.ID {
			return
		}

		dev.Endpoint = netip.AddrPortFrom(gc.ip, uint16(msg.Port)).String()
		if err := dev.Save(ctx); err != nil {
			return
		}
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

		ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
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
