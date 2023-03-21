package db

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

var db *pgxpool.Pool

const pqConfigSchema = `
CREATE TABLE IF NOT EXISTS config(
	id SMALLINT PRIMARY KEY,
	version INTEGER NOT NULL DEFAULT 0,
	CHECK(id = 1)
);
`

const pqSchema = `
CREATE TABLE users(
	id SERIAL PRIMARY KEY,
	username VARCHAR(32) NOT NULL UNIQUE,
	email VARCHAR(256) NOT NULL UNIQUE,
	name VARCHAR(64),
	password bytea,
	salt bytea
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
	ip VARCHAR(39) NOT NULL UNIQUE,
	endpoint VARCHAR(64)
);

CREATE TABLE nwdevs(
	network INTEGER NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
	device INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,

	UNIQUE(Network, device)
);
`

var pqMigrations = []string{
	"", // schema init
	"ALTER TABLE devices ADD COLUMN endpoint VARCHAR(64)",
}

// User represents a rendezvous user.
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Name     string `json:"name,omitempty"`
}

// Network represents a network to which devices connect to
type Network struct {
	ID    int64  `json:"id"`
	Owner int64  `json:"owner"`
	Name  string `json:"name"`
}

// Device represnets a device, its unique Pikonet IP, and its public key.
type Device struct {
	ID    int64  `json:"id"`
	Owner int64  `json:"owner"`
	Name  string `json:"name"`

	// PublicKey is the WireGuard public key for this device.
	PublicKey string `json:"key"`

	// IP is the Pikonet IP, which likely means a random IP in the range
	// fd00::/32.
	// This IP is not routable by the Internet, and only by Pikonet nodes.
	IP string `json:"ip"`

	// Endpoint is the endpoint of this device, which is updated whenever
	// the endpoint pings us.
	//
	// TODO: Store a date with this, dump it if it's too old.
	Endpoint string `json:"endpoint,omitempty"`
}

// Connect connects to PostgreSQL and updates the schema if it is needed.
func Connect(url string, test ...bool) error {
	var err error
	db, err = pgxpool.Connect(context.Background(), url)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}

	if test != nil {
		_, err := db.Exec(context.Background(), "SET search_path TO pg_temp")
		if err != nil {
			panic(err)
		}
	}

	if err := upgrade(); err != nil {
		db.Close()
		return err
	}

	return nil
}

func Disconnect() {
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
	if _, err := tx.Exec(context.Background(), "INSERT INTO config(id, version) VALUES(1,$1) ON CONFLICT(id) DO UPDATE SET version = $1", len(pqMigrations)); err != nil {
		return fmt.Errorf("failed to set version: %v", err)
	}

	return tx.Commit(context.Background())
}

// Users returns all users in the database.
func Users(ctx context.Context) ([]User, error) {
	var us []User

	rows, err := db.Query(ctx, "SELECT id, username, email, name FROM users")
	if err != nil {
		return us, err
	}
	defer rows.Close()

	for rows.Next() {
		u := User{}
		var ns sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &ns); err != nil {
			return us, err
		}
		u.Name = ns.String
		us = append(us, u)
	}

	return us, err
}

// UserID returns a user from their ID.
func UserID(ctx context.Context, user int64) (User, error) {
	u := User{ID: user}

	var ns sql.NullString

	err := db.QueryRow(ctx, `
		SELECT
			username,
			email,
			name
		FROM users
		WHERE id = $1
	`, user).Scan(&u.Username, &u.Email, &ns)
	u.Name = ns.String
	return u, err
}

// Username returns a user from their username.
func Username(ctx context.Context, user string) (User, error) {
	u := User{Username: user}

	var ns sql.NullString

	err := db.QueryRow(ctx, `
		SELECT
			id,
			email,
			name
		FROM users
		WHERE username = $1
	`, user).Scan(&u.ID, &u.Email, &ns)
	u.Name = ns.String
	return u, err
}

