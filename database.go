package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

var db *pgxpool.Pool

const pqConfigSchema = `
CREATE TABLE IF NOT EXISTS config(
	version INTEGER NOT NULL DEFAULT 0
);
`

const pqSchema = `
CREATE TABLE users(
	id SERIAL PRIMARY KEY,
	username VARCHAR(32) NOT NULL UNIQUE,
	email VARCHAR(256) NOT NULL UNIQUE,
	name VARCHAR(64),
	password VARCHAR(256) NOT NULL
);

CREATE TABLE networks(
	id SERIAL PRIMARY KEY,
	owner INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name VARCHAR(64) NOT NULL UNIQUE
);

CREATE TABLE devices(
	id SERIAL PRIMARY KEY,
	owner INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name VARCHAR(64) NOT NULL UNIQUE,
	pubkey VARCHAR(64) NOT NULL UNIQUE,
	ip VARCHAR(39) NOT NULL UNIQUE
);

CREATE TABLE nwdevs(
	network INTEGER NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
	device INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,

	UNIQUE(Network, device)
);
`

var pqMigrations = []string{
	"", // schema init
}

type User struct {
	id       int64
	username string
	email    string
	name     string
}

type Network struct {
	id    int64
	owner int64
	name  string
}

type Device struct {
	id     int64
	owner  int64
	name   string
	pubkey string
	ip     string
}

// connect connects to PostgreSQL and updates the schema if it is needed.
func connect() error {
	var err error
	db, err = pgxpool.Connect(context.Background(), databaseUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}

	if err := upgrade(); err != nil {
		db.Close()
		return err
	}

	return nil
}

func disconnect() {
	db.Close()
}

// upgrade upgrades the database schema.
func upgrade() error {
	tx, err := db.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	// Attempt to initialize the config schema on every startup.
	// This ensures that something is there when we check it next.
	if _, err := tx.Exec(context.Background(), pqConfigSchema); err != nil {
		return fmt.Errorf("failed to run config schema: %v", err)
	}

	// Read the version from the possibly recently initialized config
	// table.
	ver := 0
	if err := tx.QueryRow(context.Background(), "SELECT version FROM config").Scan(&ver); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("failed to determine version: %v", err)
	}

	if ver == 0 {
		// Version zero is special and initializes the database.
		if _, err := tx.Exec(context.Background(), pqSchema); err != nil {
			return fmt.Errorf("failed to init database: %v", err)
		}
		ver = len(pqMigrations)
	} else {
		// A version greater than zero will attempt to upgrade the
		// database.
		// All of this is in a transaction so if something fails, we
		// fail to start.

		for k, v := range pqMigrations[ver:] {
			if _, err := tx.Exec(context.Background(), v); err != nil {
				return fmt.Errorf("failed to upgrade to %d: %v", k, err)
			}
		}
	}

	// Set the new schema version.
	if _, err := tx.Exec(context.Background(), "UPDATE config SET version = $1", ver); err != nil {
		return fmt.Errorf("failed to set version: %v", err)
	}

	return tx.Commit(context.Background())
}

// Users returns all users in the database.
func Users(ctx context.Context) ([]User, error) {
	var us []User

	rows, err := db.Query(ctx, "SELECT id, username FROM users")
	if err != nil {
		return us, err
	}
	defer rows.Close()

	for rows.Next() {
		u := User{}
		if err := rows.Scan(&u.id, &u.username); err != nil {
			return us, err
		}
		us = append(us, u)
	}

	return us, err
}

// UserID returns a user from their ID.
func UserID(ctx context.Context, user int64) (User, error) {
	u := User{id: user}

	err := db.QueryRow(ctx, `
		SELECT
			username,
			email,
			name
		FROM users
		WHERE id = $1
	`, user).Scan(&u.username, &u.email, &u.name)
	return u, err
}

// Username returns a user from their username.
func Username(ctx context.Context, user string) (User, error) {
	u := User{username: user}

	err := db.QueryRow(ctx, `
		SELECT
			id,
			email,
			name
		FROM users
		WHERE id = $1
	`, user).Scan(&u.id, &u.email, &u.name)
	return u, err
}

// Save saves user information.
//
// If the user's ID is zero, a new user will be created.
func (n *User) Save(ctx context.Context) error {
	var err error
	if n.id == 0 {
		err = db.QueryRow(ctx, `
			INSERT INTO users (Username, email, name) VALUES ($1, $2, $3)
			RETURNING id
		`, n.username, n.email, nullString(n.name)).Scan(&n.id)
	} else {
		_, err = db.Exec(ctx, `
			UPDATE users SET
				email = $2,
				name = $3
			WHERE
				id = $1
		`, n.id, n.email, nullString(n.name))
	}
	return err
}

// SetPassword sets the user password.
func (n *User) SetPassword(ctx context.Context, pass string) error {
	salt := makeSalt()
	ct := hashPassword(pass, salt)

	_, err := db.Exec(ctx, "UPDATE users SET password = $1, salt = $2 WHERE id = $3", ct, salt, n.id)
	return err
}

