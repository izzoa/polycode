package action

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validatePath cleans a path and validates it against traversal attacks.
// Relative paths are resolved against the working directory and must stay
// within it. Absolute paths are allowed but require user confirmation
// through the normal confirm flow.
func validatePath(path string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine working directory: %w", err)
	}

	// Resolve relative paths against the working directory.
	abs := path
	if !filepath.IsAbs(path) {
		abs = filepath.Join(wd, path)
	}
	abs = filepath.Clean(abs)

	// Block relative paths that escape the working directory via traversal.
	if !filepath.IsAbs(path) {
		if !strings.HasPrefix(abs, wd+string(filepath.Separator)) && abs != wd {
			return "", fmt.Errorf("path %q escapes the working directory", path)
		}
	}

	return abs, nil
}

// readFile reads the contents of a file and returns them as a ToolResult.
// If the path is a directory, it returns a listing of its contents.
// No confirmation is required for read operations.
func (e *Executor) readFile(path string) ToolResult {
	cleanPath, err := validatePath(path)
	if err != nil {
		return ToolResult{Error: err}
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return ToolResult{
			Error: fmt.Errorf("failed to read file %s: %w", path, err),
		}
	}

	// If path is a directory, return a listing instead of an error.
	// Only allow directory listing within the working directory to prevent
	// filesystem reconnaissance on sensitive paths.
	if info.IsDir() {
		wd, wdErr := os.Getwd()
		if wdErr != nil {
			return ToolResult{Error: fmt.Errorf("cannot determine working directory: %w", wdErr)}
		}
		if !strings.HasPrefix(cleanPath, wd+string(filepath.Separator)) && cleanPath != wd {
			return ToolResult{
				Error: fmt.Errorf("file_read: %s is a directory (directory listing only allowed within the project)", path),
			}
		}
		entries, dirErr := os.ReadDir(cleanPath)
		if dirErr != nil {
			return ToolResult{
				Error: fmt.Errorf("failed to list directory %s: %w", path, dirErr),
			}
		}
		var listing strings.Builder
		fmt.Fprintf(&listing, "Directory listing of %s:\n", path)
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				name += "/"
			}
			listing.WriteString(name + "\n")
		}
		return ToolResult{Output: listing.String()}
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return ToolResult{
			Error: fmt.Errorf("failed to read file %s: %w", path, err),
		}
	}
	return ToolResult{
		Output: string(data),
	}
}

// unifiedDiff computes a simple unified diff between old and new content.
// Returns a human-readable string with + and - prefixed lines.
func unifiedDiff(oldContent, newContent string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var b strings.Builder
	// Simple line-by-line comparison. Walk both arrays with a basic LCS approach
	// limited to a context window for readability.
	oi, ni := 0, 0
	for oi < len(oldLines) || ni < len(newLines) {
		if oi < len(oldLines) && ni < len(newLines) && oldLines[oi] == newLines[ni] {
			oi++
			ni++
			continue
		}
		// Find the next matching line pair to re-sync.
		foundOld, foundNew := -1, -1
		for look := 1; look < 6; look++ {
			if oi+look < len(oldLines) && ni < len(newLines) && oldLines[oi+look] == newLines[ni] {
				foundOld = look
				break
			}
			if ni+look < len(newLines) && oi < len(oldLines) && newLines[ni+look] == oldLines[oi] {
				foundNew = look
				break
			}
		}
		if foundOld > 0 {
			for k := 0; k < foundOld; k++ {
				fmt.Fprintf(&b, "- %s\n", oldLines[oi+k])
			}
			oi += foundOld
		} else if foundNew > 0 {
			for k := 0; k < foundNew; k++ {
				fmt.Fprintf(&b, "+ %s\n", newLines[ni+k])
			}
			ni += foundNew
		} else {
			// No match nearby — emit both as changed.
			if oi < len(oldLines) {
				fmt.Fprintf(&b, "- %s\n", oldLines[oi])
				oi++
			}
			if ni < len(newLines) {
				fmt.Fprintf(&b, "+ %s\n", newLines[ni])
				ni++
			}
		}
	}
	return b.String()
}

// writeFile writes content to a file, creating parent directories as needed.
// Requires user confirmation before writing.
func (e *Executor) writeFile(path string, content string) ToolResult {
	cleanPath, err := validatePath(path)
	if err != nil {
		return ToolResult{Error: err}
	}

	// Build a confirmation description. If the file already exists, show a
	// unified diff; otherwise show a content preview (new file).
	var description string
	existing, readErr := os.ReadFile(cleanPath)
	if readErr == nil {
		diff := unifiedDiff(string(existing), content)
		if diff == "" {
			description = fmt.Sprintf("Write to file %s (no changes)", path)
		} else {
			if len(diff) > 800 {
				diff = diff[:800] + "\n... (diff truncated)"
			}
			description = fmt.Sprintf("Write to file %s:\n%s", path, diff)
		}
	} else {
		preview := content
		if len(preview) > 200 {
			preview = preview[:200] + "... (truncated)"
		}
		description = fmt.Sprintf("Create new file %s:\n%s", path, preview)
	}

	if e.confirm == nil {
		return ToolResult{
			Error: fmt.Errorf("file_write: no confirmation callback configured"),
		}
	}
	if !e.confirm("file_write", description) {
		return ToolResult{
			Output: "file write cancelled by user",
		}
	}

	// Ensure parent directory exists.
	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ToolResult{
			Error: fmt.Errorf("failed to create directory %s: %w", dir, err),
		}
	}

	if err := os.WriteFile(cleanPath, []byte(content), 0o644); err != nil {
		return ToolResult{
			Error: fmt.Errorf("failed to write file %s: %w", path, err),
		}
	}

	return ToolResult{
		Output: fmt.Sprintf("successfully wrote %d bytes to %s", len(content), path),
	}
}
