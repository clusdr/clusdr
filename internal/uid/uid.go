// Package uid generates random unique identifiers.
package uid

import (
	"crypto/rand"
	"fmt"
)

// New returns a new random UUID v4 string.
func New() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate uid: %w", err)
	}
	// Set version (4) and variant bits per RFC 4122.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
