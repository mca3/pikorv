package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v4"
)

var devCount = 1
var nwCount = 1

func openDb(t *testing.T) {
	u := os.Getenv("POSTGRES_TEST")
	if u == "" {
		t.Skip("Set POSTGRES_TEST to run database tests")
	}

	if err := Connect(u, true); err != nil {
		panic(err)
	}

	t.Cleanup(Disconnect)
}

func makeUser(t *testing.T) User {
	u := User{
		Username: "test",
		Email:    "test@example.com",
		Name:     "Test User",
	}

	if err := u.Save(context.Background()); err != nil {
		t.Fatalf("failed saving user: %v", err)
	}

	return u
}

func makeNetwork(t *testing.T, u User) Network {
	n := Network{
		Owner: u.ID,
		Name:  fmt.Sprintf("test network %d", nwCount),
	}
	nwCount++

	if err := n.Save(context.Background()); err != nil {
		t.Fatalf("failed saving user: %v", err)
	}

	return n
}

func makeDevice(t *testing.T, u User) Device {
	n := Device{
		Owner:     u.ID,
		Name:      fmt.Sprintf("my test device %d", devCount),
		PublicKey: fmt.Sprintf("dummy value goes here %d", devCount),
		IP:        fmt.Sprintf("2001:db8::%d", devCount),
	}
	devCount++

	if err := n.Save(context.Background()); err != nil {
		t.Fatalf("failed saving user: %v", err)
	}

	return n
}

func mustJoinNetwork(t *testing.T, dev, nwid int64) {
	nw, err := NetworkID(context.Background(), nwid)
	if err != nil {
		t.Fatalf("failed to fetch network: %v", err)
	}

	if err := nw.Add(context.Background(), dev); err != nil {
		t.Fatalf("failed to add to network: %v", err)
	}
}

func TestNewUser(t *testing.T) {
	openDb(t)

	u := makeUser(t)

	nu, err := Username(context.Background(), u.Username)
	if err != nil {
		t.Fatalf("failed fetching user: %v", err)
		return
	}

	if !reflect.DeepEqual(u, nu) {
		t.Fatalf("expected %v, got %v", u, nu)
	}

	nu, err = UserID(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("failed fetching user: %v", err)
		return
	}

	if !reflect.DeepEqual(u, nu) {
		t.Fatalf("expected %v, got %v", u, nu)
	}

	us, err := Users(context.Background())
	if err != nil {
		t.Fatalf("failed fetching users: %v", err)
		return
	}

	if !reflect.DeepEqual(u, us[0]) {
		t.Fatalf("expected %v, got %v", u, us[0])
	}
}

func TestUpdateUser(t *testing.T) {
	openDb(t)

	u := makeUser(t)
	u.Name = "cool person"
	if err := u.Save(context.Background()); err != nil {
		t.Fatalf("failed to update user: %v", err)
	}

	nu, err := UserID(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("failed fetching user: %v", err)
		return
	}

	if !reflect.DeepEqual(u, nu) {
		t.Fatalf("expected %v, got %v", u, nu)
	}
}

func TestDelUser(t *testing.T) {
	openDb(t)

	u := makeUser(t)

	if err := u.Delete(context.Background()); err != nil {
		t.Fatalf("delete user failed: %v", err)
		return
	}

	_, err := Username(context.Background(), u.Username)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("delete failed, user still exists: %v", err)
		return
	}
}

func TestUserPassword(t *testing.T) {
	openDb(t)

	u := makeUser(t)
	if err := u.SetPassword(context.Background(), "hunter2"); err != nil {
		t.Fatalf("failed setting password: %v", err)
		return
	}

	if CheckPassword(context.Background(), u.Username, "hunter2") == -1 {
		t.Fatal("invalid username or password")
	}

	if CheckPassword(context.Background(), u.Username, "hunter1") != -1 {
		t.Fatal("invalid password passed")
	}
}