// CheckPassword compares a supplied password with the user's password.
//
// If the password is invalid or the user does not exist, -1 is returned.
// Otherwise, the returned int64 is the user's ID.
func CheckPassword(ctx context.Context, user, pass string) int64 {
	var ct, salt []byte
	var id int64

	if err := db.QueryRow(ctx, `SELECT id, password, salt FROM users WHERE username = $1`, user).Scan(&id, &ct, &salt); err != nil {
		return -1
	}

	if bytes.Equal(hashPassword(pass, salt), ct) {
		return id
	}
	return -1
}

// Delete deletes the user from the database, along with all of their networks
// and devices.
func (n *User) Delete(ctx context.Context) error {
	_, err := db.Query(ctx, "DELETE FROM users WHERE id = $1", n.id)
	return err
}

// Networks returns all networks that a user has created.
func Networks(ctx context.Context, user int64) ([]Network, error) {
	var ns []Network

	rows, err := db.Query(ctx, "SELECT id, name FROM networks WHERE owner = $1", user)
	if err != nil {
		return ns, err
	}
	defer rows.Close()

	for rows.Next() {
		n := Network{owner: user}
		if err := rows.Scan(&n.id, &n.name); err != nil {
			return ns, err
		}
		ns = append(ns, n)
	}

	return ns, err
}

// NetworkID returns a network from its ID.
func NetworkID(ctx context.Context, nwid int64) (Network, error) {
	n := Network{id: nwid}

	err := db.QueryRow(ctx, `
		SELECT
			owner,
			name
		FROM networks
		WHERE id = $1
	`, nwid).Scan(&n.owner, &n.name)
	return n, err
}

// NetworkDevices returns all devices that are supposed to be connected to a
// given network.
func NetworkDevices(ctx context.Context, nwid int64) ([]Device, error) {
	var ns []Device

	rows, err := db.Query(ctx, `
		SELECT
			id,
			owner,
			name,
			pubkey,
			ip
		FROM (SELECT device FROM nwdevs WHERE network = $1)
	`, nwid)
	if err != nil {
		return ns, err
	}
	defer rows.Close()

	for rows.Next() {
		n := Device{}
		if err := rows.Scan(&n.id, &n.owner, &n.name, &n.pubkey, &n.ip); err != nil {
			return ns, err
		}
		ns = append(ns, n)
	}

	return ns, err
}

// Add adds a device to the network.
func (n *Network) Add(ctx context.Context, devid int64) error {
	_, err := db.Exec(ctx, `INSERT INTO nwdevs(Network, device) VALUES($1, $2)`, n.id, devid)
	return err
}

// Remove removes a device from the network.
func (n *Network) Remove(ctx context.Context, devid int64) error {
	_, err := db.Exec(ctx, `DELETE FROM nwdevs VALUES($1, $2)`, n.id, devid)
	return err
}

// Save updates existing network information or creates a new network.
func (n *Network) Save(ctx context.Context) error {
	var err error
	if n.id == 0 {
		_, err = db.Exec(ctx, `
			INSERT INTO networks (owner, name) VALUES ($1, $2)
		`, n.owner, n.name)
	} else {
		_, err = db.Exec(ctx, `
			UPDATE networks SET
				name = $2
			WHERE
				id = $1
		`, n.id, n.name)
	}
	return err
}

// Delete deletes the network.
func (n *Network) Delete(ctx context.Context) error {
	_, err := db.Query(ctx, "DELETE FROM networks WHERE id = $1", n.id)
	return err
}

// Devices returns all devices for a user.
func Devices(ctx context.Context, user int64) ([]Device, error) {
	var ns []Device

	rows, err := db.Query(ctx, `
		SELECT
			id,
			name,
			pubkey,
			ip
		FROM devices
		WHERE owner = $1
	`, user)
	if err != nil {
		return ns, err
	}
	defer rows.Close()

	for rows.Next() {
		n := Device{owner: user}
		if err := rows.Scan(&n.id, &n.name, &n.pubkey, &n.ip); err != nil {
			return ns, err
		}
		ns = append(ns, n)
	}

	return ns, err
}

// DeviceID returns a device from its ID.
func DeviceID(ctx context.Context, devid int64) (Device, error) {
	d := Device{id: devid}

	err := db.QueryRow(ctx, `
		SELECT
			owner,
			name,
			pubkey,
			ip
		FROM devices
		WHERE id = $1
	`, devid).Scan(&d.owner, &d.name, &d.pubkey, &d.ip)
	return d, err
}

// Save updates existing device information or creates a new device.
func (n *Device) Save(ctx context.Context) error {
	if n.ip == "" {
		panic("ip is nil")
	}

	var err error
	if n.id == 0 {
		_, err = db.Exec(ctx, `
			INSERT INTO devices(
				owner, name, pubkey, ip
			) VALUES ($1, $2, $3, $4)
		`, n.owner, nullString(n.name), n.pubkey, n.ip)
	} else {
		_, err = db.Exec(ctx, `
			UPDATE networks SET
				name = $2
			WHERE
				id = $1
		`, n.id, n.name)
	}
	return err
}

// Delete deletes the device from the database.
func (n *Device) Delete(ctx context.Context) error {
	_, err := db.Query(ctx, "DELETE FROM devices WHERE id = $1", n.id)
	return err
}
