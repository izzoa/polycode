package action

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/izzoa/polycode/internal/provider"
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
// startLine and endLine are 1-based inclusive; pass 0 to omit.
func (e *Executor) readFile(path string, startLine, endLine int) ToolResult {
	cleanPath, err := validatePath(path)
	if err != nil {
		return ToolResult{Error: err}
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Suggest alternatives: list the parent directory if it exists.
			dir := filepath.Dir(cleanPath)
			hint := ""
			if entries, dirErr := os.ReadDir(dir); dirErr == nil {
				var names []string
				for _, entry := range entries {
					n := entry.Name()
					if entry.IsDir() {
						n += "/"
					}
					names = append(names, n)
					if len(names) >= 10 {
						names = append(names, "...")
						break
					}
				}
				hint = fmt.Sprintf("\nAvailable in %s/: %s", filepath.Base(dir), strings.Join(names, ", "))
			}
			return ToolResult{
				Error: fmt.Errorf("file_read: %q not found.%s\nHint: use list_directory to see directory contents, or grep_search to find files by content.", path, hint),
			}
		}
		return ToolResult{
			Error: fmt.Errorf("file_read: cannot access %q: %w", path, err),
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

	content := string(data)

	// Apply line range if requested.
	if startLine > 0 || endLine > 0 {
		lines := strings.Split(content, "\n")
		// Drop trailing empty element from newline-terminated files.
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		total := len(lines)

		if startLine < 1 {
			startLine = 1
		}
		if endLine < 1 || endLine > total {
			endLine = total
		}
		if startLine > total {
			return ToolResult{Output: fmt.Sprintf("(file has %d lines, start_line %d is past end)", total, startLine)}
		}
		if startLine > endLine {
			return ToolResult{Output: fmt.Sprintf("(invalid range: start_line %d > end_line %d)", startLine, endLine)}
		}

		selected := lines[startLine-1 : endLine]
		var b strings.Builder
		fmt.Fprintf(&b, "Lines %d-%d of %d in %s:\n", startLine, endLine, total, path)
		for i, line := range selected {
			fmt.Fprintf(&b, "%d\t%s\n", startLine+i, line)
		}
		return ToolResult{Output: b.String()}
	}

	return ToolResult{
		Output: content,
	}
}

func (e *Executor) executeFileEdit(call provider.ToolCall) ToolResult {
	var args struct {
		Path       string `json:"path"`
		OldText    string `json:"old_text"`
		NewText    string `json:"new_text"`
		ReplaceAll bool   `json:"replace_all"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("invalid arguments for file_edit: %w", err)}
	}
	if args.Path == "" {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_edit: path is required")}
	}
	if args.OldText == "" {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_edit: old_text is required")}
	}
	if args.OldText == args.NewText {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_edit: old_text and new_text are identical")}
	}
	result := e.editFile(args.Path, args.OldText, args.NewText, args.ReplaceAll)
	result.ToolCallID = call.ID
	return result
}

func (e *Executor) editFile(path, oldText, newText string, replaceAll bool) ToolResult {
	cleanPath, err := validatePath(path)
	if err != nil {
		return ToolResult{Error: fmt.Errorf("file_edit: %w", err)}
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ToolResult{Error: fmt.Errorf("file_edit: %q not found", path)}
		}
		return ToolResult{Error: fmt.Errorf("file_edit: cannot read %q: %w", path, err)}
	}
	content := string(data)

	// Count occurrences.
	count := strings.Count(content, oldText)
	if count == 0 {
		return ToolResult{Error: fmt.Errorf("file_edit: old_text not found in %s", path)}
	}
	if count > 1 && !replaceAll {
		return ToolResult{Error: fmt.Errorf("file_edit: old_text matches %d locations in %s — provide more context to make it unique, or set replace_all=true", count, path)}
	}

	// Build new content.
	var newContent string
	if replaceAll {
		newContent = strings.ReplaceAll(content, oldText, newText)
	} else {
		newContent = strings.Replace(content, oldText, newText, 1)
	}

	// Confirmation with diff preview.
	diff := unifiedDiff(content, newContent)
	if len(diff) > 800 {
		diff = diff[:800] + "\n... (diff truncated)"
	}
	description := fmt.Sprintf("Edit file %s (%d replacement(s)):\n%s", path, count, diff)

	if e.confirm == nil {
		return ToolResult{Error: fmt.Errorf("file_edit: no confirmation callback configured")}
	}
	if !e.confirm("file_edit", description) {
		return ToolResult{Output: "file edit cancelled by user"}
	}

	if err := os.WriteFile(cleanPath, []byte(newContent), 0o644); err != nil {
		return ToolResult{Error: fmt.Errorf("file_edit: failed to write %s: %w", path, err)}
	}

	replacements := "1 replacement"
	if count > 1 {
		replacements = fmt.Sprintf("%d replacements", count)
	}
	return ToolResult{Output: fmt.Sprintf("successfully applied %s in %s", replacements, path)}
}

func (e *Executor) executeFileDelete(call provider.ToolCall) ToolResult {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("invalid arguments for file_delete: %w", err)}
	}
	if args.Path == "" {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_delete: path is required")}
	}

	cleanPath, err := validatePath(args.Path)
	if err != nil {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_delete: %w", err)}
	}

	// Use Lstat to inspect the path entry itself (not the symlink target).
	info, err := os.Lstat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_delete: %q not found", args.Path)}
		}
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_delete: %w", err)}
	}

	// Build confirmation description.
	var description string
	if info.Mode()&os.ModeSymlink != 0 {
		target, _ := os.Readlink(cleanPath)
		description = fmt.Sprintf("Delete symlink: %s -> %s", args.Path, target)
	} else if info.IsDir() {
		entries, _ := os.ReadDir(cleanPath)
		if len(entries) > 0 {
			return ToolResult{
				ToolCallID: call.ID,
				Error: fmt.Errorf("file_delete: directory %q is not empty (%d entries) — only empty directories can be deleted with this tool", args.Path, len(entries)),
			}
		}
		description = fmt.Sprintf("Delete empty directory: %s", args.Path)
	} else {
		description = fmt.Sprintf("Delete file: %s (%d bytes)", args.Path, info.Size())
	}

	if e.confirm == nil {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_delete: no confirmation callback configured")}
	}
	if !e.confirm("file_delete", description) {
		return ToolResult{ToolCallID: call.ID, Output: "file delete cancelled by user"}
	}

	if err := os.Remove(cleanPath); err != nil {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_delete: %w", err)}
	}

	return ToolResult{ToolCallID: call.ID, Output: fmt.Sprintf("deleted %s", args.Path)}
}

func (e *Executor) executeFileRename(call provider.ToolCall) ToolResult {
	var args struct {
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("invalid arguments for file_rename: %w", err)}
	}
	if args.OldPath == "" || args.NewPath == "" {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_rename: both old_path and new_path are required")}
	}

	cleanOld, err := validatePath(args.OldPath)
	if err != nil {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_rename: %w", err)}
	}
	cleanNew, err := validatePath(args.NewPath)
	if err != nil {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_rename: %w", err)}
	}

	// Use Lstat to check path entries themselves, not symlink targets.
	// Source must exist.
	if _, err := os.Lstat(cleanOld); err != nil {
		if os.IsNotExist(err) {
			return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_rename: %q not found", args.OldPath)}
		}
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_rename: %w", err)}
	}

	// Destination must not exist (prevent accidental overwrites).
	if _, err := os.Lstat(cleanNew); err == nil {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_rename: destination %q already exists", args.NewPath)}
	}

	// Reject renaming a path into its own subtree (e.g. dir -> dir/sub/dst)
	// to prevent MkdirAll from creating directories inside the source before
	// the rename, which would leave side effects on failure.
	if strings.HasPrefix(cleanNew, cleanOld+string(filepath.Separator)) {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_rename: destination %q is inside source %q", args.NewPath, args.OldPath)}
	}

	description := fmt.Sprintf("Rename: %s -> %s", args.OldPath, args.NewPath)

	if e.confirm == nil {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_rename: no confirmation callback configured")}
	}
	if !e.confirm("file_rename", description) {
		return ToolResult{ToolCallID: call.ID, Output: "file rename cancelled by user"}
	}

	// Ensure destination parent directory exists.
	destDir := filepath.Dir(cleanNew)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_rename: failed to create directory %s: %w", destDir, err)}
	}

	if err := os.Rename(cleanOld, cleanNew); err != nil {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_rename: %w", err)}
	}

	return ToolResult{ToolCallID: call.ID, Output: fmt.Sprintf("renamed %s -> %s", args.OldPath, args.NewPath)}
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

func (e *Executor) executeListDirectory(call provider.ToolCall) ToolResult {
	var args struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Errorf("invalid arguments for list_directory: %w", err),
		}
	}
	// Normalize empty or garbage paths to project root.
	args.Path = strings.TrimSpace(args.Path)
	if args.Path == "" || args.Path == ":" {
		args.Path = "."
	}

	cleanPath, err := validatePath(args.Path)
	if err != nil {
		return ToolResult{ToolCallID: call.ID, Error: err}
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ToolResult{
				ToolCallID: call.ID,
				Error:      fmt.Errorf("list_directory: %q not found. Use '.' for the project root.", args.Path),
			}
		}
		return ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Errorf("list_directory: cannot access %q: %w", args.Path, err),
		}
	}
	if !info.IsDir() {
		return ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Errorf("list_directory: %s is not a directory", args.Path),
		}
	}

	// Restrict to working directory.
	wd, _ := os.Getwd()
	if !strings.HasPrefix(cleanPath, wd+string(filepath.Separator)) && cleanPath != wd {
		return ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Errorf("list_directory: path outside project directory"),
		}
	}

	var listing strings.Builder
	if args.Recursive {
		maxDepth := 3
		baseDepth := strings.Count(cleanPath, string(filepath.Separator))
		_ = filepath.Walk(cleanPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // skip errors
			}
			depth := strings.Count(path, string(filepath.Separator)) - baseDepth
			if depth > maxDepth {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			rel, _ := filepath.Rel(cleanPath, path)
			if rel == "." {
				return nil
			}
			// Skip hidden directories like .git
			if info.IsDir() && strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			indent := strings.Repeat("  ", depth)
			name := info.Name()
			if info.IsDir() {
				name += "/"
			}
			fmt.Fprintf(&listing, "%s%s\n", indent, name)
			return nil
		})
	} else {
		entries, dirErr := os.ReadDir(cleanPath)
		if dirErr != nil {
			return ToolResult{
				ToolCallID: call.ID,
				Error:      fmt.Errorf("list_directory: %w", dirErr),
			}
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				name += "/"
			}
			listing.WriteString(name + "\n")
		}
	}

	result := listing.String()
	if result == "" {
		result = "(empty directory)"
	}
	return ToolResult{ToolCallID: call.ID, Output: result}
}

func (e *Executor) executeFindFiles(call provider.ToolCall) ToolResult {
	var args struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("invalid arguments for find_files: %w", err)}
	}
	if args.Pattern == "" {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("find_files: pattern is required")}
	}
	if args.Path == "" {
		args.Path = "."
	}

	cleanPath, err := validatePath(args.Path)
	if err != nil {
		return ToolResult{ToolCallID: call.ID, Error: err}
	}

	// Restrict to working directory.
	wd, _ := os.Getwd()
	if !strings.HasPrefix(cleanPath, wd+string(filepath.Separator)) && cleanPath != wd {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("find_files: path outside project directory")}
	}

	var matches []string
	maxResults := 200
	truncated := false

	isRecursive := strings.Contains(args.Pattern, "**")

	_ = filepath.Walk(cleanPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden directories.
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		if len(matches) >= maxResults {
			truncated = true
			return filepath.SkipAll
		}

		rel, _ := filepath.Rel(wd, path)

		if isRecursive {
			parts := strings.Split(args.Pattern, "/")
			filePattern := parts[len(parts)-1]
			if matched, _ := filepath.Match(filePattern, info.Name()); matched {
				matches = append(matches, rel)
			}
		} else {
			if matched, _ := filepath.Match(args.Pattern, info.Name()); matched {
				matches = append(matches, rel)
			}
		}
		return nil
	})

	if len(matches) == 0 {
		return ToolResult{ToolCallID: call.ID, Output: fmt.Sprintf("No files found matching %q", args.Pattern)}
	}

	output := strings.Join(matches, "\n")
	if truncated {
		output += fmt.Sprintf("\n... (truncated at %d results)", maxResults)
	}
	return ToolResult{ToolCallID: call.ID, Output: output}
}

func (e *Executor) executeFileInfo(call provider.ToolCall) ToolResult {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("invalid arguments for file_info: %w", err)}
	}
	if args.Path == "" {
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_info: path is required")}
	}

	cleanPath, err := validatePath(args.Path)
	if err != nil {
		return ToolResult{ToolCallID: call.ID, Error: err}
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ToolResult{ToolCallID: call.ID, Output: fmt.Sprintf("not found: %s", args.Path)}
		}
		return ToolResult{ToolCallID: call.ID, Error: fmt.Errorf("file_info: %w", err)}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Path: %s\n", args.Path)
	if info.IsDir() {
		fmt.Fprintf(&b, "Type: directory\n")
		entries, _ := os.ReadDir(cleanPath)
		fmt.Fprintf(&b, "Entries: %d\n", len(entries))
	} else {
		fmt.Fprintf(&b, "Type: file\n")
		fmt.Fprintf(&b, "Size: %d bytes\n", info.Size())

		// Line count + binary detection for files under 2MB.
		if info.Size() <= 2*1024*1024 {
			data, readErr := os.ReadFile(cleanPath)
			if readErr == nil {
				sample := data
				if len(sample) > 8192 {
					sample = sample[:8192]
				}
				isBinary := false
				for _, c := range sample {
					if c == 0 {
						isBinary = true
						break
					}
				}
				if isBinary {
					fmt.Fprintf(&b, "Content: binary\n")
				} else {
					lines := strings.Count(string(data), "\n")
					if len(data) > 0 && data[len(data)-1] != '\n' {
						lines++
					}
					fmt.Fprintf(&b, "Content: text\n")
					fmt.Fprintf(&b, "Lines: %d\n", lines)
				}
			}
		} else {
			fmt.Fprintf(&b, "Content: (too large to inspect)\n")
		}
	}
	fmt.Fprintf(&b, "Permissions: %s\n", info.Mode().Perm())
	fmt.Fprintf(&b, "Modified: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))

	return ToolResult{ToolCallID: call.ID, Output: b.String()}
}

func (e *Executor) executeGrepSearch(call provider.ToolCall) ToolResult {
	var args struct {
		Pattern         string `json:"pattern"`
		Path            string `json:"path"`
		Include         string `json:"include"`
		Exclude         string `json:"exclude"`
		ContextLines    int    `json:"context_lines"`
		CaseInsensitive bool   `json:"case_insensitive"`
		MaxCount        int    `json:"max_count"`
		FilesOnly       bool   `json:"files_only"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Errorf("invalid arguments for grep_search: %w", err),
		}
	}
	if args.Pattern == "" {
		return ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Errorf("grep_search: pattern is required"),
		}
	}
	if args.Path == "" {
		args.Path = "."
	}
	if args.MaxCount <= 0 {
		args.MaxCount = 100
	}
	if args.MaxCount > 1000 {
		args.MaxCount = 1000
	}
	if args.ContextLines < 0 {
		args.ContextLines = 0
	}
	if args.ContextLines > 10 {
		args.ContextLines = 10
	}

	cleanPath, err := validatePath(args.Path)
	if err != nil {
		return ToolResult{ToolCallID: call.ID, Error: err}
	}

	// Restrict to working directory.
	wd, _ := os.Getwd()
	if !strings.HasPrefix(cleanPath, wd+string(filepath.Separator)) && cleanPath != wd {
		return ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Errorf("grep_search: path outside project directory"),
		}
	}

	// Compile pattern, applying case-insensitive flag if requested.
	pattern := args.Pattern
	if args.CaseInsensitive {
		pattern = "(?i:" + pattern + ")"
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		// Try as literal string if regex fails.
		escaped := regexp.QuoteMeta(args.Pattern)
		if args.CaseInsensitive {
			escaped = "(?i:" + escaped + ")"
		}
		re2, err2 := regexp.Compile(escaped)
		if err2 != nil {
			return ToolResult{
				ToolCallID: call.ID,
				Error:      fmt.Errorf("grep_search: invalid pattern %q. Hint: use a plain text string or valid Go regex.", args.Pattern),
			}
		}
		re = re2
	}

	var results strings.Builder
	matchCount := 0
	maxMatches := args.MaxCount
	truncated := false
	seenFiles := make(map[string]bool) // for files_only dedup

	_ = filepath.Walk(cleanPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			// Skip hidden directories
			if info != nil && info.IsDir() && strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if matchCount >= maxMatches {
			truncated = true
			return filepath.SkipAll
		}

		// Apply include filter.
		if args.Include != "" {
			matched, _ := filepath.Match(args.Include, info.Name())
			if !matched {
				return nil
			}
		}

		// Apply exclude filter.
		if args.Exclude != "" {
			matched, _ := filepath.Match(args.Exclude, info.Name())
			if matched {
				return nil
			}
		}

		// Skip binary/large files.
		if info.Size() > 1024*1024 {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		rel, _ := filepath.Rel(wd, path)
		scanner := bufio.NewScanner(f)

		if args.FilesOnly {
			// files_only mode: just detect if any line matches, emit path once.
			lineNum := 0
			for scanner.Scan() {
				lineNum++
				if re.MatchString(scanner.Text()) {
					if !seenFiles[rel] {
						seenFiles[rel] = true
						matchCount++
						fmt.Fprintf(&results, "%s\n", rel)
					}
					return nil // skip rest of file
				}
			}
		} else if args.ContextLines > 0 {
			// Context-aware scanning.
			contextN := args.ContextLines

			type bufferedLine struct {
				num  int
				text string
			}
			beforeBuf := make([]bufferedLine, 0, contextN)
			afterRemaining := 0
			lastEmittedLine := 0

			lineNum := 0
			for scanner.Scan() {
				lineNum++
				line := scanner.Text()

				isMatch := re.MatchString(line)

				if isMatch {
					// Check if this match would exceed the limit before
					// emitting any separator or pre-context for it.
					matchCount++
					if matchCount > maxMatches {
						truncated = true
						break
					}

					// Group separator when there's a gap between context groups.
					if lastEmittedLine > 0 && lineNum-len(beforeBuf) > lastEmittedLine+1 {
						fmt.Fprintf(&results, "--\n")
					}

					// Emit buffered before-context lines.
					for _, bl := range beforeBuf {
						if bl.num > lastEmittedLine {
							fmt.Fprintf(&results, "%s-%d- %s\n", rel, bl.num, bl.text)
							lastEmittedLine = bl.num
						}
					}
					beforeBuf = beforeBuf[:0]

					// Emit the match line.
					if lineNum > lastEmittedLine {
						fmt.Fprintf(&results, "%s:%d: %s\n", rel, lineNum, line)
						lastEmittedLine = lineNum
					}
					afterRemaining = contextN
				} else if afterRemaining > 0 {
					// Emit as trailing context.
					if lineNum > lastEmittedLine {
						fmt.Fprintf(&results, "%s-%d- %s\n", rel, lineNum, line)
						lastEmittedLine = lineNum
					}
					afterRemaining--
				} else {
					// Buffer for potential before-context.
					if len(beforeBuf) >= contextN {
						beforeBuf = beforeBuf[1:]
					}
					beforeBuf = append(beforeBuf, bufferedLine{num: lineNum, text: line})
				}
			}
		} else {
			// Simple mode: no context, original behavior.
			lineNum := 0
			for scanner.Scan() {
				lineNum++
				line := scanner.Text()
				if re.MatchString(line) {
					matchCount++
					if matchCount > maxMatches {
						truncated = true
						break
					}
					fmt.Fprintf(&results, "%s:%d: %s\n", rel, lineNum, line)
				}
			}
		}
		return nil
	})

	output := results.String()
	if output == "" {
		output = fmt.Sprintf("No matches found for %q", args.Pattern)
	} else if truncated {
		if args.FilesOnly {
			output += fmt.Sprintf("\n... (truncated at %d files)", maxMatches)
		} else {
			output += fmt.Sprintf("\n... (truncated at %d matches)", maxMatches)
		}
	}
	return ToolResult{ToolCallID: call.ID, Output: output}
}
