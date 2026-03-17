package action

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectVerifyCommandGo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)

	cmd := DetectVerifyCommand(dir)
	if cmd != "go test ./..." {
		t.Errorf("expected 'go test ./...', got %q", cmd)
	}
}

func TestDetectVerifyCommandNode(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)

	cmd := DetectVerifyCommand(dir)
	if cmd != "npm test" {
		t.Errorf("expected 'npm test', got %q", cmd)
	}
}

func TestDetectVerifyCommandMakefile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Makefile"), []byte("test:"), 0644)

	cmd := DetectVerifyCommand(dir)
	if cmd != "make test" {
		t.Errorf("expected 'make test', got %q", cmd)
	}
}

func TestDetectVerifyCommandUnknown(t *testing.T) {
	dir := t.TempDir()

	cmd := DetectVerifyCommand(dir)
	if cmd != "" {
		t.Errorf("expected empty string for unknown project, got %q", cmd)
	}
}

func TestDetectVerifyCommandPriority(t *testing.T) {
	// go.mod should take priority over Makefile
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
	os.WriteFile(filepath.Join(dir, "Makefile"), []byte("test:"), 0644)

	cmd := DetectVerifyCommand(dir)
	if cmd != "go test ./..." {
		t.Errorf("expected 'go test ./...' (go.mod priority), got %q", cmd)
	}
}
