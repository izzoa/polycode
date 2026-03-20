package auth

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
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

	// Read the raw file and verify JSON structure with base64 values.
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

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if string(decoded) != "sk-raw-test" {
		t.Fatalf("raw file value decoded to %q, want %q", string(decoded), "sk-raw-test")
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

// ---------------------------------------------------------------------------
// NewStore tests
// ---------------------------------------------------------------------------

func TestNewStore_ReturnsNonNil(t *testing.T) {
	s := NewStore()
	if s == nil {
		t.Fatal("NewStore returned nil")
	}
}
