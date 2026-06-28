package auth

import (
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 10

// HashPassword produces a bcrypt hash of the given plain-text password.
func HashPassword(plain string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPassword returns true if plain matches the stored bcrypt hash.
func CheckPassword(plain, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	return err == nil
}
