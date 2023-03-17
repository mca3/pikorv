package gateway

import (
	"context"

	"github.com/mca3/pikorv/db"
)

func OnNetworkJoin(dev db.Device, nw db.Network) {
	// Figure out who we need to notify
	devs, err := db.NetworkDevices(context.Background(), nw.ID)
	if err != nil {
		return
	}

	msg := gatewayMsg{
		Type:    gatewayNetworkJoin,
		Device:  &dev,
		Network: &nw,
	}

	for _, v := range devs {
		if v.ID == dev.ID {
			continue
		}

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

func OnNetworkLeave(dev db.Device, nw db.Network) {
	// Figure out who we need to notify
	devs, err := db.NetworkDevices(context.Background(), nw.ID)
	if err != nil {
		return
	}

	msg := gatewayMsg{
		Type:    gatewayNetworkLeave,
		Device:  &dev,
		Network: &nw,
	}

	for _, v := range devs {
		if v.ID == dev.ID {
			continue
		}

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
