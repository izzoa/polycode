package auth

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// MemStore tests
// ---------------------------------------------------------------------------

func TestMemStore_GetMissing(t *testing.T) {
	s := NewMemStore()
	_, err := s.Get("nonexistent")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemStore_SetAndGet(t *testing.T) {
	s := NewMemStore()
	if err := s.Set("openai", "sk-abc123"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	secret, err := s.Get("openai")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if secret != "sk-abc123" {
		t.Fatalf("expected %q, got %q", "sk-abc123", secret)
	}
}

func TestMemStore_Overwrite(t *testing.T) {
	s := NewMemStore()
	if err := s.Set("openai", "old-key"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Set("openai", "new-key"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	secret, err := s.Get("openai")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if secret != "new-key" {
		t.Fatalf("expected %q, got %q", "new-key", secret)
	}
}

func TestMemStore_Delete(t *testing.T) {
	s := NewMemStore()
	if err := s.Set("openai", "sk-abc123"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Delete("openai"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := s.Get("openai")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after Delete, got %v", err)
	}
}

func TestMemStore_DeleteNonExistent(t *testing.T) {
	s := NewMemStore()
	err := s.Delete("nonexistent")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemStore_MultipleKeys(t *testing.T) {
	s := NewMemStore()
	keys := map[string]string{
		"openai":    "sk-openai",
		"anthropic": "sk-anthropic",
		"gemini":    "sk-gemini",
	}
	for k, v := range keys {
		if err := s.Set(k, v); err != nil {
			t.Fatalf("Set(%q): %v", k, err)
		}
	}
	for k, want := range keys {
		got, err := s.Get(k)
		if err != nil {
			t.Fatalf("Get(%q): %v", k, err)
		}
		if got != want {
			t.Fatalf("Get(%q) = %q, want %q", k, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// fileStore tests
// ---------------------------------------------------------------------------

func newTestFileStore(t *testing.T) *fileStore {
	t.Helper()
	dir := t.TempDir()
	return &fileStore{
		path: filepath.Join(dir, "credentials.json"),
	}
}

func TestFileStore_GetMissing(t *testing.T) {
	s := newTestFileStore(t)
	_, err := s.Get("nonexistent")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFileStore_SetAndGet(t *testing.T) {
	s := newTestFileStore(t)
	if err := s.Set("anthropic", "sk-ant-secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	secret, err := s.Get("anthropic")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if secret != "sk-ant-secret" {
		t.Fatalf("expected %q, got %q", "sk-ant-secret", secret)
	}
}

func TestFileStore_Overwrite(t *testing.T) {
	s := newTestFileStore(t)
	if err := s.Set("openai", "old-key"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Set("openai", "new-key"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	secret, err := s.Get("openai")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if secret != "new-key" {
		t.Fatalf("expected %q, got %q", "new-key", secret)
	}
}

func TestFileStore_Delete(t *testing.T) {
	s := newTestFileStore(t)
	if err := s.Set("gemini", "gm-key"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Delete("gemini"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := s.Get("gemini")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after Delete, got %v", err)
	}
}

func TestFileStore_DeleteNonExistent(t *testing.T) {
	s := newTestFileStore(t)
	err := s.Delete("nonexistent")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFileStore_LoadNonExistentFile(t *testing.T) {
	s := newTestFileStore(t)
	// The file doesn't exist yet; load should return an empty map, not error.
	creds, err := s.load()
	if err != nil {
		t.Fatalf("load from non-existent file: %v", err)
	}
	if len(creds) != 0 {
		t.Fatalf("expected empty credentials, got %v", creds)
	}
}

func TestFileStore_RawFileContent(t *testing.T) {
	s := newTestFileStore(t)

	if err := s.Set("openai", "sk-raw-test"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Read the raw file and verify it contains encrypted data.
	data, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatalf("reading raw file: %v", err)
	}

	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parsing raw JSON: %v", err)
	}

	encoded, ok := raw["openai"]
	if !ok {
		t.Fatal("expected 'openai' key in raw JSON")
	}

	// The raw value must carry the "enc:" prefix.
	if !strings.HasPrefix(encoded, "enc:") {
		t.Fatal("raw value missing enc: prefix — expected encrypted")
	}

	// The base64 payload should have enough bytes for nonce + ciphertext + tag.
	blob, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, "enc:"))
	if err != nil {
		t.Fatalf("base64 decode of encrypted value: %v", err)
	}
	// AES-GCM: 12-byte nonce + 16-byte tag + at least 1 byte ciphertext = 29 min.
	if len(blob) < 29 {
		t.Fatalf("encrypted blob too short: %d bytes", len(blob))
	}

	// Verify round-trip via Get.
	got, err := s.Get("openai")
	if err != nil {
		t.Fatalf("Get after encrypted Set: %v", err)
	}
	if got != "sk-raw-test" {
		t.Fatalf("expected %q, got %q", "sk-raw-test", got)
	}
}

func TestFileStore_FilePermissions(t *testing.T) {
	s := newTestFileStore(t)
	if err := s.Set("perm-test", "secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	info, err := os.Stat(s.path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Fatalf("expected file permissions 0600, got %04o", perm)
	}
}

func TestFileStore_MultipleKeys(t *testing.T) {
	s := newTestFileStore(t)
	keys := map[string]string{
		"openai":    "sk-openai",
		"anthropic": "sk-anthropic",
		"gemini":    "sk-gemini",
	}
	for k, v := range keys {
		if err := s.Set(k, v); err != nil {
			t.Fatalf("Set(%q): %v", k, err)
		}
	}
	for k, want := range keys {
		got, err := s.Get(k)
		if err != nil {
			t.Fatalf("Get(%q): %v", k, err)
		}
		if got != want {
			t.Fatalf("Get(%q) = %q, want %q", k, got, want)
		}
	}
}

func TestFileStore_DeleteDoesNotAffectOtherKeys(t *testing.T) {
	s := newTestFileStore(t)
	if err := s.Set("keep", "keep-val"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Set("remove", "remove-val"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.Delete("remove"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// "keep" should still be present.
	got, err := s.Get("keep")
	if err != nil {
		t.Fatalf("Get(keep): %v", err)
	}
	if got != "keep-val" {
		t.Fatalf("expected %q, got %q", "keep-val", got)
	}
	// "remove" should be gone.
	_, err = s.Get("remove")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for removed key, got %v", err)
	}
}

func TestFileStore_ConcurrentAccess(t *testing.T) {
	s := newTestFileStore(t)
	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			key := "provider"
			val := "secret"

			// Each goroutine does a set then get cycle.
			if err := s.Set(key, val); err != nil {
				t.Errorf("goroutine %d Set: %v", n, err)
				return
			}
			got, err := s.Get(key)
			if err != nil {
				t.Errorf("goroutine %d Get: %v", n, err)
				return
			}
			if got != val {
				t.Errorf("goroutine %d: got %q, want %q", n, got, val)
			}
		}(i)
	}
	wg.Wait()
}

func TestFileStore_KeyFileCreation(t *testing.T) {
	s := newTestFileStore(t)

	keyPath := filepath.Join(filepath.Dir(s.path), ".keyfile")
	if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
		t.Fatal("keyfile should not exist before first use")
	}

	if err := s.Set("test", "secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("Stat keyfile: %v", err)
	}
	if info.Size() != 32 {
		t.Fatalf("expected 32-byte key, got %d bytes", info.Size())
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("expected keyfile permissions 0600, got %04o", perm)
	}
}

func TestFileStore_KeyFileReuse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")

	s1 := &fileStore{path: path}
	if err := s1.Set("provider", "my-secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// A second fileStore instance on the same dir should decrypt successfully.
	s2 := &fileStore{path: path}
	got, err := s2.Get("provider")
	if err != nil {
		t.Fatalf("Get from second instance: %v", err)
	}
	if got != "my-secret" {
		t.Fatalf("expected %q, got %q", "my-secret", got)
	}
}

func TestFileStore_MigrationFromBase64(t *testing.T) {
	s := newTestFileStore(t)

	// Write a legacy base64-encoded credentials file.
	legacy := map[string]string{
		"openai": base64.StdEncoding.EncodeToString([]byte("sk-legacy-key")),
	}
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(s.path, data, 0600); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	// Get should return the legacy value via migration.
	got, err := s.Get("openai")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "sk-legacy-key" {
		t.Fatalf("expected %q, got %q", "sk-legacy-key", got)
	}

	// The file should now be re-encrypted.
	rawData, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatalf("reading raw file: %v", err)
	}
	var raw map[string]string
	if err := json.Unmarshal(rawData, &raw); err != nil {
		t.Fatalf("parsing raw JSON: %v", err)
	}
	if !strings.HasPrefix(raw["openai"], "enc:") {
		t.Fatal("credential was not re-encrypted after migration")
	}

	// A subsequent Get should work (now decrypting the re-encrypted value).
	got2, err := s.Get("openai")
	if err != nil {
		t.Fatalf("Get after migration: %v", err)
	}
	if got2 != "sk-legacy-key" {
		t.Fatalf("expected %q after migration, got %q", "sk-legacy-key", got2)
	}
}

func TestFileStore_MigrationMultipleKeys(t *testing.T) {
	s := newTestFileStore(t)

	legacy := map[string]string{
		"openai":    base64.StdEncoding.EncodeToString([]byte("sk-openai")),
		"anthropic": base64.StdEncoding.EncodeToString([]byte("sk-anthropic")),
	}
	data, _ := json.MarshalIndent(legacy, "", "  ")
	os.MkdirAll(filepath.Dir(s.path), 0700)
	os.WriteFile(s.path, data, 0600)

	for name, want := range map[string]string{"openai": "sk-openai", "anthropic": "sk-anthropic"} {
		got, err := s.Get(name)
		if err != nil {
			t.Fatalf("Get(%q): %v", name, err)
		}
		if got != want {
			t.Fatalf("Get(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestFileStore_EncryptDecryptRoundTrip(t *testing.T) {
	s := newTestFileStore(t)

	cases := []string{
		"simple-key",
		"",
		"sk-ant-api03-very-long-key-with-special-chars!@#$%^&*()",
		"unicode: \u3053\u3093\u306b\u3061\u306f\u4e16\u754c",
	}
	for _, want := range cases {
		encrypted, err := s.encrypt(want)
		if err != nil {
			t.Fatalf("encrypt(%q): %v", want, err)
		}
		got, err := s.decrypt(encrypted)
		if err != nil {
			t.Fatalf("decrypt(%q): %v", want, err)
		}
		if got != want {
			t.Fatalf("round-trip failed: got %q, want %q", got, want)
		}
	}
}

func TestFileStore_EncryptProducesUniqueOutput(t *testing.T) {
	s := newTestFileStore(t)

	enc1, err := s.encrypt("same-secret")
	if err != nil {
		t.Fatalf("encrypt 1: %v", err)
	}
	enc2, err := s.encrypt("same-secret")
	if err != nil {
		t.Fatalf("encrypt 2: %v", err)
	}
	if enc1 == enc2 {
		t.Fatal("two encryptions of the same plaintext produced identical ciphertext")
	}
}

// ---------------------------------------------------------------------------
// NewStore tests
// ---------------------------------------------------------------------------

func TestNewStore_ReturnsNonNil(t *testing.T) {
	s := NewStore()
	if s == nil {
		t.Fatal("NewStore returned nil")
	}
}
