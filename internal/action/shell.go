package action

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// destructivePatterns are substrings that indicate a potentially dangerous command.
var destructivePatterns = []string{
	"rm ",
	"rm\t",
	"rm-rf", // no-space variant
	"sudo ",
	"sudo\t",
	"mkfs",
	"dd ",
	"dd\t",
	"> /dev/",
	">|",  // clobber operator
	"chmod -r",
	"chown -r",
	"kill ",
	"kill\t",
	"killall ",
	"pkill ",
	"shutdown",
	"reboot",
	"format ",
	"|sh",    // pipe to shell
	"| sh",
	"|bash",
	"| bash",
	"|zsh",
	"| zsh",
	"curl|",  // curl piped to anything
	"curl |",
	"wget|",
	"wget |",
	"/dev/sd",
	"/dev/nvme",
	"/sys/",
	"/proc/",
	"find ", "find\t", // find with -delete, -exec rm
	"-delete",
	"truncate ",
	"shred ",
	"mv /",
	"\\rm",  // backslash prefix to bypass alias
	"> /",   // redirect to root paths
}

// isDestructive performs a simple heuristic check to determine if a command
// might be destructive.
func isDestructive(command string) bool {
	lower := strings.ToLower(command)
	for _, pattern := range destructivePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// execShell executes a shell command and returns the combined output.
// Requires user confirmation; destructive commands receive an extra warning.
func (e *Executor) execShell(command string, workDir string) ToolResult {
	description := fmt.Sprintf("Execute command: %s", command)
	if workDir != "" {
		description += fmt.Sprintf("\n  in directory: %s", workDir)
	}
	if isDestructive(command) {
		description += "\n\n  WARNING: This command appears to be destructive!"
	}

	if e.confirm == nil {
		return ToolResult{
			Error: fmt.Errorf("shell_exec: no confirmation callback configured"),
		}
	}
	approved, edited := e.confirm("shell_exec", description)
	if !approved {
		return ToolResult{
			Output: "command execution cancelled by user",
		}
	}
	// Apply edited command if user modified it
	if edited != nil {
		command = *edited
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if workDir != "" {
		cmd.Dir = workDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Build the output combining stdout and stderr.
	var output strings.Builder
	if stdout.Len() > 0 {
		output.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString("STDERR:\n")
		output.WriteString(stderr.String())
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ToolResult{
				Output: output.String(),
				Error:  fmt.Errorf("command timed out after %s", e.cmdTimeout),
			}
		}
		return ToolResult{
			Output: output.String(),
			Error:  fmt.Errorf("command failed: %w", err),
		}
	}

	if output.Len() == 0 {
		return ToolResult{
			Output: "(no output)",
		}
	}

	return ToolResult{
		Output: output.String(),
	}
}
