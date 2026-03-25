package action

import "github.com/izzoa/polycode/internal/provider"

// FileReadTool returns a ToolDefinition for reading files.
func FileReadTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "file_read",
		Description: "Read the contents of a file at the given path.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "The absolute or relative path of the file to read.",
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

// AllTools returns all available tool definitions.
func AllTools() []provider.ToolDefinition {
	return []provider.ToolDefinition{
		FileReadTool(),
		FileWriteTool(),
		ShellExecTool(),
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
			},
			"required": []string{"pattern"},
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
	}
}
