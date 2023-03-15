package routes

// This package holds all API routes.

import (
	"github.com/mca3/mwr"
	"github.com/mca3/pikorv/db"
)

// NewDevice creates a new device and attaches it to the user's account.
//
// Path: /api/new/device
// Method: POST
// Authenticated.
// Body: JSON. Specify "name" and "key", where "key" is a WireGuard public key.
func NewDevice(c *mwr.Ctx) error {
	user, ok := isAuthed(c)
	if !ok {
		return api403(c, errNoAuth)
	}

	data := struct {
		Name, Key string
	}{}

	if err := c.BodyParser(&data); err != nil {
		return api400(c, err)
	} else if data.Name == "" || data.Key == "" {
		return api400(c)
	}

	dev := db.Device{
		Name:      data.Name,
		Owner:     user.ID,
		PublicKey: data.Key,
		IP:        genIPv6(),
	}
	if err := dev.Save(c.Context()); err != nil {
		return api500(c, err)
	}

	return sendJSON(c, dev)
}

// ListDevices lists the devices on the user's account.
//
// Path: /api/list/devices
// Method: GET
// Authenticated.
func ListDevices(c *mwr.Ctx) error {
	user, ok := isAuthed(c)
	if !ok {
		return api403(c, errNoAuth)
	}

	devs, err := db.Devices(c.Context(), user.ID)
	if err != nil {
		return api500(c, err)
	}

	out := make([]struct {
		db.Device
		Networks []db.Network `json:"networks"`
	}, len(devs))

	for k, v := range devs {
		out[k].Device = v
		nws, err := db.DeviceNetworks(c.Context(), v.ID)
		if err != nil {
			return api500(c, err)
		}
		out[k].Networks = nws
	}

	return sendJSON(c, out)
}

// DeleteDevice deletes a device from the user's account.
//
// Path: /api/del/device
// Method: POST
// Authenticated.
// Body: JSON. Specify "id".
func DeleteDevice(c *mwr.Ctx) error {
	user, ok := isAuthed(c)
	if !ok {
		return api403(c, errNoAuth)
	}

	data := struct {
		ID int64
	}{}

	if err := c.BodyParser(&data); err != nil {
		return api400(c, err)
	} else if data.ID == 0 {
		return api400(c)
	}

	dev, err := db.DeviceID(c.Context(), data.ID)
	if err != nil {
		return api500(c, err)
	}

	if dev.Owner != user.ID {
		return api404(c)
	}

	if err := dev.Delete(c.Context()); err != nil {
		return api500(c, err)
	}

	// TODO: Notify networks

	return c.SendStatus(204)
}

// DevicePing updates the device's IP.
// TODO: Not implemented.
//
// Path: /api/device/ping
// Method: POST
// Authenticated.
// Body: JSON. Specify "device".
func DevicePing(c *mwr.Ctx) error {
	user, ok := isAuthed(c)
	if !ok {
		return api403(c, errNoAuth)
	}

	data := struct {
		Device int64
	}{}

	if err := c.BodyParser(&data); err != nil {
		return api400(c, err)
	} else if data.Device == 0 {
		return api400(c)
	}

	dev, err := db.DeviceID(c.Context(), data.Device)
	if err != nil {
		return api500(c, err)
	}

	if dev.Owner != user.ID {
		return api404(c)
	}

	// TODO: Functionality goes here
	// TODO: Notify others on network

	return c.SendStatus(204)
}

// DeviceJoin joins a device to a network.
//
// Path: /api/device/join
// Method: POST
// Authenticated.
// Body: JSON. Specify "device" and "network".
func DeviceJoin(c *mwr.Ctx) error {
	user, ok := isAuthed(c)
	if !ok {
		return api403(c, errNoAuth)
	}

	data := struct {
		Device, Network int64
	}{}

	if err := c.BodyParser(&data); err != nil {
		return api400(c, err)
	} else if data.Device == 0 || data.Network == 0 {
		return api400(c)
	}

	dev, err := db.DeviceID(c.Context(), data.Device)
	if err != nil {
		return api500(c, err)
	}

	if dev.Owner != user.ID {
		return api404(c)
	}

	nw, err := db.NetworkID(c.Context(), data.Network)
	if err != nil {
		return api500(c, err)
	}

	if nw.Owner != user.ID {
		return api404(c)
	}

	if err := nw.Add(c.Context(), dev.ID); err != nil {
		return api500(c, err)
	}

	// TODO: Notify others on network

	return c.SendStatus(204)
}

// DeviceLeave removes a device from a network.
//
// Path: /api/device/leave
// Method: POST
// Authenticated.
// Body: JSON. Specify "device" and "network".
func DeviceLeave(c *mwr.Ctx) error {
	user, ok := isAuthed(c)
	if !ok {
		return api403(c, errNoAuth)
	}

	data := struct {
		Device, Network int64
	}{}

	if err := c.BodyParser(&data); err != nil {
		return api400(c, err)
	} else if data.Device == 0 || data.Network == 0 {
		return api400(c)
	}

	dev, err := db.DeviceID(c.Context(), data.Device)
	if err != nil {
		return api500(c, err)
	}

	if dev.Owner != user.ID {
		return api404(c)
	}

	nw, err := db.NetworkID(c.Context(), data.Network)
	if err != nil {
		return api500(c, err)
	}

	if nw.Owner != user.ID {
		return api404(c)
	}

	if err := nw.Remove(c.Context(), dev.ID); err != nil {
		return api500(c, err)
	}

	// TODO: Notify others on network

	return c.SendStatus(204)
}
