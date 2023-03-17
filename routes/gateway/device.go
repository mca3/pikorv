package gateway

import (
	"context"

	"github.com/mca3/pikorv/db"
)

func OnDeviceChange(dev db.Device) {
	// Figure out who we need to notify
	devs, err := dev.ConnectedTo(context.Background())
	if err != nil {
		return
	}

	msg := gatewayMsg{
		Type:   gatewayDevUpdate,
		Device: &dev,
	}

	for _, v := range devs {
		sendChan <- sendReq{
			Device: v.ID,
			Msg:    msg,
		}
	}

	// Send it to the device itself
	sendChan <- sendReq{
		Device: dev.ID,
		Msg:    msg,
	}
}
