package action

import (
	"fmt"
	"os"
	"path/filepath"
)

// readFile reads the contents of a file and returns them as a ToolResult.
// No confirmation is required for read operations.
func (e *Executor) readFile(path string) ToolResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return ToolResult{
			Error: fmt.Errorf("failed to read file %s: %w", path, err),
		}
	}
	return ToolResult{
		Output: string(data),
	}
}

// writeFile writes content to a file, creating parent directories as needed.
// Requires user confirmation before writing.
func (e *Executor) writeFile(path string, content string) ToolResult {
	// Build a confirmation description showing what will be written.
	preview := content
	if len(preview) > 200 {
		preview = preview[:200] + "... (truncated)"
	}
	description := fmt.Sprintf("Write to file %s:\n%s", path, preview)

	if !e.confirm(description) {
		return ToolResult{
			Output: "file write cancelled by user",
		}
	}

	// Ensure parent directory exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ToolResult{
			Error: fmt.Errorf("failed to create directory %s: %w", dir, err),
		}
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return ToolResult{
			Error: fmt.Errorf("failed to write file %s: %w", path, err),
		}
	}

	return ToolResult{
		Output: fmt.Sprintf("successfully wrote %d bytes to %s", len(content), path),
	}
}
