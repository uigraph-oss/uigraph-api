package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
)

const (
	tokenPrefix    = "uig_"
	tokenRandBytes = 32                   // 256 bits → 64 hex chars after encoding
	prefixLen      = len(tokenPrefix) + 8 // "uig_" + 8 random hex chars = 12 chars
)

// Generate produces a new token ID (UUID), the plaintext token, and its
// SHA-256 hex hash. Format: uig_<64 random hex chars>
//
// The plaintext is returned exactly once; only the hash is persisted.
func Generate() (id, plaintext, hash string, err error) {
	b := make([]byte, tokenRandBytes)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("identity: generate token: %w", err)
	}
	id = uuid.NewString()
	plaintext = tokenPrefix + hex.EncodeToString(b)
	hash = Hash(plaintext)
	return id, plaintext, hash, nil
}

// Hash returns the lower-case hex SHA-256 digest of plaintext.
func Hash(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// Prefix extracts the display prefix used for indexed database lookup.
// Returns the first 12 characters of plaintext (e.g. "uig_a3f9b1c2").
func Prefix(plaintext string) string {
	if len(plaintext) < prefixLen {
		return plaintext
	}
	return plaintext[:prefixLen]
}

// Verify returns true when the SHA-256 hash of plaintext equals storedHash.
// Uses a constant-time XOR comparison to prevent timing attacks.
func Verify(plaintext, storedHash string) bool {
	computed := Hash(plaintext)
	if len(computed) != len(storedHash) {
		return false
	}
	var diff byte
	for i := 0; i < len(computed); i++ {
		diff |= computed[i] ^ storedHash[i]
	}
	return diff == 0
}