// Save saves user information.
//
// If the user's ID is zero, a new user will be created.
func (n *User) Save(ctx context.Context) error {
	var err error
	if n.ID == 0 {
		err = db.QueryRow(ctx, `
			INSERT INTO users (username, email, name) VALUES ($1, $2, $3)
			RETURNING id
		`, n.Username, n.Email, nullString(n.Name)).Scan(&n.ID)
	} else {
		_, err = db.Exec(ctx, `
			UPDATE users SET
				email = $2,
				name = $3
			WHERE
				id = $1
		`, n.ID, n.Email, nullString(n.Name))
	}
	return err
}

// SetPassword sets the user password.
func (n *User) SetPassword(ctx context.Context, pass string) error {
	salt := makeSalt()
	ct := hashPassword(pass, salt)

	_, err := db.Exec(ctx, "UPDATE users SET password = $1, salt = $2 WHERE id = $3", ct, salt, n.ID)
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
	_, err := db.Exec(ctx, "DELETE FROM users WHERE id = $1", n.ID)
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
		n := Network{Owner: user}
		if err := rows.Scan(&n.ID, &n.Name); err != nil {
			return ns, err
		}
		ns = append(ns, n)
	}

	return ns, err
}

// NetworkID returns a network from its ID.
func NetworkID(ctx context.Context, nwid int64) (Network, error) {
	n := Network{ID: nwid}

	err := db.QueryRow(ctx, `
		SELECT
			owner,
			name
		FROM networks
		WHERE id = $1
	`, nwid).Scan(&n.Owner, &n.Name)
	return n, err
}

// NetworkDevices returns all devices that are supposed to be connected to a
// given network.
func NetworkDevices(ctx context.Context, nwid int64) ([]Device, error) {
	var ns []Device

	rows, err := db.Query(ctx, `
		SELECT
			devices.id,
			devices.owner,
			devices.name,
			devices.pubkey,
			devices.ip,
			devices.endpoint
		FROM nwdevs
		INNER JOIN devices ON devices.id = nwdevs.device
		WHERE network = $1
	`, nwid)
	if err != nil {
		return ns, err
	}
	defer rows.Close()

	for rows.Next() {
		n := Device{}
		var ens sql.NullString
		if err := rows.Scan(&n.ID, &n.Owner, &n.Name, &n.PublicKey, &n.IP, &ens); err != nil {
			return ns, err
		}
		n.Endpoint = ens.String
		ns = append(ns, n)
	}

	return ns, err
}

// DeviceNetworks returns all networks that this device is supposed to be
// connected to.
func DeviceNetworks(ctx context.Context, devid int64) ([]Network, error) {
	var ns []Network

	rows, err := db.Query(ctx, `
		SELECT
			networks.id,
			networks.owner,
			networks.name
		FROM nwdevs
		INNER JOIN networks ON networks.id = nwdevs.network
		WHERE device = $1
	`, devid)
	if err != nil {
		return ns, err
	}
	defer rows.Close()

	for rows.Next() {
		n := Network{}
		if err := rows.Scan(&n.ID, &n.Owner, &n.Name); err != nil {
			return ns, err
		}
		ns = append(ns, n)
	}

	return ns, err
}

// Add adds a device to the network.
func (n *Network) Add(ctx context.Context, devid int64) error {
	_, err := db.Exec(ctx, `INSERT INTO nwdevs(Network, device) VALUES($1, $2)`, n.ID, devid)
	return err
}

// Remove removes a device from the network.
func (n *Network) Remove(ctx context.Context, devid int64) error {
	_, err := db.Exec(ctx, `DELETE FROM nwdevs WHERE network = $1 AND device = $2`, n.ID, devid)
	return err
}

// Save updates existing network information or creates a new network.
func (n *Network) Save(ctx context.Context) error {
	var err error
	if n.ID == 0 {
		err = db.QueryRow(ctx, `
			INSERT INTO networks (owner, name) VALUES ($1, $2)
			RETURNING id
		`, n.Owner, n.Name).Scan(&n.ID)
	} else {
		_, err = db.Exec(ctx, `
			UPDATE networks SET
				name = $2
			WHERE
				id = $1
		`, n.ID, n.Name)
	}
	return err
}

