package action

import "github.com/izzoa/polycode/internal/provider"

// FileReadTool returns a ToolDefinition for reading files.
func FileReadTool() provider.ToolDefinition {
	return provider.ToolDefinition{
		Name:        "file_read",
		Description: "Read the contents of a file at the given path.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
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
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The absolute or relative path of the file to write.",
				},
				"content": map[string]interface{}{
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
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The shell command to execute.",
				},
				"working_dir": map[string]interface{}{
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
