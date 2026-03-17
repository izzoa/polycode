package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/izzoa/polycode/internal/config"
)

func TestRun_EmptyHookReturnsNil(t *testing.T) {
	mgr := NewHookManager(config.HooksConfig{})

	if err := mgr.Run(PreQuery, HookContext{Prompt: "hello"}); err != nil {
		t.Fatalf("expected nil error for empty hook, got %v", err)
	}
	if err := mgr.Run(PostQuery, HookContext{}); err != nil {
		t.Fatalf("expected nil error for empty hook, got %v", err)
	}
	if err := mgr.Run(PostTool, HookContext{}); err != nil {
		t.Fatalf("expected nil error for empty hook, got %v", err)
	}
	if err := mgr.Run(OnError, HookContext{}); err != nil {
		t.Fatalf("expected nil error for empty hook, got %v", err)
	}
}

func TestRun_TemplateSubstitution(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "hook_output.txt")

	mgr := NewHookManager(config.HooksConfig{
		PreQuery: "echo 'prompt={{.Prompt}}' > " + outFile,
	})

	if err := mgr.Run(PreQuery, HookContext{Prompt: "test-query"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read hook output: %v", err)
	}

	expected := "prompt=test-query\n"
	if string(data) != expected {
		t.Errorf("hook output = %q, want %q", string(data), expected)
	}
}

func TestRun_OnErrorTemplateSubstitution(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "error_output.txt")

	mgr := NewHookManager(config.HooksConfig{
		OnError: "echo 'err={{.Error}}' > " + outFile,
	})

	if err := mgr.Run(OnError, HookContext{Error: "something broke"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read hook output: %v", err)
	}

	expected := "err=something broke\n"
	if string(data) != expected {
		t.Errorf("hook output = %q, want %q", string(data), expected)
	}
}

func TestRun_PostToolTemplateSubstitution(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "tool_output.txt")

	mgr := NewHookManager(config.HooksConfig{
		PostTool: "echo 'tool={{.ToolName}} response={{.Response}}' > " + outFile,
	})

	if err := mgr.Run(PostTool, HookContext{ToolName: "file_read", Response: "contents"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read hook output: %v", err)
	}

	expected := "tool=file_read response=contents\n"
	if string(data) != expected {
		t.Errorf("hook output = %q, want %q", string(data), expected)
	}
}

func TestRun_InvalidCommandDoesNotReturnError(t *testing.T) {
	mgr := NewHookManager(config.HooksConfig{
		PreQuery: "false", // exits non-zero
	})

	// Hooks should log but not return errors.
	if err := mgr.Run(PreQuery, HookContext{}); err != nil {
		t.Fatalf("expected nil error even for failing hook, got %v", err)
	}
}
