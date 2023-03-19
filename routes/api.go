package routes

// This package holds all API routes.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mca3/mwr"
	"github.com/mca3/pikorv/config"
	"github.com/mca3/pikorv/db"

	"github.com/golang-jwt/jwt/v5"
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
	jtok, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}

		return []byte(config.JWTSecret), nil
	})
	if err != nil {
		log.Println(err)
		return nil, false
	} else if !jtok.Valid {
		return nil, false
	}

	d := jtok.Claims.(jwt.MapClaims)["id"].(float64)
	u, err := db.UserID(context.Background(), int64(d))
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

	uid := db.CheckPassword(c.Context(), data.Username, data.Password)
	if uid == -1 {
		return api403(c) // TODO: Something proper
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":  uid,
		"exp": time.Now().Add(time.Hour * 24 * 7).Unix(),
	})

	etoken, err := token.SignedString([]byte(config.JWTSecret))
	if err != nil {
		return api500(c, err)
	}

	return sendJSON(c, struct {
		Token string `json:"token"`
	}{etoken})
}
