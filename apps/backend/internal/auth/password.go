package auth

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const passwordCost = 12

// HashPassword hashes a plaintext password using bcrypt.
func HashPassword(plaintext string) (string, error) {
	if strings.TrimSpace(plaintext) == "" {
		return "", errors.New("password must not be empty")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(plaintext), passwordCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// VerifyPassword verifies a plaintext password against a stored bcrypt hash.
func VerifyPassword(hash, plaintext string) error {
	if hash == "" || plaintext == "" {
		return errors.New("invalid credentials")
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
}
