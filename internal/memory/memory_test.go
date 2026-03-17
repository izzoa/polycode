package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- 6.4: LoadInstructions with repo + user + default ---

func TestLoadInstructions_AllSources(t *testing.T) {
	// Set up a temporary config dir.
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	// Create user-level instructions.
	userDir := filepath.Join(tmpHome, "polycode")
	if err := os.MkdirAll(userDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "instructions.md"), []byte("User instructions here."), 0600); err != nil {
		t.Fatal(err)
	}

	// Create repo-level instructions.
	workDir := t.TempDir()
	repoDir := filepath.Join(workDir, ".polycode")
	if err := os.MkdirAll(repoDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "instructions.md"), []byte("Repo instructions here."), 0600); err != nil {
		t.Fatal(err)
	}

	result := LoadInstructions(workDir)

	// Should contain all three sources.
	if !strings.Contains(result, "Repo instructions here.") {
		t.Error("missing repo-level instructions")
	}
	if !strings.Contains(result, "User instructions here.") {
		t.Error("missing user-level instructions")
	}
	if !strings.Contains(result, defaultInstructions) {
		t.Error("missing built-in default instructions")
	}

	// Repo should appear before user.
	repoIdx := strings.Index(result, "Repo instructions here.")
	userIdx := strings.Index(result, "User instructions here.")
	defaultIdx := strings.Index(result, defaultInstructions)

	if repoIdx >= userIdx {
		t.Error("repo instructions should appear before user instructions")
	}
	if userIdx >= defaultIdx {
		t.Error("user instructions should appear before default instructions")
	}
}

func TestLoadInstructions_DefaultOnly(t *testing.T) {
	// Set up a temporary config dir with no instructions file.
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	workDir := t.TempDir()

	result := LoadInstructions(workDir)
	if result != defaultInstructions {
		t.Errorf("expected only default instructions, got: %q", result)
	}
}

func TestLoadInstructions_UserOnly(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	userDir := filepath.Join(tmpHome, "polycode")
	if err := os.MkdirAll(userDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "instructions.md"), []byte("Personal prefs."), 0600); err != nil {
		t.Fatal(err)
	}

	workDir := t.TempDir() // no .polycode directory

	result := LoadInstructions(workDir)
	if !strings.Contains(result, "Personal prefs.") {
		t.Error("missing user-level instructions")
	}
	if !strings.Contains(result, defaultInstructions) {
		t.Error("missing default instructions")
	}
	// Should NOT contain repo instructions.
	parts := strings.Split(result, "\n\n")
	if len(parts) != 2 {
		t.Errorf("expected 2 parts (user + default), got %d", len(parts))
	}
}

// --- 6.5: MemoryStore.Load reads .md files ---

func TestMemoryStore_Load(t *testing.T) {
	dir := t.TempDir()

	// Create some .md files.
	files := map[string]string{
		"build.md":        "Run `go build ./...` to build.",
		"architecture.md": "This is a Go project using Bubble Tea.",
		"notes.txt":       "This should not be loaded.",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	store := NewMemoryStore(dir)
	memories := store.Load()

	if len(memories) != 2 {
		t.Fatalf("Load returned %d entries, want 2", len(memories))
	}

	if memories["build"] != "Run `go build ./...` to build." {
		t.Errorf("build memory = %q, want expected content", memories["build"])
	}
	if memories["architecture"] != "This is a Go project using Bubble Tea." {
		t.Errorf("architecture memory = %q, want expected content", memories["architecture"])
	}

	// .txt file should not be included.
	if _, ok := memories["notes"]; ok {
		t.Error("notes.txt should not be loaded as memory")
	}
}

func TestMemoryStore_Load_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	store := NewMemoryStore(dir)
	memories := store.Load()
	if len(memories) != 0 {
		t.Errorf("expected empty map, got %d entries", len(memories))
	}
}

func TestMemoryStore_Load_NoDir(t *testing.T) {
	store := NewMemoryStore("/nonexistent/path")
	memories := store.Load()
	if len(memories) != 0 {
		t.Errorf("expected empty map for missing dir, got %d entries", len(memories))
	}
}

func TestMemoryStore_SaveAndGet(t *testing.T) {
	dir := t.TempDir()
	store := NewMemoryStore(dir)

	if err := store.Save("conventions", "Use camelCase for variables."); err != nil {
		t.Fatalf("Save: %v", err)
	}

	content, err := store.Get("conventions")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if content != "Use camelCase for variables." {
		t.Errorf("Get = %q, want expected content", content)
	}
}

func TestMemoryStore_Get_NotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewMemoryStore(dir)

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Fatal("Get for missing file should return error")
	}
}

func TestMemoryStore_FormatForPrompt(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "build.md"), []byte("Use make."), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "style.md"), []byte("Tabs not spaces."), 0600); err != nil {
		t.Fatal(err)
	}

	store := NewMemoryStore(dir)
	prompt := store.FormatForPrompt()

	if !strings.Contains(prompt, "## Project Memory") {
		t.Error("missing header")
	}
	if !strings.Contains(prompt, "### build") {
		t.Error("missing build section")
	}
	if !strings.Contains(prompt, "Use make.") {
		t.Error("missing build content")
	}
	if !strings.Contains(prompt, "### style") {
		t.Error("missing style section")
	}
	if !strings.Contains(prompt, "Tabs not spaces.") {
		t.Error("missing style content")
	}
}

func TestMemoryStore_FormatForPrompt_Empty(t *testing.T) {
	dir := t.TempDir()
	store := NewMemoryStore(dir)
	prompt := store.FormatForPrompt()
	if prompt != "" {
		t.Errorf("expected empty prompt, got: %q", prompt)
	}
}
