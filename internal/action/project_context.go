package action

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// projectFile defines a well-known project file and its significance.
type projectFile struct {
	name string
	desc string
}

var keyProjectFiles = []projectFile{
	{"go.mod", "Go module"},
	{"package.json", "Node.js project"},
	{"Cargo.toml", "Rust project"},
	{"pyproject.toml", "Python project"},
	{"requirements.txt", "Python dependencies"},
	{"Makefile", "Makefile"},
	{"Dockerfile", "Docker"},
	{"docker-compose.yml", "Docker Compose"},
	{"README.md", "README"},
	{"CLAUDE.md", "AI instructions"},
	{".gitignore", "Git ignore rules"},
}

// BuildProjectContext generates a snapshot of the project structure and key
// files for inclusion in the system prompt. This gives providers immediate
// context without needing a tool round to explore.
func BuildProjectContext(workDir string) string {
	var b strings.Builder

	b.WriteString("## Project Context\n\n")

	// Detect project type from key files.
	var detected []string
	for _, kf := range keyProjectFiles {
		if _, err := os.Stat(filepath.Join(workDir, kf.name)); err == nil {
			detected = append(detected, fmt.Sprintf("%s (%s)", kf.name, kf.desc))
		}
	}
	if len(detected) > 0 {
		b.WriteString("**Project type:** ")
		b.WriteString(strings.Join(detected, ", "))
		b.WriteString("\n\n")
	}

	// Top-level directory listing.
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return b.String()
	}

	b.WriteString("**Project root (`./`):**\n```\n")
	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden files except key ones.
		if strings.HasPrefix(name, ".") {
			switch name {
			case ".gitignore", ".github", ".env.example":
				// keep
			default:
				continue
			}
		}
		if entry.IsDir() {
			name += "/"
		}
		b.WriteString(name + "\n")
	}
	b.WriteString("```\n")

	return b.String()
}

// ToolUsageHints returns guidance text for providers on how to use the
// available read-only tools effectively during fan-out.
func ToolUsageHints() string {
	return `## Available Tools

You have the following read-only tools for exploring this codebase:

- **file_read** — Read a file's contents. Pass a directory path to get its listing. Use "." for the project root.
- **list_directory** — List directory contents. Set recursive=true to see up to 3 levels deep. Use "." for the project root.
- **grep_search** — Search for text/regex patterns across files. Supports file type filtering with the "include" parameter (e.g., "*.go").

**Tips for efficient exploration:**
1. Start by reading key files like README.md, go.mod, or the main entry point.
2. Use grep_search to find specific functions, types, or patterns.
3. Use list_directory with recursive=true on specific subdirectories, not the whole project.
4. Do NOT attempt to use shell_exec, file_write, or any tools not listed above — they are not available to you.`
}
