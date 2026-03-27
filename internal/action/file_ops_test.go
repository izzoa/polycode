package action

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/izzoa/polycode/internal/provider"
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

// helper: create executor with no confirm (for read-only tools).
func newTestExecutor() *Executor {
	return NewExecutor(nil, 0)
}

// helper: create executor that auto-approves confirmations.
func newTestExecutorWithConfirm(approve bool) *Executor {
	return NewExecutor(func(_, _ string) bool { return approve }, 0)
}

// helper: make a ToolCall with JSON arguments.
func makeCall(name string, args any) provider.ToolCall {
	b, _ := json.Marshal(args)
	return provider.ToolCall{ID: "test-1", Name: name, Arguments: string(b)}
}

// --- find_files tests ---

func TestFindFiles_SimpleGlob(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.go"), []byte("package a"), 0o644)
	os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(tmp, "c.go"), []byte("package c"), 0o644)

	// chdir into the temp dir so validatePath allows it.
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.executeFindFiles(makeCall("find_files", map[string]string{"pattern": "*.go"}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "a.go") || !strings.Contains(result.Output, "c.go") {
		t.Errorf("expected a.go and c.go in output, got: %s", result.Output)
	}
	if strings.Contains(result.Output, "b.txt") {
		t.Errorf("should not contain b.txt, got: %s", result.Output)
	}
}

