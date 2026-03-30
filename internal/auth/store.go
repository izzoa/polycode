package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

const keyFileName = ".keyfile"

// encPrefix marks encrypted credential values on disk, distinguishing them
// from legacy plain-base64 values. This prevents the migration path from
// accidentally treating a corrupted encrypted blob as a legacy value.
const encPrefix = "enc:"

// loadOrCreateKey reads (or generates) a 32-byte AES-256 key stored alongside
// the credentials file.
func (f *fileStore) loadOrCreateKey() ([]byte, error) {
	keyPath := filepath.Join(filepath.Dir(f.path), keyFileName)

	key, err := os.ReadFile(keyPath)
	if err == nil {
		if len(key) != 32 {
			return nil, fmt.Errorf("invalid encryption key length %d in %s", len(key), keyPath)
		}
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading encryption key: %w", err)
	}

	// Generate a new 32-byte random key.
	key = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generating encryption key: %w", err)
	}

	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("creating key dir: %w", err)
	}
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return nil, fmt.Errorf("writing encryption key: %w", err)
	}
	return key, nil
}

// encrypt returns base64(nonce || ciphertext || GCM-tag) for the given plaintext.
func (f *fileStore) encrypt(plaintext string) (string, error) {
	key, err := f.loadOrCreateKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// decrypt reverses encrypt. The encoded value must start with encPrefix.
func (f *fileStore) decrypt(encoded string) (string, error) {
	if !strings.HasPrefix(encoded, encPrefix) {
		return "", fmt.Errorf("not an encrypted value")
	}

	key, err := f.loadOrCreateKey()
	if err != nil {
		return "", err
	}

	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, encPrefix))
	if err != nil {
		return "", fmt.Errorf("base64 decoding credential: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// credentialsFile is the on-disk representation. Values are AES-256-GCM
// encrypted and base64-encoded. Legacy base64-only values are migrated
// transparently on first read.
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

	// Encrypted values carry the "enc:" prefix.
	if strings.HasPrefix(encoded, encPrefix) {
		secret, err := f.decrypt(encoded)
		if err != nil {
			return "", fmt.Errorf("decrypting credential for %q: %w", providerName, err)
		}
		return secret, nil
	}

	// No prefix — legacy base64 value. Decode and migrate.
	legacy, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding legacy credential for %q: %w", providerName, err)
	}

	// Re-encrypt and persist. Best-effort; migration retried on next Get.
	reEncrypted, encErr := f.encrypt(string(legacy))
	if encErr == nil {
		creds[providerName] = reEncrypted
		_ = f.save(creds)
	}
	return string(legacy), nil
}

func (f *fileStore) Set(providerName string, secret string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	creds, err := f.load()
	if err != nil {
		return err
	}
	encrypted, err := f.encrypt(secret)
	if err != nil {
		return fmt.Errorf("encrypting credential for %q: %w", providerName, err)
	}
	creds[providerName] = encrypted
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
