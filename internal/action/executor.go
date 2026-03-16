package action

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/izzoa/polycode/internal/provider"
)

// ToolResult holds the outcome of executing a single tool call.
type ToolResult struct {
	ToolCallID string
	Output     string
	Error      error
}

// ConfirmFunc is a callback that asks the user to confirm an action.
// It receives a human-readable description and returns true if the user approves.
type ConfirmFunc func(description string) bool

// Executor dispatches tool calls to the appropriate handler.
type Executor struct {
	confirm    ConfirmFunc
	cmdTimeout time.Duration
}

// NewExecutor creates an Executor with the given confirmation callback and
// command timeout for shell operations.
func NewExecutor(confirm ConfirmFunc, cmdTimeout time.Duration) *Executor {
	return &Executor{
		confirm:    confirm,
		cmdTimeout: cmdTimeout,
	}
}

// Execute parses a ToolCall and routes it to the correct handler.
func (e *Executor) Execute(call provider.ToolCall) ToolResult {
	switch call.Name {
	case "file_read":
		return e.executeFileRead(call)
	case "file_write":
		return e.executeFileWrite(call)
	case "shell_exec":
		return e.executeShellExec(call)
	default:
		return ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Errorf("unknown tool: %s", call.Name),
		}
	}
}

func (e *Executor) executeFileRead(call provider.ToolCall) ToolResult {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Errorf("invalid arguments for file_read: %w", err),
		}
	}
	if args.Path == "" {
		return ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Errorf("file_read: path is required"),
		}
	}
	result := e.readFile(args.Path)
	result.ToolCallID = call.ID
	return result
}

func (e *Executor) executeFileWrite(call provider.ToolCall) ToolResult {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Errorf("invalid arguments for file_write: %w", err),
		}
	}
	if args.Path == "" {
		return ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Errorf("file_write: path is required"),
		}
	}
	if args.Content == "" {
		return ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Errorf("file_write: content is required"),
		}
	}
	result := e.writeFile(args.Path, args.Content)
	result.ToolCallID = call.ID
	return result
}

func (e *Executor) executeShellExec(call provider.ToolCall) ToolResult {
	var args struct {
		Command    string `json:"command"`
		WorkingDir string `json:"working_dir"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Errorf("invalid arguments for shell_exec: %w", err),
		}
	}
	if args.Command == "" {
		return ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Errorf("shell_exec: command is required"),
		}
	}
	result := e.execShell(args.Command, args.WorkingDir)
	result.ToolCallID = call.ID
	return result
}