func TestNewNetwork(t *testing.T) {
	openDb(t)

	u := makeUser(t)
	nw := makeNetwork(t, u)

	nnw, err := NetworkID(context.Background(), nw.ID)
	if err != nil {
		t.Fatalf("failed fetching network: %v", err)
		return
	}

	if !reflect.DeepEqual(nw, nnw) {
		t.Fatalf("expected %v, got %v", nw, nnw)
	}

	ns, err := Networks(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("failed fetching users: %v", err)
		return
	}

	if !reflect.DeepEqual(nw, ns[0]) {
		t.Fatalf("expected %v, got %v", nw, ns[0])
	}
}

func TestUpdateNetwork(t *testing.T) {
	openDb(t)

	u := makeUser(t)
	nw := makeNetwork(t, u)
	nw.Name = "network name"

	if err := nw.Save(context.Background()); err != nil {
		t.Fatalf("failed saving network: %v", err)
	}

	nnw, err := NetworkID(context.Background(), nw.ID)
	if err != nil {
		t.Fatalf("failed fetching network: %v", err)
		return
	}

	if !reflect.DeepEqual(nw, nnw) {
		t.Fatalf("expected %v, got %v", nw, nnw)
	}
}

func TestDelNetwork(t *testing.T) {
	openDb(t)

	u := makeUser(t)
	nw := makeNetwork(t, u)

	if err := nw.Delete(context.Background()); err != nil {
		t.Fatalf("delete network failed: %v", err)
		return
	}

	_, err := NetworkID(context.Background(), nw.ID)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("delete failed, network still exists: %v", err)
		return
	}
}

func TestNewDevice(t *testing.T) {
	openDb(t)

	u := makeUser(t)
	dev := makeDevice(t, u)

	if dev.ID == 0 {
		t.Fatalf("device ID is zero")
	}

	ndev, err := DeviceID(context.Background(), dev.ID)
	if err != nil {
		t.Fatalf("failed fetching network: %v", err)
	}

	if !reflect.DeepEqual(dev, ndev) {
		t.Fatalf("expected %v, got %v", dev, ndev)
	}

	devs, err := Devices(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("failed fetching devices: %v", err)
	}

	if len(devs) == 0 {
		t.Fatal("zero items returned, should have one")
	} else if !reflect.DeepEqual(dev, devs[0]) {
		t.Fatalf("expected %v, got %v", dev, devs[0])
	}
}

func TestUpdateDevice(t *testing.T) {
	openDb(t)

	u := makeUser(t)
	dev := makeDevice(t, u)
	dev.Name = "new device name"

	if err := dev.Save(context.Background()); err != nil {
		t.Fatalf("failed updating device: %v", err)
	}

	ndev, err := DeviceID(context.Background(), dev.ID)
	if err != nil {
		t.Fatalf("failed fetching network: %v", err)
	}

	if !reflect.DeepEqual(dev, ndev) {
		t.Fatalf("expected %v, got %v", dev, ndev)
	}
}

func TestDelDevice(t *testing.T) {
	openDb(t)

	u := makeUser(t)
	dev := makeDevice(t, u)

	if err := dev.Delete(context.Background()); err != nil {
		t.Fatalf("delete network failed: %v", err)
		return
	}

	_, err := DeviceID(context.Background(), dev.ID)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("delete failed, network still exists: %v", err)
		return
	}
}

func TestAddDevice(t *testing.T) {
	openDb(t)

	u := makeUser(t)
	nw := makeNetwork(t, u)
	dev := makeDevice(t, u)

	if err := nw.Add(context.Background(), dev.ID); err != nil {
		t.Fatalf("failed to add device to network: %v", err)
	}

	devs, err := NetworkDevices(context.Background(), nw.ID)
	if err != nil {
		t.Fatalf("failed to list devices: %v", err)
	}

	if len(devs) == 0 {
		t.Fatal("returned zero devices, should have one")
	}

	if !reflect.DeepEqual(dev, devs[0]) {
		t.Fatalf("expected %v, got %v", dev, devs[0])
	}
}

