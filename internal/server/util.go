package server

import (
	"crypto/rand"
	"encoding/base64"
)

// randomGuestID returns a short, URL-safe random token used when a guest
// upload arrives without a valid client-supplied id.
func randomGuestID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
