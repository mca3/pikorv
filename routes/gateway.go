package routes

import (
	"net/http"
	"net/netip"

	"github.com/mca3/mwr"
	"github.com/mca3/pikorv/routes/gateway"
	"nhooyr.io/websocket"
)

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

		gateway.Accept(r.Context(), c, user, addr)
	})
}
