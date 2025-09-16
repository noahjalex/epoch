package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	// Session token length in bytes (32 bytes = 64 hex chars)
	SessionTokenLength = 32
	// Default session duration
	DefaultSessionDuration = 30 * 24 * time.Hour // 30 days
)

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword verifies a password against a hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateSessionToken generates a cryptographically secure random session token
func GenerateSessionToken() (string, error) {
	bytes := make([]byte, SessionTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// IsSessionExpired checks if a session has expired
func IsSessionExpired(expiresAt time.Time) bool {
	return time.Now().After(expiresAt)
}

// GetSessionExpiry returns the expiry time for a new session
func GetSessionExpiry() time.Time {
	return time.Now().Add(DefaultSessionDuration)
}
