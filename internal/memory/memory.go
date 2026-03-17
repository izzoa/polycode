package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MemoryStore manages persistent memory files stored as markdown.
type MemoryStore struct {
	dir string
}

// NewMemoryStore creates a MemoryStore that reads from and writes to memDir.
func NewMemoryStore(memDir string) *MemoryStore {
	return &MemoryStore{dir: memDir}
}

// Load reads all .md files from the memory directory and returns a map of
// name (without extension) to file content. Returns an empty map if the
// directory does not exist.
func (s *MemoryStore) Load() map[string]string {
	result := make(map[string]string)

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		path := filepath.Join(s.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		result[name] = string(data)
	}

	return result
}

// Get reads a single memory file by name (without the .md extension).
func (s *MemoryStore) Get(name string) (string, error) {
	path := filepath.Join(s.dir, name+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading memory %q: %w", name, err)
	}
	return string(data), nil
}

// Save writes content to a memory file with the given name (the .md extension
// is appended automatically). It creates the memory directory if it does not
// exist.
func (s *MemoryStore) Save(name, content string) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return fmt.Errorf("creating memory dir: %w", err)
	}

	path := filepath.Join(s.dir, name+".md")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing memory %q: %w", name, err)
	}
	return nil
}

// FormatForPrompt formats all loaded memory files into a single string
// suitable for inclusion in a system prompt.
func (s *MemoryStore) FormatForPrompt() string {
	memories := s.Load()
	if len(memories) == 0 {
		return ""
	}

	// Sort names for deterministic output.
	names := make([]string, 0, len(memories))
	for name := range memories {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("## Project Memory\n\n")
	for _, name := range names {
		b.WriteString("### ")
		b.WriteString(name)
		b.WriteString("\n\n")
		b.WriteString(strings.TrimSpace(memories[name]))
		b.WriteString("\n\n")
	}

	return strings.TrimRight(b.String(), "\n")
}
