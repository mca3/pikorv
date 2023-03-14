package main

// This package holds all API routes.

import (
	"fmt"

	"github.com/mca3/mwr"
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

// apiIsAuthed determines if the client is authenticated or not.
func apiIsAuthed(c *mwr.Ctx) (*User, bool) {
	return nil, false
}

// apiNewUser creates a new user.
// XXX: This is a debug route. There is no authentication.
//
// Path: /api/new/user
// Method: POST
// Body: JSON.
//
//	Must have the strings "username", "email", and "password".
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
// Body: JSON.
//
//	Must have the strings "username" or "id".
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

// apiAuth creates a token for the client from a username and password.
// XXX: The token is a dummy token. Should be a JWT.
//
// Path: /api/auth
// Method: POST
// Body: JSON.
//
//	Must have the strings "username", and "password".
func apiAuth(c *mwr.Ctx) error {
	data := struct {
		Username, Password string
	}{}

	if err := c.BodyParser(&data); err != nil {
		return api400(c, err)
	} else if data.Username == "" || data.Password == "" {
		return api400(c)
	}

	if uid := CheckPassword(c.Context(), data.Username, data.Password); uid != -1 {
		return c.SendString(fmt.Sprint(uid))
	}

	return api403(c) // TODO: Something proper
}
