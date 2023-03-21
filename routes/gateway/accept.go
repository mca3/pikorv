package gateway

import (
	"context"
	"log"
	"net/netip"
	"sync"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"github.com/mca3/pikorv/db"
)

type gatewayMsgType int

type gatewayMsg struct {
	Type gatewayMsgType

	Device  *db.Device  `json:"device,omitempty"`
	Network *db.Network `json:"network,omitempty"`
	Remove  bool

	Endpoint  string `json:"endpoint,omitempty"`
	DeviceID  int64  `json:"device_id,omitempty"`
	NetworkID int64  `json:"network_id,omitempty"`
}

type gatewayClient struct {
	u  *db.User
	c  *websocket.Conn
	d  int64
	ip netip.Addr

	sync.Mutex
}

const (
	gatewayPing gatewayMsgType = iota
	gatewayNetworkJoin
	gatewayNetworkLeave
	gatewayDevUpdate
)

const (
	sendTimeout = time.Second * 15
	// recvTimeout = time.Minute*5
)

var (
	gatewayClients = []*gatewayClient{}
	gwcMu          = sync.RWMutex{}
)

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

func Accept(ctx context.Context, c *websocket.Conn, user *db.User, addr netip.AddrPort) {
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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		msg, err := gc.Read(ctx)
		if err != nil {
			log.Println(err)
			return
		}

		gc.handle(ctx, &msg)
	}

	c.Close(websocket.StatusNormalClosure, "")
}

func (gc *gatewayClient) handle(ctx context.Context, msg *gatewayMsg) {
	gc.Lock()
	defer gc.Unlock()

	switch msg.Type {
	case gatewayPing:
		if msg.DeviceID <= 0 || msg.Endpoint == "" {
			return
		}

		dev, err := db.DeviceID(ctx, msg.DeviceID)
		if err != nil || dev.Owner != gc.u.ID {
			return
		}

		if gc.d == 0 {
			gc.d = dev.ID
		}

		// TODO: Sanity checking
		if dev.Endpoint == msg.Endpoint {
			return
		}

		dev.Endpoint = msg.Endpoint
		if err := dev.Save(ctx); err != nil {
			return
		}

		OnDeviceChange(dev)
	}
}

// Send queues msg to be sent.
func (gc *gatewayClient) Send(msg gatewayMsg) {
	sendChan <- sendReq{
		Device: gc.d,
		Msg:    msg,
	}
}

// send immediately sends a message to the client instead of queuing it to be
// sent.
// You shouldn't use this method unless you're working with a worker.
func (gc *gatewayClient) send(ctx context.Context, msg gatewayMsg) error {
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	return wsjson.Write(ctx, gc.c, msg)
}

// Read reads a message from the client.
func (gc *gatewayClient) Read(ctx context.Context) (gatewayMsg, error) {
	// ctx, cancel := context.WithTimeout(ctx, recvTimeout)
	// defer cancel()

	msg := gatewayMsg{}
	err := wsjson.Read(ctx, gc.c, &msg)
	return msg, err
}
