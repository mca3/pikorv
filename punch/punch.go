// This package implements the pikopunch protocol, as described at
// https://github.com/mca3/pikopunch.
package punch

import (
	"context"
	"log"
	"net"
	"sync"
)

// LookupFunc provides a UDP address which may be used to lookup the source
// address from a different source, such as if you were to use it over
// WireGuard.
type LookupFunc func(ctx context.Context, addr *net.UDPAddr) (string, error)

// Server implements the pikopunch server.
type Server struct {
	// Lookup defines the lookup function and may be nil, in which case it
	// will fall back to returning the string representation of the source
	// address.
	//
	// Lookup must not become nil once Server handles its first request.
	Lookup        LookupFunc
	lookupSetOnce sync.Once
}

// Listen is the read loop for the pikopunch server.
//
// Listen waits until a packet is read or the connection is closed.
// All packets have their address looked up with s.Lookup which is in turn sent
// back to the client.
func (s *Server) Listen(c *net.UDPConn) error {
	c.SetWriteBuffer(64)

	s.lookupSetOnce.Do(func() {
		if s.Lookup != nil {
			return
		}

		s.Lookup = defaultLookup
	})

	for {
		// TODO: The server isn't particularly fast because it is,
		// well, single-threaded, but I think for something of this
		// simplicity a multi-threaded server isn't very important.
		//
		// Responses don't have to be instantaneous, they just have to
		// arrive eventually, within about five seconds ideally.

		_, addr, err := c.ReadFromUDP([]byte(nil))
		if err != nil {
			return err
		}

		ret, err := s.Lookup(context.TODO(), addr)
		if err != nil || len(ret) > 64 {
			// Ignore the packet
			log.Printf("punch: lookup failed for %v: %v", addr, err)
			continue
		}
		ret += "\x00"

		c.WriteToUDP([]byte(ret), addr)
	}

	// Unreachable.
}

func defaultLookup(_ context.Context, addr *net.UDPAddr) (string, error) {
	return addr.String(), nil
}
