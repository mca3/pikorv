package main

import (
	"crypto/rand"
	"crypto/sha512"
	"database/sql"
)

const saltLength = 16

// makeSalt makes a salt of saltLength length.
func makeSalt() []byte {
	salt := make([]byte, saltLength)
	_, err := rand.Read(salt)
	if err != nil {
		// This really shouldn't happen
		panic(err)
	}

	return salt
}

// nullString converts a string to sql.NullString, with Valid set if the string
// is not empty.
func nullString(n string) sql.NullString {
	return sql.NullString{
		String: n,
		Valid:  n != "",
	}
}

// hashPassword takes the sha512 hash of a password and a salt.
func hashPassword(pass string, salt []byte) []byte {
	pt := append([]byte(pass), salt...)
	ct := sha512.Sum512(pt)
	return ct[:]
}