func TestFindFiles_RecursiveGlob(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "sub")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(tmp, "top.go"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(sub, "deep.go"), []byte("x"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.executeFindFiles(makeCall("find_files", map[string]string{"pattern": "**/*.go"}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "deep.go") {
		t.Errorf("expected deep.go in recursive results, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "top.go") {
		t.Errorf("expected top.go in recursive results, got: %s", result.Output)
	}
}

func TestFindFiles_NoMatches(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("x"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.executeFindFiles(makeCall("find_files", map[string]string{"pattern": "*.go"}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "No files found") {
		t.Errorf("expected 'No files found' message, got: %s", result.Output)
	}
}

func TestFindFiles_SkipsHiddenDirs(t *testing.T) {
	tmp := t.TempDir()
	hidden := filepath.Join(tmp, ".git")
	os.MkdirAll(hidden, 0o755)
	os.WriteFile(filepath.Join(hidden, "config.go"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte("x"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.executeFindFiles(makeCall("find_files", map[string]string{"pattern": "**/*.go"}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.Contains(result.Output, ".git") {
		t.Errorf("should skip .git directory, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "main.go") {
		t.Errorf("expected main.go, got: %s", result.Output)
	}
}

func TestFindFiles_PathTraversal(t *testing.T) {
	e := newTestExecutor()
	result := e.executeFindFiles(makeCall("find_files", map[string]any{
		"pattern": "*.go",
		"path":    "../../",
	}))
	if result.Error == nil {
		t.Error("expected error for path traversal, got nil")
	}
}

func TestFindFiles_MaxResults(t *testing.T) {
	tmp := t.TempDir()
	for i := 0; i < 210; i++ {
		name := filepath.Join(tmp, fmt.Sprintf("file_%04d.go", i))
		os.WriteFile(name, []byte("x"), 0o644)
	}

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.executeFindFiles(makeCall("find_files", map[string]string{"pattern": "*.go"}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "truncated at 200") {
		t.Errorf("expected truncation message, got: %s", result.Output)
	}
}

// --- file_info tests ---

func TestFileInfo_TextFile(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "test.txt"), []byte("line1\nline2\nline3\n"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.executeFileInfo(makeCall("file_info", map[string]string{"path": "test.txt"}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "Type: file") {
		t.Errorf("expected Type: file, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Content: text") {
		t.Errorf("expected Content: text, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Lines: 3") {
		t.Errorf("expected Lines: 3, got: %s", result.Output)
	}
}

func TestFileInfo_Directory(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("x"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.executeFileInfo(makeCall("file_info", map[string]string{"path": "."}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "Type: directory") {
		t.Errorf("expected Type: directory, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Entries: 2") {
		t.Errorf("expected Entries: 2, got: %s", result.Output)
	}
}

func TestFileInfo_BinaryFile(t *testing.T) {
	tmp := t.TempDir()
	// Write a file with null bytes to trigger binary detection.
	os.WriteFile(filepath.Join(tmp, "binary.dat"), []byte{0x00, 0x01, 0x02, 0x03}, 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.executeFileInfo(makeCall("file_info", map[string]string{"path": "binary.dat"}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "Content: binary") {
		t.Errorf("expected Content: binary, got: %s", result.Output)
	}
}

func TestFileInfo_NotFound(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.executeFileInfo(makeCall("file_info", map[string]string{"path": "nonexistent.txt"}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "not found") {
		t.Errorf("expected 'not found' message, got: %s", result.Output)
	}
}

func TestFileInfo_PathTraversal(t *testing.T) {
	e := newTestExecutor()
	result := e.executeFileInfo(makeCall("file_info", map[string]string{"path": "../../etc/passwd"}))
	if result.Error == nil {
		// file_info returns output "not found" for non-existent, error for traversal
		// validatePath allows absolute paths; traversal is blocked for relative.
		t.Error("expected error for path traversal, got nil")
	}
}

// --- file_read line range tests ---

func TestFileRead_LineRange(t *testing.T) {
	tmp := t.TempDir()
	lines := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"
	os.WriteFile(filepath.Join(tmp, "ten.txt"), []byte(lines), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.readFile("ten.txt", 5, 7)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "Lines 5-7 of") {
		t.Errorf("expected line range header, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "5\tline5") {
		t.Errorf("expected line5 with number, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "7\tline7") {
		t.Errorf("expected line7 with number, got: %s", result.Output)
	}
	if strings.Contains(result.Output, "8\tline8") {
		t.Errorf("should not include line8, got: %s", result.Output)
	}
}

func TestFileRead_StartOnly(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "five.txt"), []byte("a\nb\nc\nd\ne\n"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.readFile("five.txt", 3, 0)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "3\tc") {
		t.Errorf("expected line 3 = 'c', got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "5\te") {
		t.Errorf("expected line 5 = 'e', got: %s", result.Output)
	}
}

func TestFileRead_EndOnly(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "five.txt"), []byte("a\nb\nc\nd\ne\n"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.readFile("five.txt", 0, 3)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "1\ta") {
		t.Errorf("expected line 1 = 'a', got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "3\tc") {
		t.Errorf("expected line 3 = 'c', got: %s", result.Output)
	}
	if strings.Contains(result.Output, "4\td") {
		t.Errorf("should not include line 4, got: %s", result.Output)
	}
}

func TestFileRead_PastEnd(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "short.txt"), []byte("one\ntwo\nthree\n"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.readFile("short.txt", 999, 0)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "past end") {
		t.Errorf("expected 'past end' message, got: %s", result.Output)
	}
}

func TestFileRead_ReversedRange(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "rev.txt"), []byte("a\nb\nc\nd\ne\n"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.readFile("rev.txt", 5, 3)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "invalid range") {
		t.Errorf("expected 'invalid range' message for reversed range, got: %s", result.Output)
	}
}

func TestFileRead_TrailingNewlineCount(t *testing.T) {
	tmp := t.TempDir()
	// 3 actual lines, newline-terminated.
	os.WriteFile(filepath.Join(tmp, "nl.txt"), []byte("one\ntwo\nthree\n"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.readFile("nl.txt", 1, 0)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	// Should report "of 3" not "of 4".
	if !strings.Contains(result.Output, "of 3") {
		t.Errorf("expected total of 3 lines for newline-terminated file, got: %s", result.Output)
	}
}

func TestFileRead_NoRange(t *testing.T) {
	tmp := t.TempDir()
	content := "hello world\n"
	os.WriteFile(filepath.Join(tmp, "full.txt"), []byte(content), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.readFile("full.txt", 0, 0)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Output != content {
		t.Errorf("expected full file content %q, got %q", content, result.Output)
	}
}

// --- file_edit tests ---

func TestFileEdit_SingleMatch(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "edit.txt")
	os.WriteFile(path, []byte("hello world\nfoo bar\n"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(true)
	result := e.executeFileEdit(makeCall("file_edit", map[string]any{
		"path": "edit.txt", "old_text": "foo bar", "new_text": "baz qux",
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "1 replacement") {
		t.Errorf("expected '1 replacement', got: %s", result.Output)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "baz qux") {
		t.Errorf("file should contain 'baz qux', got: %s", string(data))
	}
	if strings.Contains(string(data), "foo bar") {
		t.Errorf("file should no longer contain 'foo bar'")
	}
}

func TestFileEdit_MultipleMatchFails(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "dup.txt")
	os.WriteFile(path, []byte("aaa\nbbb\naaa\n"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(true)
	result := e.executeFileEdit(makeCall("file_edit", map[string]any{
		"path": "dup.txt", "old_text": "aaa", "new_text": "zzz",
	}))
	if result.Error == nil {
		t.Fatal("expected error for multiple matches without replace_all")
	}
	if !strings.Contains(result.Error.Error(), "2 locations") {
		t.Errorf("expected '2 locations' in error, got: %v", result.Error)
	}
	// File should be unchanged.
	data, _ := os.ReadFile(path)
	if string(data) != "aaa\nbbb\naaa\n" {
		t.Errorf("file should be unchanged, got: %s", string(data))
	}
}

func TestFileEdit_ReplaceAll(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "rep.txt")
	os.WriteFile(path, []byte("aaa\nbbb\naaa\n"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(true)
	result := e.executeFileEdit(makeCall("file_edit", map[string]any{
		"path": "rep.txt", "old_text": "aaa", "new_text": "zzz", "replace_all": true,
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "2 replacements") {
		t.Errorf("expected '2 replacements', got: %s", result.Output)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "aaa") {
		t.Errorf("all 'aaa' should be replaced, got: %s", string(data))
	}
}

func TestFileEdit_NotFound(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "nf.txt"), []byte("hello\n"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(true)
	result := e.executeFileEdit(makeCall("file_edit", map[string]any{
		"path": "nf.txt", "old_text": "nonexistent", "new_text": "x",
	}))
	if result.Error == nil {
		t.Fatal("expected error for old_text not found")
	}
	if !strings.Contains(result.Error.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", result.Error)
	}
}

func TestFileEdit_IdenticalTexts(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "id.txt"), []byte("same\n"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(true)
	result := e.executeFileEdit(makeCall("file_edit", map[string]any{
		"path": "id.txt", "old_text": "same", "new_text": "same",
	}))
	if result.Error == nil {
		t.Fatal("expected error for identical old_text and new_text")
	}
	if !strings.Contains(result.Error.Error(), "identical") {
		t.Errorf("expected 'identical' in error, got: %v", result.Error)
	}
}

func TestFileEdit_DeleteText(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "del.txt")
	os.WriteFile(path, []byte("keep this\nremove me\nkeep this too\n"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(true)
	result := e.executeFileEdit(makeCall("file_edit", map[string]any{
		"path": "del.txt", "old_text": "remove me\n", "new_text": "",
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "remove me") {
		t.Errorf("'remove me' should be deleted, got: %s", string(data))
	}
	if !strings.Contains(string(data), "keep this") {
		t.Errorf("'keep this' should remain, got: %s", string(data))
	}
}

func TestFileEdit_PathTraversal(t *testing.T) {
	e := newTestExecutorWithConfirm(true)
	result := e.executeFileEdit(makeCall("file_edit", map[string]any{
		"path": "../../etc/passwd", "old_text": "root", "new_text": "x",
	}))
	if result.Error == nil {
		t.Error("expected error for path traversal, got nil")
	}
}

func TestFileEdit_Cancelled(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cancel.txt")
	os.WriteFile(path, []byte("original\n"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(false) // user rejects
	result := e.executeFileEdit(makeCall("file_edit", map[string]any{
		"path": "cancel.txt", "old_text": "original", "new_text": "modified",
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "cancelled") {
		t.Errorf("expected 'cancelled' message, got: %s", result.Output)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "original\n" {
		t.Errorf("file should be unchanged after cancel, got: %s", string(data))
	}
}

// --- file_delete tests ---

func TestFileDelete_File(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "doomed.txt")
	os.WriteFile(path, []byte("bye"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(true)
	result := e.executeFileDelete(makeCall("file_delete", map[string]string{"path": "doomed.txt"}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "deleted") {
		t.Errorf("expected 'deleted' message, got: %s", result.Output)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
}

func TestFileDelete_EmptyDir(t *testing.T) {
	tmp := t.TempDir()
	emptyDir := filepath.Join(tmp, "empty")
	os.MkdirAll(emptyDir, 0o755)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(true)
	result := e.executeFileDelete(makeCall("file_delete", map[string]string{"path": "empty"}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if _, err := os.Stat(emptyDir); !os.IsNotExist(err) {
		t.Error("empty directory should be deleted")
	}
}

func TestFileDelete_NonEmptyDir(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "full")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "child.txt"), []byte("x"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(true)
	result := e.executeFileDelete(makeCall("file_delete", map[string]string{"path": "full"}))
	if result.Error == nil {
		t.Fatal("expected error for non-empty directory")
	}
	if !strings.Contains(result.Error.Error(), "not empty") {
		t.Errorf("expected 'not empty' in error, got: %v", result.Error)
	}
	// Directory should still exist.
	if _, err := os.Stat(dir); err != nil {
		t.Error("directory should still exist after rejection")
	}
}

func TestFileDelete_NotFound(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(true)
	result := e.executeFileDelete(makeCall("file_delete", map[string]string{"path": "ghost.txt"}))
	if result.Error == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(result.Error.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", result.Error)
	}
}

func TestFileDelete_PathTraversal(t *testing.T) {
	e := newTestExecutorWithConfirm(true)
	result := e.executeFileDelete(makeCall("file_delete", map[string]string{"path": "../../etc/passwd"}))
	if result.Error == nil {
		t.Error("expected error for path traversal")
	}
}

func TestFileDelete_Cancelled(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "keep.txt")
	os.WriteFile(path, []byte("safe"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(false)
	result := e.executeFileDelete(makeCall("file_delete", map[string]string{"path": "keep.txt"}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "cancelled") {
		t.Errorf("expected 'cancelled' message, got: %s", result.Output)
	}
	if _, err := os.Stat(path); err != nil {
		t.Error("file should still exist after cancel")
	}
}

// --- file_rename tests ---

func TestFileRename_Basic(t *testing.T) {
	tmp := t.TempDir()
	oldPath := filepath.Join(tmp, "old.txt")
	os.WriteFile(oldPath, []byte("content"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(true)
	result := e.executeFileRename(makeCall("file_rename", map[string]string{
		"old_path": "old.txt", "new_path": "new.txt",
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "renamed") {
		t.Errorf("expected 'renamed' message, got: %s", result.Output)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old path should be gone")
	}
	data, _ := os.ReadFile(filepath.Join(tmp, "new.txt"))
	if string(data) != "content" {
		t.Errorf("new file should have original content, got: %s", string(data))
	}
}

func TestFileRename_Move(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "move.txt"), []byte("data"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(true)
	result := e.executeFileRename(makeCall("file_rename", map[string]string{
		"old_path": "move.txt", "new_path": "sub/moved.txt",
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	// Parent dir should have been created.
	data, err := os.ReadFile(filepath.Join(tmp, "sub", "moved.txt"))
	if err != nil {
		t.Fatalf("new file should exist: %v", err)
	}
	if string(data) != "data" {
		t.Errorf("moved file should have original content, got: %s", string(data))
	}
}

func TestFileRename_DestExists(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "src.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(tmp, "dst.txt"), []byte("b"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(true)
	result := e.executeFileRename(makeCall("file_rename", map[string]string{
		"old_path": "src.txt", "new_path": "dst.txt",
	}))
	if result.Error == nil {
		t.Fatal("expected error when destination exists")
	}
	if !strings.Contains(result.Error.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", result.Error)
	}
}

func TestFileRename_SourceNotFound(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(true)
	result := e.executeFileRename(makeCall("file_rename", map[string]string{
		"old_path": "ghost.txt", "new_path": "new.txt",
	}))
	if result.Error == nil {
		t.Fatal("expected error for nonexistent source")
	}
	if !strings.Contains(result.Error.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", result.Error)
	}
}

func TestFileRename_PathTraversal(t *testing.T) {
	e := newTestExecutorWithConfirm(true)
	result := e.executeFileRename(makeCall("file_rename", map[string]string{
		"old_path": "../../etc/passwd", "new_path": "stolen.txt",
	}))
	if result.Error == nil {
		t.Error("expected error for path traversal on old_path")
	}
}

func TestFileRename_Cancelled(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "stay.txt")
	os.WriteFile(path, []byte("here"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(false)
	result := e.executeFileRename(makeCall("file_rename", map[string]string{
		"old_path": "stay.txt", "new_path": "gone.txt",
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "cancelled") {
		t.Errorf("expected 'cancelled' message, got: %s", result.Output)
	}
	if _, err := os.Stat(path); err != nil {
		t.Error("file should still exist after cancel")
	}
}

func TestFileRename_SelfSubtree(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "mydir")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutorWithConfirm(true)
	result := e.executeFileRename(makeCall("file_rename", map[string]string{
		"old_path": "mydir", "new_path": "mydir/sub/nested",
	}))
	if result.Error == nil {
		t.Fatal("expected error for self-subtree rename")
	}
	if !strings.Contains(result.Error.Error(), "inside source") {
		t.Errorf("expected 'inside source' in error, got: %v", result.Error)
	}
}

// --- grep_search tests ---

// helper: set up a temp dir with test files and chdir into it.
func setupGrepDir(t *testing.T) (cleanup func()) {
	t.Helper()
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello world\")\n}\n"), 0o644)
	os.WriteFile(filepath.Join(tmp, "main_test.go"), []byte("package main\n\nfunc TestMain(t *testing.T) {\n\t// test\n}\n"), 0o644)
	os.WriteFile(filepath.Join(tmp, "readme.txt"), []byte("Hello World\nThis is a readme.\nHELLO again.\n"), 0o644)
	sub := filepath.Join(tmp, "sub")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "util.go"), []byte("package sub\n\nfunc helper() {\n\tfmt.Println(\"hello from sub\")\n}\n"), 0o644)
	hidden := filepath.Join(tmp, ".hidden")
	os.MkdirAll(hidden, 0o755)
	os.WriteFile(filepath.Join(hidden, "secret.go"), []byte("package hidden\n// hello secret\n"), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	return func() { os.Chdir(orig) }
}

func TestGrepSearch_BasicMatch(t *testing.T) {
	cleanup := setupGrepDir(t)
	defer cleanup()

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{"pattern": "hello"}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "main.go:") {
		t.Errorf("expected match in main.go, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "hello world") {
		t.Errorf("expected 'hello world' in output, got: %s", result.Output)
	}
}

func TestGrepSearch_NoMatches(t *testing.T) {
	cleanup := setupGrepDir(t)
	defer cleanup()

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{"pattern": "zzzznotfound"}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "No matches found") {
		t.Errorf("expected 'No matches found', got: %s", result.Output)
	}
}

func TestGrepSearch_RegexPattern(t *testing.T) {
	cleanup := setupGrepDir(t)
	defer cleanup()

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{"pattern": `func\s+\w+`}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "func main") {
		t.Errorf("expected regex match for 'func main', got: %s", result.Output)
	}
}

func TestGrepSearch_LiteralFallback(t *testing.T) {
	cleanup := setupGrepDir(t)
	defer cleanup()

	e := newTestExecutor()
	// Invalid regex — should fall back to literal match.
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{"pattern": "fmt.Println("}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "fmt.Println(") {
		t.Errorf("expected literal match for 'fmt.Println(', got: %s", result.Output)
	}
}

func TestGrepSearch_IncludeFilter(t *testing.T) {
	cleanup := setupGrepDir(t)
	defer cleanup()

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{
		"pattern": "hello", "include": "*.go",
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.Contains(result.Output, "readme.txt") {
		t.Errorf("include *.go should exclude readme.txt, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "main.go") {
		t.Errorf("expected match in main.go, got: %s", result.Output)
	}
}

func TestGrepSearch_ExcludeFilter(t *testing.T) {
	cleanup := setupGrepDir(t)
	defer cleanup()

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{
		"pattern": "package", "exclude": "*_test.go",
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.Contains(result.Output, "main_test.go") {
		t.Errorf("exclude *_test.go should remove test file, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "main.go") {
		t.Errorf("expected match in main.go, got: %s", result.Output)
	}
}

func TestGrepSearch_IncludeAndExclude(t *testing.T) {
	cleanup := setupGrepDir(t)
	defer cleanup()

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{
		"pattern": "package", "include": "*.go", "exclude": "*_test.go",
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.Contains(result.Output, "main_test.go") {
		t.Errorf("should exclude test files, got: %s", result.Output)
	}
	if strings.Contains(result.Output, "readme.txt") {
		t.Errorf("should exclude non-go files, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "main.go") {
		t.Errorf("expected match in main.go, got: %s", result.Output)
	}
}

func TestGrepSearch_CaseInsensitive(t *testing.T) {
	cleanup := setupGrepDir(t)
	defer cleanup()

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{
		"pattern": "hello", "include": "*.txt", "case_insensitive": true,
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	// readme.txt has "Hello World" and "HELLO again." — both should match.
	if !strings.Contains(result.Output, "Hello World") {
		t.Errorf("expected case-insensitive match for 'Hello World', got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "HELLO again") {
		t.Errorf("expected case-insensitive match for 'HELLO again', got: %s", result.Output)
	}
}

func TestGrepSearch_CaseSensitiveDefault(t *testing.T) {
	cleanup := setupGrepDir(t)
	defer cleanup()

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{
		"pattern": "hello", "include": "*.txt",
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	// Case-sensitive: "Hello" and "HELLO" should NOT match "hello".
	if strings.Contains(result.Output, "Hello World") {
		t.Errorf("case-sensitive should not match 'Hello World', got: %s", result.Output)
	}
}

func TestGrepSearch_MaxCountCustom(t *testing.T) {
	cleanup := setupGrepDir(t)
	defer cleanup()

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{
		"pattern": ".", "max_count": 3,
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	// Count colon-separated match lines (file:line: content).
	lines := strings.Split(strings.TrimSpace(result.Output), "\n")
	matchLines := 0
	for _, l := range lines {
		if strings.Contains(l, ":") && !strings.HasPrefix(l, "...") {
			matchLines++
		}
	}
	if matchLines > 3 {
		t.Errorf("expected at most 3 matches, got %d: %s", matchLines, result.Output)
	}
	if !strings.Contains(result.Output, "truncated at 3") {
		t.Errorf("expected truncation message, got: %s", result.Output)
	}
}

func TestGrepSearch_MaxCountDefault(t *testing.T) {
	tmp := t.TempDir()
	// Create a file with 150 matching lines.
	var content strings.Builder
	for i := 1; i <= 150; i++ {
		fmt.Fprintf(&content, "match line %d\n", i)
	}
	os.WriteFile(filepath.Join(tmp, "big.txt"), []byte(content.String()), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{"pattern": "match line"}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "truncated at 100") {
		t.Errorf("expected default truncation at 100, got: %s", result.Output)
	}
}

func TestGrepSearch_FilesOnly(t *testing.T) {
	cleanup := setupGrepDir(t)
	defer cleanup()

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{
		"pattern": "package", "files_only": true,
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	// Should list file paths without line numbers.
	if strings.Contains(result.Output, ":1:") {
		t.Errorf("files_only should not contain line numbers, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "main.go") {
		t.Errorf("expected main.go in output, got: %s", result.Output)
	}
}

func TestGrepSearch_FilesOnlyDedup(t *testing.T) {
	cleanup := setupGrepDir(t)
	defer cleanup()

	e := newTestExecutor()
	// "hello" appears multiple times in main.go — should only list it once.
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{
		"pattern": "fmt", "include": "*.go", "files_only": true,
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	count := strings.Count(result.Output, "main.go")
	if count > 1 {
		t.Errorf("files_only should dedup, main.go appeared %d times: %s", count, result.Output)
	}
}

func TestGrepSearch_FilesOnlyMaxCount(t *testing.T) {
	cleanup := setupGrepDir(t)
	defer cleanup()

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{
		"pattern": "package", "files_only": true, "max_count": 1,
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	lines := strings.Split(strings.TrimSpace(result.Output), "\n")
	// First line is a file path, possibly a truncation message after.
	filePaths := 0
	for _, l := range lines {
		if l != "" && !strings.HasPrefix(l, "...") {
			filePaths++
		}
	}
	if filePaths > 1 {
		t.Errorf("files_only max_count=1 should return 1 file, got %d: %s", filePaths, result.Output)
	}
}

func TestGrepSearch_ContextLines(t *testing.T) {
	tmp := t.TempDir()
	// File with a single match at line 5, surrounded by numbered lines.
	content := "line1\nline2\nline3\nline4\nMATCH_HERE\nline6\nline7\nline8\nline9\n"
	os.WriteFile(filepath.Join(tmp, "ctx.txt"), []byte(content), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{
		"pattern": "MATCH_HERE", "context_lines": 2,
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	// Should have before-context (lines 3-4), match (line 5), after-context (lines 6-7).
	if !strings.Contains(result.Output, "-3- line3") {
		t.Errorf("expected before-context line3, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "-4- line4") {
		t.Errorf("expected before-context line4, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, ":5: MATCH_HERE") {
		t.Errorf("expected match at line 5, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "-6- line6") {
		t.Errorf("expected after-context line6, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "-7- line7") {
		t.Errorf("expected after-context line7, got: %s", result.Output)
	}
	// Should NOT include line8.
	if strings.Contains(result.Output, "line8") {
		t.Errorf("should not include line8 with context_lines=2, got: %s", result.Output)
	}
}

func TestGrepSearch_ContextAtFileStart(t *testing.T) {
	tmp := t.TempDir()
	content := "MATCH_FIRST\nline2\nline3\nline4\n"
	os.WriteFile(filepath.Join(tmp, "start.txt"), []byte(content), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{
		"pattern": "MATCH_FIRST", "context_lines": 3,
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, ":1: MATCH_FIRST") {
		t.Errorf("expected match at line 1, got: %s", result.Output)
	}
	// After-context should show lines 2-4.
	if !strings.Contains(result.Output, "-2- line2") {
		t.Errorf("expected after-context line2, got: %s", result.Output)
	}
}

func TestGrepSearch_ContextOverlapping(t *testing.T) {
	tmp := t.TempDir()
	// Two matches 2 lines apart with context_lines=3 — contexts overlap, no duplicates.
	content := "line1\nMATCH_A\nline3\nMATCH_B\nline5\nline6\n"
	os.WriteFile(filepath.Join(tmp, "overlap.txt"), []byte(content), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{
		"pattern": "MATCH_", "context_lines": 3,
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	// line3 is between the two matches — should appear once as context, not twice.
	count := strings.Count(result.Output, "line3")
	if count > 1 {
		t.Errorf("overlapping context should not duplicate line3, appeared %d times: %s", count, result.Output)
	}
	// No separator between overlapping groups.
	if strings.Contains(result.Output, "--") {
		t.Errorf("should not have separator between overlapping groups, got: %s", result.Output)
	}
}

func TestGrepSearch_ContextSeparator(t *testing.T) {
	tmp := t.TempDir()
	// Two matches far apart — should have a -- separator.
	var content strings.Builder
	content.WriteString("MATCH_FIRST\n")
	for i := 2; i <= 20; i++ {
		fmt.Fprintf(&content, "filler%d\n", i)
	}
	content.WriteString("MATCH_SECOND\n")
	os.WriteFile(filepath.Join(tmp, "sep.txt"), []byte(content.String()), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{
		"pattern": "MATCH_", "context_lines": 1,
	}))
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "--\n") {
		t.Errorf("expected group separator between distant matches, got: %s", result.Output)
	}
}

func TestGrepSearch_PathTraversal(t *testing.T) {
	e := newTestExecutor()
	result := e.executeGrepSearch(makeCall("grep_search", map[string]any{
		"pattern": "secret", "path": "../../",
	}))
	if result.Error == nil {
		t.Error("expected error for path traversal")
	}
}
