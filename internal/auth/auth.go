package auth

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const serviceName = "polycode"

// ErrNotFound is returned when a credential does not exist.
var ErrNotFound = errors.New("credential not found")

// Store is the interface for credential storage backends.
type Store interface {
	// Get retrieves a secret for the given provider name.
	Get(providerName string) (string, error)
	// Set stores a secret for the given provider name.
	Set(providerName string, secret string) error
	// Delete removes the secret for the given provider name.
	Delete(providerName string) error
}

// NewStore returns a keyring-backed store. If the system keyring is
// unavailable it falls back to a file-backed store that persists
// base64-encoded credentials to ~/.config/polycode/credentials.json.
func NewStore() Store {
	// Probe the keyring with a harmless get to see if it is functional.
	_, err := keyring.Get(serviceName, "__probe__")
	if err == keyring.ErrNotFound || err == nil {
		return &keyringStore{}
	}
	// Keyring is broken or unavailable — fall back to file store.
	return newFileStore()
}
