package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/sahilm/fuzzy"
)

// fileIndex maintains a searchable list of project files.
type fileIndex struct {
	mu    sync.RWMutex
	files []string // relative paths from project root
	root  string   // project root directory
}

// newFileIndex creates a file index rooted at dir.
func newFileIndex(dir string) *fileIndex {
	idx := &fileIndex{root: dir}
	idx.refresh()
	return idx
}

// refresh rebuilds the file list using git ls-files, falling back to a simple
// walk if the directory is not a git repo. Capped at 10,000 entries.
func (idx *fileIndex) refresh() {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	const maxFiles = 10000

	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = idx.root
	out, err := cmd.Output()
	if err != nil {
		// Not a git repo or git not available — walk the directory.
		idx.files = walkDir(idx.root, maxFiles)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	files := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			files = append(files, l)
		}
	}

	sort.Strings(files)
	if len(files) > maxFiles {
		files = files[:maxFiles]
	}
	idx.files = files
}

// walkDir collects files from dir up to limit entries.
func walkDir(dir string, limit int) []string {
	var files []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == ".cache" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return nil
		}
		files = append(files, rel)
		if len(files) >= limit {
			return filepath.SkipAll
		}
		return nil
	})
	return files
}

// search returns fuzzy-matched files for the given query, up to limit results.
func (idx *fileIndex) search(query string, limit int) []fileMatch {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if query == "" {
		// Return first N files when no query
		n := limit
		if n > len(idx.files) {
			n = len(idx.files)
		}
		result := make([]fileMatch, n)
		for i := 0; i < n; i++ {
			result[i] = fileMatch{
				Path:    idx.files[i],
				Name:    filepath.Base(idx.files[i]),
				Ext:     strings.TrimPrefix(filepath.Ext(idx.files[i]), "."),
				Indices: nil,
			}
		}
		return result
	}

	matches := fuzzy.Find(query, idx.files)
	if len(matches) > limit {
		matches = matches[:limit]
	}

	result := make([]fileMatch, len(matches))
	for i, m := range matches {
		result[i] = fileMatch{
			Path:    m.Str,
			Name:    filepath.Base(m.Str),
			Ext:     strings.TrimPrefix(filepath.Ext(m.Str), "."),
			Indices: m.MatchedIndexes,
			Score:   m.Score,
		}
	}
	return result
}

// fileMatch represents a file matching a fuzzy query.
type fileMatch struct {
	Path    string // relative path from project root
	Name    string // basename
	Ext     string // extension without dot
	Indices []int  // matched character positions in Path
	Score   int    // fuzzy match score
}
