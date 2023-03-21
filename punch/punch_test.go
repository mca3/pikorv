package punch

import (
	"net"
	"testing"
	"time"
)

func resolveUDP(nw, addr string) *net.UDPAddr {
	a, err := net.ResolveUDPAddr(nw, addr)
	if err != nil {
		panic(err)
	}
	return a
}

func TestPunch4(t *testing.T) {
	srvc, err := net.ListenUDP("udp4", resolveUDP("udp4", "127.0.0.1:0"))
	if err != nil {
		t.Fatalf("server connection listen failed: %v", err)
	}
	defer srvc.Close()

	srv := Server{}

	// exits when srvc closes
	go srv.Listen(srvc)

	cli, err := net.DialUDP("udp4", resolveUDP("udp4", "127.0.0.1:0"), srvc.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("client connection open failed: %v", err)
	}
	defer cli.Close()

	t.Logf("server on %v", srvc.LocalAddr())
	t.Logf("client on %v", cli.LocalAddr())

	// Reasonable time
	cli.SetReadDeadline(time.Now().Add(time.Second))
	cli.SetWriteDeadline(time.Now().Add(time.Second))
	srvc.SetReadDeadline(time.Now().Add(time.Second))
	srvc.SetWriteDeadline(time.Now().Add(time.Second))

	// Send a zero byte packet
	cli.Write([]byte(nil))

	buf := make([]byte, 64)
	n := 0
	if n, err = cli.Read(buf); err != nil {
		t.Fatalf("failed to read from the server: %v", err)
	}
	buf = buf[:n-1]

	t.Logf("received %d bytes, final buf is %s", n, string(buf))

	if cli.LocalAddr().String() != string(buf) {
		t.Fatalf("expected %v, got %v", cli.LocalAddr().String(), string(buf))
	}
}

func TestPunch6(t *testing.T) {
	srvc, err := net.ListenUDP("udp6", resolveUDP("udp6", "[::1]:0"))
	if err != nil {
		t.Fatalf("server connection listen failed: %v", err)
	}
	defer srvc.Close()

	srv := Server{}

	// exits when srvc closes
	go srv.Listen(srvc)

	cli, err := net.DialUDP("udp6", resolveUDP("udp6", "[::1]:0"), srvc.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("client connection open failed: %v", err)
	}
	defer cli.Close()

	t.Logf("server on %v", srvc.LocalAddr())
	t.Logf("client on %v", cli.LocalAddr())

	// Reasonable time
	cli.SetReadDeadline(time.Now().Add(time.Second))
	cli.SetWriteDeadline(time.Now().Add(time.Second))
	srvc.SetReadDeadline(time.Now().Add(time.Second))
	srvc.SetWriteDeadline(time.Now().Add(time.Second))

	// Send a zero byte packet
	cli.Write([]byte(nil))

	buf := make([]byte, 64)
	n := 0
	if n, err = cli.Read(buf); err != nil {
		t.Fatalf("failed to read from the server: %v", err)
	}
	buf = buf[:n-1]

	t.Logf("received %d bytes, final buf is %s", n, string(buf))

	if cli.LocalAddr().String() != string(buf) {
		t.Fatalf("expected %v, got %v", cli.LocalAddr().String(), string(buf))
	}
}

func BenchmarkPunch(b *testing.B) {
	srvc, err := net.ListenUDP("udp6", resolveUDP("udp6", "[::1]:0"))
	if err != nil {
		panic(err)
	}
	defer srvc.Close()

	srv := Server{}

	// exits when srvc closes
	go srv.Listen(srvc)

	cli, err := net.DialUDP("udp6", resolveUDP("udp6", "[::1]:0"), srvc.LocalAddr().(*net.UDPAddr))
	if err != nil {
		panic(err)
	}
	defer cli.Close()

	buf := make([]byte, 64)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cli.Write([]byte(nil))
		cli.Read(buf)
	}
}
