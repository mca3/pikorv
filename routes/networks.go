package routes

// This package holds all API routes.

import (
	"github.com/mca3/mwr"
	"github.com/mca3/pikorv/db"
)

// apiNewNetwork creates a new network and attaches it to the user's account.
//
// Path: /api/new/network
// Method: POST
// Authenticated.
// Body: JSON. Specify "name".
func NewNetwork(c *mwr.Ctx) error {
	user, ok := isAuthed(c)
	if !ok {
		return api403(c, errNoAuth)
	}

	data := struct {
		Name string
	}{}

	if err := c.BodyParser(&data); err != nil {
		return api400(c, err)
	} else if data.Name == "" {
		return api400(c)
	}

	nw := db.Network{
		Name:  data.Name,
		Owner: user.ID,
	}
	if err := nw.Save(c.Context()); err != nil {
		return api500(c, err)
	}

	return sendJSON(c, nw)
}

// apiListNetworks lists the networks on the user's account.
//
// Path: /api/list/networks
// Method: GET
// Authenticated.
func ListNetworks(c *mwr.Ctx) error {
	user, ok := isAuthed(c)
	if !ok {
		return api403(c, errNoAuth)
	}

	nws, err := db.Networks(c.Context(), user.ID)
	if err != nil {
		return api500(c, err)
	}

	out := make([]struct {
		db.Network
		Devices []db.Device `json:"devices"`
	}, len(nws))

	for k, v := range nws {
		out[k].Network = v
		devs, err := db.NetworkDevices(c.Context(), v.ID)
		if err != nil {
			return api500(c, err)
		}
		out[k].Devices = devs
	}

	return sendJSON(c, out)
}

// apiDeleteNetwork deletes a network from the user's account.
//
// Path: /api/del/network
// Method: POST
// Authenticated.
// Body: JSON. Specify "id".
func DeleteNetwork(c *mwr.Ctx) error {
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

	nw, err := db.NetworkID(c.Context(), data.ID)
	if err != nil {
		return api500(c, err)
	}

	if nw.Owner != user.ID {
		return api404(c)
	}

	if err := nw.Delete(c.Context()); err != nil {
		return api500(c, err)
	}

	// TODO: Notify network devices

	return c.SendStatus(204)
}
