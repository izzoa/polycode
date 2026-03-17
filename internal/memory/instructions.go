package memory

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/izzoa/polycode/internal/config"
)

// defaultInstructions is the built-in system prompt used when no custom
// instructions are provided.
const defaultInstructions = "You are polycode, a helpful coding assistant. Provide clear, concise answers to programming questions. When the user asks you to make changes, use the available tools (file_read, file_write, shell_exec) to interact with their codebase."

// LoadInstructions loads and concatenates instructions from multiple sources:
//  1. Repo-level: .polycode/instructions.md in workDir
//  2. User-level: ~/.config/polycode/instructions.md
//  3. Built-in default
//
// All sources that exist are concatenated, separated by "\n\n".
func LoadInstructions(workDir string) string {
	var parts []string

	// 1. Repo-level instructions.
	repoPath := filepath.Join(workDir, ".polycode", "instructions.md")
	if data, err := os.ReadFile(repoPath); err == nil {
		if content := strings.TrimSpace(string(data)); content != "" {
			parts = append(parts, content)
		}
	}

	// 2. User-level instructions.
	userPath := filepath.Join(config.ConfigDir(), "instructions.md")
	if data, err := os.ReadFile(userPath); err == nil {
		if content := strings.TrimSpace(string(data)); content != "" {
			parts = append(parts, content)
		}
	}

	// 3. Built-in default.
	parts = append(parts, defaultInstructions)

	return strings.Join(parts, "\n\n")
}
