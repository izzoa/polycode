package action

import "github.com/izzoa/polycode/internal/provider"

// FileReadTool returns a ToolDefinition for reading files.
func FileReadTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "file_read",
		Description: "Read the contents of a file at the given path. " +
			"Optionally read a specific line range to avoid loading large files entirely.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "The absolute or relative path of the file to read.",
				},
				"start_line": map[string]any{
					"type":        "integer",
					"description": "First line to read (1-based, inclusive). Optional — omit to read from the beginning.",
				},
				"end_line": map[string]any{
					"type":        "integer",
					"description": "Last line to read (1-based, inclusive). Optional — omit to read to the end.",
				},
			},
			"required": []string{"path"},
		},
	}
}

// FileWriteTool returns a ToolDefinition for writing files.
func FileWriteTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "file_write",
		Description: "Write content to a file at the given path, creating or overwriting it.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "The absolute or relative path of the file to write.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The content to write to the file.",
				},
			},
			"required": []string{"path", "content"},
		},
	}
}

// ShellExecTool returns a ToolDefinition for executing shell commands.
func ShellExecTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "shell_exec",
		Description: "Execute a shell command and return its output.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The shell command to execute.",
				},
				"working_dir": map[string]any{
					"type":        "string",
					"description": "The working directory for the command. Defaults to the current directory if not specified.",
				},
			},
			"required": []string{"command"},
		},
	}
}

// FileEditTool returns a ToolDefinition for targeted search-and-replace editing.
func FileEditTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "file_edit",
		Description: "Apply a targeted edit to a file by replacing an exact string match. " +
			"Read the file first, then provide the exact text to find and its replacement. " +
			"Fails if old_text is not found or matches multiple locations (unless replace_all is true).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to edit.",
				},
				"old_text": map[string]any{
					"type":        "string",
					"description": "The exact text to find in the file. Must match exactly, including whitespace and indentation.",
				},
				"new_text": map[string]any{
					"type":        "string",
					"description": "The replacement text. Use an empty string to delete the matched text.",
				},
				"replace_all": map[string]any{
					"type":        "boolean",
					"description": "If true, replace all occurrences. If false (default), fail when old_text matches more than once.",
				},
			},
			"required": []string{"path", "old_text", "new_text"},
		},
	}
}

// FileDeleteTool returns a ToolDefinition for deleting files or empty directories.
func FileDeleteTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "file_delete",
		Description: "Delete a file or empty directory. Shows what will be deleted and requires confirmation.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file or empty directory to delete.",
				},
			},
			"required": []string{"path"},
		},
	}
}

// FileRenameTool returns a ToolDefinition for renaming or moving files.
func FileRenameTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "file_rename",
		Description: "Rename or move a file or directory within the project.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"old_path": map[string]any{
					"type":        "string",
					"description": "Current path of the file or directory.",
				},
				"new_path": map[string]any{
					"type":        "string",
					"description": "New path for the file or directory.",
				},
			},
			"required": []string{"old_path", "new_path"},
		},
	}
}

// AllTools returns all available tool definitions.
func AllTools() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		FileReadTool(),
		FileWriteTool(),
		FileEditTool(),
		FileDeleteTool(),
		FileRenameTool(),
		ShellExecTool(),
		ListDirectoryTool(),
		GrepSearchTool(),
		FindFilesTool(),
		FileInfoTool(),
	}
}

// ListDirectoryTool returns a ToolDefinition for listing directory contents.
func ListDirectoryTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "list_directory",
		Description: "List the contents of a directory. Returns file and subdirectory names. Use this to explore project structure.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "The directory path to list. Use '.' for the current/project directory.",
				},
				"recursive": map[string]any{
					"type":        "boolean",
					"description": "If true, list contents recursively (up to 3 levels deep). Defaults to false.",
				},
			},
			"required": []string{"path"},
		},
	}
}

// GrepSearchTool returns a ToolDefinition for searching file contents.
func GrepSearchTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "grep_search",
		Description: "Search for a text pattern across files in the project. Returns matching lines with file paths and line numbers.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "The text or regex pattern to search for.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Directory or file to search in. Defaults to '.' (project root).",
				},
				"include": map[string]any{
					"type":        "string",
					"description": "File glob pattern to include (e.g., '*.go', '*.py'). Optional.",
				},
				"exclude": map[string]any{
					"type":        "string",
					"description": "File glob pattern to exclude (e.g., '*_test.go', '*.min.js'). Applied after include filter.",
				},
				"context_lines": map[string]any{
					"type":        "integer",
					"description": "Number of lines to show before and after each match (like grep -C). Defaults to 0.",
				},
				"case_insensitive": map[string]any{
					"type":        "boolean",
					"description": "If true, match case-insensitively. Defaults to false.",
				},
				"max_count": map[string]any{
					"type":        "integer",
					"description": "Maximum number of matches to return. Defaults to 100.",
				},
				"files_only": map[string]any{
					"type":        "boolean",
					"description": "If true, return only unique file paths containing matches (like grep -l). Defaults to false.",
				},
			},
			"required": []string{"pattern"},
		},
	}
}

// FindFilesTool returns a ToolDefinition for glob-based file search.
func FindFilesTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "find_files",
		Description: "Find files by name or glob pattern. Returns matching file paths, not contents. " +
			"Use this to locate files before reading them. Supports patterns like '*.go', '**/*_test.go', 'cmd/*/main.go'.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob pattern to match file names/paths (e.g., '*.go', '**/*.yaml', 'internal/**/*_test.go').",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Directory to search in. Defaults to '.' (project root).",
				},
			},
			"required": []string{"pattern"},
		},
	}
}

// FileInfoTool returns a ToolDefinition for getting file metadata.
func FileInfoTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "file_info",
		Description: "Get metadata about a file or directory: size, type, permissions, modification time, " +
			"and line count (for text files). Use this to check a file before reading it.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file or directory.",
				},
			},
			"required": []string{"path"},
		},
	}
}

// ReadOnlyTools returns tool definitions that are safe for concurrent fan-out
// execution — read-only operations with no side effects.
func ReadOnlyTools() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		FileReadTool(),
		ListDirectoryTool(),
		GrepSearchTool(),
		FindFilesTool(),
		FileInfoTool(),
	}
}
