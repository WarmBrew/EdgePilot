package device

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const tokenLength = 32 // 32 bytes = 64 hex chars

// GenerateAgentToken creates a cryptographically secure random token.
func GenerateAgentToken() string {
	b := make([]byte, tokenLength)
	if _, err := rand.Read(b); err != nil {
		// This should never happen in practice
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// HashAgentToken computes the SHA-256 hash of a plain token.
func HashAgentToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// VerifyAgentToken compares a plain token against a stored hash.
func VerifyAgentToken(hashed, plain string) bool {
	return HashAgentToken(plain) == hashed
}