func TestRemoveDevice(t *testing.T) {
	openDb(t)

	u := makeUser(t)
	nw := makeNetwork(t, u)
	dev := makeDevice(t, u)

	if err := nw.Add(context.Background(), dev.ID); err != nil {
		t.Fatalf("failed to add device to network: %v", err)
	}

	if err := nw.Remove(context.Background(), dev.ID); err != nil {
		t.Fatalf("failed to delete device from network: %v", err)
	}

	devs, err := NetworkDevices(context.Background(), nw.ID)
	if err != nil {
		t.Fatalf("failed to list devices: %v", err)
	}

	if len(devs) != 0 {
		t.Fatalf("returned %d devices, should have zero", len(devs))
	}
}

func TestDeviceNetworks(t *testing.T) {
	openDb(t)

	u := makeUser(t)
	nw := makeNetwork(t, u)
	nw2 := makeNetwork(t, u)
	nw3 := makeNetwork(t, u)
	dev := makeDevice(t, u)
	dev2 := makeDevice(t, u)

	mustJoinNetwork(t, dev.ID, nw.ID)
	mustJoinNetwork(t, dev.ID, nw2.ID)
	mustJoinNetwork(t, dev2.ID, nw2.ID)
	mustJoinNetwork(t, dev2.ID, nw3.ID)

	// device 1
	nws, err := DeviceNetworks(context.Background(), dev.ID)
	if err != nil {
		t.Fatalf("failed to fetch device 2 networks: %v", err)
	}

	if len(nws) != 2 {
		t.Fatalf("device 1 is in %d networks, expected 2", len(nws))
	}

	if !reflect.DeepEqual(nws[0], nw) {
		t.Fatalf("device 1 nws[0] = %v, expected %v", nws[0], nw)
	}
	if !reflect.DeepEqual(nws[1], nw2) {
		t.Fatalf("device 1 nws[1] = %v, expected %v", nws[1], nw2)
	}

	// device 2
	nws, err = DeviceNetworks(context.Background(), dev2.ID)
	if err != nil {
		t.Fatalf("failed to fetch device 2 networks: %v", err)
	}

	if len(nws) != 2 {
		t.Fatalf("device 2 is in %d networks, expected 2", len(nws))
	}

	if !reflect.DeepEqual(nws[0], nw2) {
		t.Fatalf("device 2 nws[0] = %v, expected %v", nws[0], nw2)
	}
	if !reflect.DeepEqual(nws[1], nw3) {
		t.Fatalf("device 2 nws[1] = %v, expected %v", nws[1], nw3)
	}
}

func TestConnectedTo(t *testing.T) {
	openDb(t)

	u := makeUser(t)
	nw := makeNetwork(t, u)
	nw2 := makeNetwork(t, u)
	dev := makeDevice(t, u)
	dev2 := makeDevice(t, u)
	dev3 := makeDevice(t, u)

	mustJoinNetwork(t, dev.ID, nw.ID)
	mustJoinNetwork(t, dev2.ID, nw.ID)
	mustJoinNetwork(t, dev3.ID, nw.ID)
	mustJoinNetwork(t, dev2.ID, nw2.ID)
	mustJoinNetwork(t, dev3.ID, nw2.ID)

	devs, err := dev.ConnectedTo(context.Background())
	if err != nil {
		t.Fatalf("failed to fetch device connections: %v", err)
	}

	if len(devs) != 2 {
		t.Fatalf("device 1 knows %d devices, expected 2", len(devs))
	}

	// I don't know why the order is swapped, it just is
	if !reflect.DeepEqual(devs[0], dev3) {
		t.Fatalf("device 1 devs[0] = %v, expected %v", devs[0], dev3)
	}
	if !reflect.DeepEqual(devs[1], dev2) {
		t.Fatalf("device 1 devs[1] = %v, expected %v", devs[1], dev2)
	}
}
