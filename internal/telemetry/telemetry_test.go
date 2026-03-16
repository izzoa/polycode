package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// setTestConfigDir overrides XDG_CONFIG_HOME so ConfigDir() points at a
// temporary directory, keeping the real home directory clean.
func setTestConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return filepath.Join(dir, "polycode")
}

func TestNewLoggerCreatesFileAndLogWritesJSONL(t *testing.T) {
	cfgDir := setTestConfigDir(t)

	logger, err := NewLogger()
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer logger.Close()

	path := filepath.Join(cfgDir, "telemetry.jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("telemetry file not created: %v", err)
	}

	success := true
	logger.Log(Event{
		ProviderID:   "anthropic-1",
		EventType:    EventProviderResponse,
		LatencyMS:    142,
		InputTokens:  500,
		OutputTokens: 200,
		Success:      &success,
	})

	logger.Log(Event{
		EventType: EventQueryStart,
	})

	// Close to flush writes.
	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading telemetry file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d:\n%s", len(lines), string(data))
	}

	var ev Event
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatalf("unmarshal line 0: %v", err)
	}
	if ev.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
	if ev.EventType != EventProviderResponse {
		t.Errorf("expected event_type %q, got %q", EventProviderResponse, ev.EventType)
	}
	if ev.ProviderID != "anthropic-1" {
		t.Errorf("expected provider_id %q, got %q", "anthropic-1", ev.ProviderID)
	}
	if ev.LatencyMS != 142 {
		t.Errorf("expected latency_ms 142, got %d", ev.LatencyMS)
	}
	if ev.Success == nil || !*ev.Success {
		t.Error("expected success to be true")
	}
}

func TestLogConcurrentSafety(t *testing.T) {
	cfgDir := setTestConfigDir(t)

	logger, err := NewLogger()
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			logger.Log(Event{
				EventType:  EventToolExecuted,
				ToolName:   "file_read",
				LatencyMS:  10,
				ProviderID: "test",
			})
		}()
	}
	wg.Wait()

	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "telemetry.jsonl"))
	if err != nil {
		t.Fatalf("reading telemetry file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != goroutines {
		t.Fatalf("expected %d lines, got %d", goroutines, len(lines))
	}

	// Verify every line is valid JSON.
	for i, line := range lines {
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
		}
	}
}

func TestFileRotationTruncatesOverLimit(t *testing.T) {
	cfgDir := setTestConfigDir(t)
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(cfgDir, "telemetry.jsonl")

	// Build a file that is ~11 MB: a known sentinel line at the end that
	// must survive rotation, preceded by padding.
	padding := strings.Repeat("x", 1024) // 1 KB per line
	var b strings.Builder

	// Write enough padding to exceed 10 MB.
	linesNeeded := (11 * 1024 * 1024) / (len(padding) + 1) // +1 for newline
	for i := 0; i < linesNeeded; i++ {
		b.WriteString(padding)
		b.WriteByte('\n')
	}

	sentinel := `{"event_type":"sentinel"}` + "\n"
	b.WriteString(sentinel)

	if err := os.WriteFile(path, []byte(b.String()), 0600); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() <= int64(maxFileSize) {
		t.Fatalf("setup error: file should exceed 10 MB, got %d bytes", info.Size())
	}

	// Opening a new logger should trigger rotation.
	logger, err := NewLogger()
	if err != nil {
		t.Fatalf("NewLogger after rotation: %v", err)
	}
	defer logger.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) > maxFileSize {
		t.Errorf("file should be <= %d bytes after rotation, got %d", maxFileSize, len(data))
	}

	if !strings.Contains(string(data), `{"event_type":"sentinel"}`) {
		t.Error("sentinel line at end of file was lost during rotation")
	}
}