// Delete deletes the network.
func (n *Network) Delete(ctx context.Context) error {
	_, err := db.Exec(ctx, "DELETE FROM networks WHERE id = $1", n.ID)
	return err
}

// AllDevices returns all devices.
func AllDevices(ctx context.Context) ([]Device, error) {
	var ns []Device

	rows, err := db.Query(ctx, `
		SELECT
			id,
			owner,
			name,
			pubkey,
			ip,
			endpoint
		FROM devices
	`)
	if err != nil {
		return ns, err
	}
	defer rows.Close()

	for rows.Next() {
		n := Device{}
		var ens sql.NullString
		if err := rows.Scan(&n.ID, &n.Owner, &n.Name, &n.PublicKey, &n.IP, &ens); err != nil {
			return ns, err
		}
		n.Endpoint = ens.String
		ns = append(ns, n)
	}

	return ns, err
}

// Devices returns all devices for a user.
func Devices(ctx context.Context, user int64) ([]Device, error) {
	var ns []Device

	rows, err := db.Query(ctx, `
		SELECT
			id,
			name,
			pubkey,
			ip,
			endpoint
		FROM devices
		WHERE owner = $1
	`, user)
	if err != nil {
		return ns, err
	}
	defer rows.Close()

	for rows.Next() {
		n := Device{Owner: user}
		var ens sql.NullString
		if err := rows.Scan(&n.ID, &n.Name, &n.PublicKey, &n.IP, &ens); err != nil {
			return ns, err
		}
		n.Endpoint = ens.String
		ns = append(ns, n)
	}

	return ns, err
}

// DeviceID returns a device from its ID.
func DeviceID(ctx context.Context, devid int64) (Device, error) {
	d := Device{ID: devid}
	var ens sql.NullString

	err := db.QueryRow(ctx, `
		SELECT
			owner,
			name,
			pubkey,
			ip,
			endpoint
		FROM devices
		WHERE id = $1
	`, devid).Scan(&d.Owner, &d.Name, &d.PublicKey, &d.IP, &ens)
	d.Endpoint = ens.String
	return d, err
}

// Save updates existing device information or creates a new device.
func (n *Device) Save(ctx context.Context) error {
	if n.IP == "" {
		panic("ip is nil")
	}

	var err error
	if n.ID == 0 {
		err = db.QueryRow(ctx, `
			INSERT INTO devices(
				owner, name, pubkey, ip, endpoint
			) VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, n.Owner, nullString(n.Name), n.PublicKey, n.IP, nullString(n.Endpoint)).Scan(&n.ID)
	} else {
		_, err = db.Exec(ctx, `
			UPDATE devices
			SET
				name = $2,
				pubkey = $3,
				ip = $4,
				endpoint = $5
			WHERE
				id = $1
		`, n.ID, nullString(n.Name), n.PublicKey, n.IP, nullString(n.Endpoint))
	}
	return err
}

// Delete deletes the device from the database.
func (n *Device) Delete(ctx context.Context) error {
	_, err := db.Exec(ctx, "DELETE FROM devices WHERE id = $1", n.ID)
	return err
}

// ConnectedTo returns a list of devices that this device is connected to.
func (n *Device) ConnectedTo(ctx context.Context) ([]Device, error) {
	rows, err := db.Query(ctx, `
		WITH nets AS (
			SELECT
				network
			FROM nwdevs
			WHERE device = $1
		)
		SELECT
			devices.id,
			devices.owner,
			devices.name,
			devices.pubkey,
			devices.ip,
			devices.endpoint
		FROM devices
		INNER JOIN nwdevs ON
			nwdevs.device = devices.id
			AND nwdevs.device != $1
			AND network IN (SELECT network FROM nets)
		GROUP BY devices.id
	`, n.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devs []Device

	for rows.Next() {
		dev := Device{}
		var ens sql.NullString

		if err := rows.Scan(&dev.ID, &dev.Owner, &dev.Name, &dev.PublicKey, &dev.IP, &ens); err != nil {
			return devs, err
		}

		dev.Endpoint = ens.String
		devs = append(devs, dev)
	}

	return devs, nil
}
