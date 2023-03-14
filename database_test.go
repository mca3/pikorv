package main

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v4"
)

func openDb(t *testing.T) {
	u := os.Getenv("POSTGRES_TEST")
	if u == "" {
		t.Skip("Set POSTGRES_TEST to run database tests")
	}

	databaseUrl = u
	if err := connect(true); err != nil {
		panic(err)
	}

	t.Cleanup(disconnect)
}

func makeUser(t *testing.T) User {
	u := User{
		username: "test",
		email:    "test@example.com",
		name:     "Test User",
	}

	if err := u.Save(context.Background()); err != nil {
		t.Fatalf("failed saving user: %v", err)
	}

	return u
}

func makeNetwork(t *testing.T, u User) Network {
	n := Network{
		owner: u.id,
		name:  "test network",
	}

	if err := n.Save(context.Background()); err != nil {
		t.Fatalf("failed saving user: %v", err)
	}

	return n
}

func makeDevice(t *testing.T, u User) Device {
	n := Device{
		owner:  u.id,
		name:   "my test device",
		pubkey: "dummy value goes here",
		ip:     "2001:db8::1",
	}

	if err := n.Save(context.Background()); err != nil {
		t.Fatalf("failed saving user: %v", err)
	}

	return n
}

func TestNewUser(t *testing.T) {
	openDb(t)

	u := makeUser(t)

	nu, err := Username(context.Background(), u.username)
	if err != nil {
		t.Fatalf("failed fetching user: %v", err)
		return
	}

	if !reflect.DeepEqual(u, nu) {
		t.Fatalf("expected %v, got %v", u, nu)
	}

	nu, err = UserID(context.Background(), u.id)
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
	u.name = "cool person"
	if err := u.Save(context.Background()); err != nil {
		t.Fatalf("failed to update user: %v", err)
	}

	nu, err := UserID(context.Background(), u.id)
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

	_, err := Username(context.Background(), u.username)
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

	if CheckPassword(context.Background(), u.username, "hunter2") == -1 {
		t.Fatal("invalid username or password")
	}

	if CheckPassword(context.Background(), u.username, "hunter1") != -1 {
		t.Fatal("invalid password passed")
	}
}

func TestNewNetwork(t *testing.T) {
	openDb(t)

	u := makeUser(t)
	nw := makeNetwork(t, u)

	nnw, err := NetworkID(context.Background(), nw.id)
	if err != nil {
		t.Fatalf("failed fetching network: %v", err)
		return
	}

	if !reflect.DeepEqual(nw, nnw) {
		t.Fatalf("expected %v, got %v", nw, nnw)
	}

	ns, err := Networks(context.Background(), u.id)
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
	nw.name = "network name"

	if err := nw.Save(context.Background()); err != nil {
		t.Fatalf("failed saving network: %v", err)
	}

	nnw, err := NetworkID(context.Background(), nw.id)
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

	_, err := NetworkID(context.Background(), nw.id)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("delete failed, network still exists: %v", err)
		return
	}
}

func TestNewDevice(t *testing.T) {
	openDb(t)

	u := makeUser(t)
	dev := makeDevice(t, u)

	ndev, err := DeviceID(context.Background(), dev.id)
	if err != nil {
		t.Fatalf("failed fetching network: %v", err)
		return
	}

	if !reflect.DeepEqual(dev, ndev) {
		t.Fatalf("expected %v, got %v", dev, ndev)
	}

	devs, err := Devices(context.Background(), u.id)
	if err != nil {
		t.Fatalf("failed fetching devices: %v", err)
		return
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
	dev.name = "new device name"

	if err := dev.Save(context.Background()); err != nil {
		t.Fatalf("failed updating device: %v", err)
	}

	ndev, err := DeviceID(context.Background(), dev.id)
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

	_, err := DeviceID(context.Background(), dev.id)
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

	if err := nw.Add(context.Background(), dev.id); err != nil {
		t.Fatalf("failed to add device to network: %v", err)
	}

	devs, err := NetworkDevices(context.Background(), nw.id)
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

	if err := nw.Add(context.Background(), dev.id); err != nil {
		t.Fatalf("failed to add device to network: %v", err)
	}

	if err := nw.Remove(context.Background(), dev.id); err != nil {
		t.Fatalf("failed to delete device from network: %v", err)
	}

	devs, err := NetworkDevices(context.Background(), nw.id)
	if err != nil {
		t.Fatalf("failed to list devices: %v", err)
	}

	if len(devs) != 0 {
		t.Fatalf("returned %d devices, should have zero", len(devs))
	}
}
