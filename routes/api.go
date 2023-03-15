package routes

// This package holds all API routes.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mca3/mwr"
	"github.com/mca3/pikorv/db"
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
func tryAuth(token string) (*db.User, bool) {
	// TODO: This is *very* temporary!
	i, _ := strconv.Atoi(token)
	u, err := db.UserID(context.Background(), int64(i))
	return &u, err == nil
}

// sendJSON encodes data as JSON and sends it to the client.
func sendJSON(c *mwr.Ctx, data any) error {
	c.Set("Content-Type", "application/json")

	err := json.NewEncoder(c).Encode(data)
	if err != nil {
		return api500(c, err)
	}
	return nil
}

// isAuthed determines if the client is authenticated or not.
func isAuthed(c *mwr.Ctx) (*db.User, bool) {
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
func Auth(c *mwr.Ctx) error {
	if c.Method() == "GET" {
		resp := struct {
			Methods []string `json:"methods"`
		}{
			Methods: []string{
				"username-password",
			},
		}
		return sendJSON(c, resp)
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

	if uid := db.CheckPassword(c.Context(), data.Username, data.Password); uid != -1 {
		return c.SendString(fmt.Sprint(uid))
	}

	return api403(c) // TODO: Something proper
}
