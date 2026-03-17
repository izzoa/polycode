package action

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// DetectVerifyCommand auto-detects the appropriate test/verify command
// for the project in the given working directory.
func DetectVerifyCommand(workDir string) string {
	checks := []struct {
		file    string
		command string
	}{
		{"go.mod", "go test ./..."},
		{"package.json", "npm test"},
		{"Cargo.toml", "cargo test"},
		{"Makefile", "make test"},
		{"pyproject.toml", "python -m pytest"},
		{"requirements.txt", "python -m pytest"},
	}

	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(workDir, c.file)); err == nil {
			return c.command
		}
	}

	return ""
}

// RunVerification executes a verification command and returns the output
// and whether it succeeded.
func RunVerification(ctx context.Context, command string, workDir string, timeout time.Duration) (output string, success bool, err error) {
	if command == "" {
		return "", true, fmt.Errorf("no verify command configured or detected")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir

	out, err := cmd.CombinedOutput()
	output = string(out)

	if err != nil {
		return output, false, nil // command failed but that's expected — not an error in our logic
	}

	return output, true, nil
}
