package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const BcryptCost = 12

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

func CheckPassword(hashed, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
	if err != nil {
		return fmt.Errorf("password mismatch: %w", err)
	}
	return nil
}
