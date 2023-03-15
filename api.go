package main

// This package holds all API routes.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mca3/mwr"
)

var (
	errNoAuth = errors.New("need authentication")
)

// genApiError creates a function which is capable of sending predetermined
// error messages to the client.
func genApiError(code int, msg string) func(c *mwr.Ctx, e ...error) error {
	return func(c *mwr.Ctx, e ...error) error {
		err := c.Status(code).SendString(msg)
		if e != nil {
			return e[0]
		}
		return err
	}
}

// API error functions
var (
	api400 = genApiError(400, "Bad Request")
	api403 = genApiError(403, "Forbidden")
	api404 = genApiError(404, "Not Found")
	api500 = genApiError(500, "Internal Server Error")
)

// tryAuth decodes a token and attempts to authenticate as a user using it.
func tryAuth(token string) (*User, bool) {
	// TODO: This is *very* temporary!
	i, _ := strconv.Atoi(token)
	u, err := UserID(context.Background(), int64(i))
	return &u, err == nil
}

// apiJSON encodes data as JSON and sends it to the client.
func apiJSON(c *mwr.Ctx, data any) error {
	c.Set("Content-Type", "application/json")

	err := json.NewEncoder(c).Encode(data)
	if err != nil {
		return api500(c, err)
	}
	return nil
}

// apiIsAuthed determines if the client is authenticated or not.
func apiIsAuthed(c *mwr.Ctx) (*User, bool) {
	// We're looking for the Authorization header or a cookie
	val := c.Get("Authorization")
	if val != "" {
		return tryAuth(strings.TrimPrefix(val, "Bearer "))
	}

	// TODO: Try cookie. mwr has no support yet.

	return nil, false
}

// apiAuth creates a token for the client from a username and password.
// XXX: The token is a dummy token. Should be a JWT.
//
// Path: /api/auth
// Method: POST
// Body: JSON. Must have the strings "username", and "password".
func apiAuth(c *mwr.Ctx) error {
	if c.Method() == "GET" {
		resp := struct {
			Methods []string `json:"methods"`
		}{
			Methods: []string{
				"username-password",
			},
		}
		return apiJSON(c, resp)
	}
	data := struct {
		Username, Password, Method string
	}{}

	if err := c.BodyParser(&data); err != nil {
		return api400(c, err)
	} else if data.Username == "" || data.Password == "" {
		// TODO: Proper error response
		return api400(c)
	} else if data.Method != "username-password" {
		// TODO: Proper error response
		return api400(c)
	}

	if uid := CheckPassword(c.Context(), data.Username, data.Password); uid != -1 {
		return c.SendString(fmt.Sprint(uid))
	}

	return api403(c) // TODO: Something proper
}

// apiNewUser creates a new user.
// XXX: This is a debug route. There is no authentication.
//
// Path: /api/new/user
// Method: POST
// Body: JSON. Must have the strings "username", "email", and "password".
func apiNewUser(c *mwr.Ctx) error {
	data := struct {
		Username, Email, Password string
	}{}

	if err := c.BodyParser(&data); err != nil {
		return api400(c, err)
	} else if data.Username == "" || data.Email == "" || data.Password == "" {
		return api400(c)
	}

	u := User{
		username: data.Username,
		email:    data.Email,
	}

	if err := u.Save(c.Context()); err != nil {
		return api500(c, err)
	}

	if err := u.SetPassword(c.Context(), data.Password); err != nil {
		return api500(c, err)
	}

	return c.SendString(fmt.Sprint(u.id))
}

// apiDeleteUser deletes a user.
// XXX: This is a debug route. There is no authentication.
//
// Path: /api/del/user
// Method: POST
// Body: JSON. Must have the strings "username" or "id".
func apiDeleteUser(c *mwr.Ctx) error {
	data := struct {
		Username string
		ID       int64
	}{}

	if err := c.BodyParser(&data); err != nil {
		return api400(c, err)
	} else if data.Username == "" && data.ID == 0 {
		return api400(c)
	}

	u := User{
		username: data.Username,
		id:       data.ID,
	}

	if err := u.Delete(c.Context()); err != nil {
		return api500(c, err)
	}

	return nil
}

// apiNewDevice creates a new device and attaches it to the user's account.
//
// Path: /api/new/device
// Method: POST
// Authenticated.
// Body: JSON. Specify "name" and "key", where "key" is a WireGuard public key.
func apiNewDevice(c *mwr.Ctx) error {
	user, ok := apiIsAuthed(c)
	if !ok {
		return api403(c, errNoAuth)
	}

	data := struct {
		Name, Key string
	}{}

	if err := c.BodyParser(&data); err != nil {
		return api400(c, err)
	} else if data.Name == "" || data.Key == "" {
		return api400(c)
	}

	dev := Device{
		Name:      data.Name,
		Owner:     user.id,
		PublicKey: data.Key,
		IP:        genIPv6(),
	}
	if err := dev.Save(c.Context()); err != nil {
		return api500(c, err)
	}

	return apiJSON(c, dev)
}

// apiListDevices lists the devices on the user's account.
//
// Path: /api/list/devices
// Method: GET
// Authenticated.
func apiListDevices(c *mwr.Ctx) error {
	user, ok := apiIsAuthed(c)
	if !ok {
		return api403(c, errNoAuth)
	}

	devs, err := Devices(c.Context(), user.id)
	if err != nil {
		return api500(c, err)
	}

	return apiJSON(c, devs)
}

// apiDeleteDevice deletes a device from the user's account.
//
// Path: /api/del/device
// Method: POST
// Authenticated.
// Body: JSON. Specify "id".
func apiDeleteDevice(c *mwr.Ctx) error {
	user, ok := apiIsAuthed(c)
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

	dev, err := DeviceID(c.Context(), data.ID)
	if err != nil {
		return api500(c, err)
	}

	if dev.Owner != user.id {
		return api404(c)
	}

	if err := dev.Delete(c.Context()); err != nil {
		return api500(c, err)
	}

	return c.SendStatus(204)
}
