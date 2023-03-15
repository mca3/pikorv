package routes

import (
	"net"
	"testing"

	"github.com/mca3/pikorv/config"
)

func TestApplySubnet(t *testing.T) {
	// 2001:db8::/32
	// TODO: Maybe we shouldn't do this?
	config.SubnetIp = &net.IPNet{
		IP:   net.IP{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		Mask: net.IPMask{0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}

	ip := net.IP{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	applySubnet(ip)

	exp := net.IP{0x20, 0x01, 0x0d, 0xb8, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

	if !ip.Equal(exp) {
		t.Fatalf("expected %s, got %s", exp.String(), ip.String())
	}
}
