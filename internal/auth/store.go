package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/izzoa/polycode/internal/config"
	"github.com/zalando/go-keyring"
)

// ---------------------------------------------------------------------------
// Keyring-backed store
// ---------------------------------------------------------------------------

type keyringStore struct{}

func (k *keyringStore) Get(providerName string) (string, error) {
	secret, err := keyring.Get(serviceName, providerName)
	if err != nil {
		if err == keyring.ErrNotFound {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("keyring get %q: %w", providerName, err)
	}
	return secret, nil
}

func (k *keyringStore) Set(providerName string, secret string) error {
	if err := keyring.Set(serviceName, providerName, secret); err != nil {
		return fmt.Errorf("keyring set %q: %w", providerName, err)
	}
	return nil
}

func (k *keyringStore) Delete(providerName string) error {
	if err := keyring.Delete(serviceName, providerName); err != nil {
		if err == keyring.ErrNotFound {
			return ErrNotFound
		}
		return fmt.Errorf("keyring delete %q: %w", providerName, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// File-backed store (fallback)
// ---------------------------------------------------------------------------

type fileStore struct {
	path string
	mu   sync.Mutex
}

func newFileStore() *fileStore {
	return &fileStore{
		path: filepath.Join(config.ConfigDir(), "credentials.json"),
	}
}

// credentialsFile is the on-disk representation. Values are base64-encoded.
type credentialsFile map[string]string

func (f *fileStore) load() (credentialsFile, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(credentialsFile), nil
		}
		return nil, fmt.Errorf("reading credentials file: %w", err)
	}
	var creds credentialsFile
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parsing credentials file: %w", err)
	}
	return creds, nil
}

func (f *fileStore) save(creds credentialsFile) error {
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating credentials dir: %w", err)
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling credentials: %w", err)
	}
	return os.WriteFile(f.path, data, 0600)
}

func (f *fileStore) Get(providerName string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	creds, err := f.load()
	if err != nil {
		return "", err
	}
	encoded, ok := creds[providerName]
	if !ok {
		return "", ErrNotFound
	}
	secret, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding credential for %q: %w", providerName, err)
	}
	return string(secret), nil
}

func (f *fileStore) Set(providerName string, secret string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	creds, err := f.load()
	if err != nil {
		return err
	}
	creds[providerName] = base64.StdEncoding.EncodeToString([]byte(secret))
	return f.save(creds)
}

func (f *fileStore) Delete(providerName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	creds, err := f.load()
	if err != nil {
		return err
	}
	if _, ok := creds[providerName]; !ok {
		return ErrNotFound
	}
	delete(creds, providerName)
	return f.save(creds)
}

// ---------------------------------------------------------------------------
// In-memory store (for testing and ephemeral use)
// ---------------------------------------------------------------------------

// MemStore is an in-memory credential store useful for testing and
// ephemeral operations like the CLI wizard's connection test.
type MemStore struct {
	mu      sync.Mutex
	secrets map[string]string
}

// NewMemStore creates a new in-memory credential store.
func NewMemStore() *MemStore {
	return &MemStore{secrets: make(map[string]string)}
}

func (m *MemStore) Get(providerName string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.secrets[providerName]
	if !ok {
		return "", ErrNotFound
	}
	return s, nil
}

func (m *MemStore) Set(providerName string, secret string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.secrets[providerName] = secret
	return nil
}

func (m *MemStore) Delete(providerName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.secrets[providerName]; !ok {
		return ErrNotFound
	}
	delete(m.secrets, providerName)
	return nil
}
