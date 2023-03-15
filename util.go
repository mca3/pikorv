package main

import (
	"crypto/rand"
	"net"
)

func getBit(b []byte, i int) bool {
	index := int(i / 32)
	if index > len(b) {
		return false
	}

	byt := b[index]
	bit := i % 32
	return byt&(1<<bit) != 0
}

func setBit(b []byte, i int, val bool) {
	index := int(i / 32)
	if index > len(b) {
		return
	}

	byt := &b[index]
	bit := i % 32

	if val { // Set bit
		*byt = *byt | (1 << bit)
	} else { // Clear bit
		*byt = *byt & ^(1 << bit)
	}
}

func applySubnet(ip net.IP) {
	// Hacky bit manipulation follows
	_, bits := subnetIp.Mask.Size()

	for i := 0; i < bits; i++ {
		setBit(ip, i, getBit(subnetIp.IP, i))
	}
}

func genIPv6() string {
	newIP := net.IP(make([]byte, 16))
	rand.Read(newIP)

	applySubnet(newIP)
	return newIP.String()
}
