package routes

// This package holds all API routes.

import (
	"github.com/mca3/mwr"
	"github.com/mca3/pikorv/db"
)

// apiNewDevice creates a new device and attaches it to the user's account.
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

// apiListDevices lists the devices on the user's account.
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

	return sendJSON(c, devs)
}

// apiDeleteDevice deletes a device from the user's account.
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

	return c.SendStatus(204)
}
