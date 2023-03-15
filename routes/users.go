package routes

// This package holds all API routes.

import (
	"fmt"

	"github.com/mca3/mwr"
	"github.com/mca3/pikorv/db"
)

// apiNewUser creates a new user.
// XXX: This is a debug route. There is no authentication.
//
// Path: /api/new/user
// Method: POST
// Body: JSON. Must have the strings "username", "email", and "password".
func NewUser(c *mwr.Ctx) error {
	data := struct {
		Username, Email, Password string
	}{}

	if err := c.BodyParser(&data); err != nil {
		return api400(c, err)
	} else if data.Username == "" || data.Email == "" || data.Password == "" {
		return api400(c)
	}

	u := db.User{
		Username: data.Username,
		Email:    data.Email,
	}

	if err := u.Save(c.Context()); err != nil {
		return api500(c, err)
	}

	if err := u.SetPassword(c.Context(), data.Password); err != nil {
		return api500(c, err)
	}

	return c.SendString(fmt.Sprint(u.ID))
}

// apiDeleteUser deletes a user.
// XXX: This is a debug route. There is no authentication.
//
// Path: /api/del/user
// Method: POST
// Body: JSON. Must have the strings "username" or "id".
func DeleteUser(c *mwr.Ctx) error {
	data := struct {
		Username string
		ID       int64
	}{}

	if err := c.BodyParser(&data); err != nil {
		return api400(c, err)
	} else if data.Username == "" && data.ID == 0 {
		return api400(c)
	}

	u := db.User{
		Username: data.Username,
		ID:       data.ID,
	}

	if err := u.Delete(c.Context()); err != nil {
		return api500(c, err)
	}

	return nil
}
