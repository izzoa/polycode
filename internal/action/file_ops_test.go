package action

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePath_RelativeInside(t *testing.T) {
	wd, _ := os.Getwd()
	got, err := validatePath("foo/bar.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(wd, "foo/bar.go")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidatePath_TraversalBlocked(t *testing.T) {
	_, err := validatePath("../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal, got nil")
	}
}

func TestValidatePath_AbsoluteAllowed(t *testing.T) {
	// Absolute paths pass through (guarded by confirm flow).
	got, err := validatePath("/tmp/test-file.txt")
	if err != nil {
		t.Fatalf("unexpected error for absolute path: %v", err)
	}
	if got != "/tmp/test-file.txt" {
		t.Errorf("got %q, want /tmp/test-file.txt", got)
	}
}

func TestValidatePath_DotSlash(t *testing.T) {
	wd, _ := os.Getwd()
	got, err := validatePath("./main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(wd, "main.go")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
