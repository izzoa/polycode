package telemetry

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/izzoa/polycode/internal/config"
)

// maxFileSize is the threshold at which the telemetry file is truncated on open.
const maxFileSize = 10 * 1024 * 1024 // 10 MB

// keepSize is the amount of data retained from the tail after truncation.
const keepSize = 5 * 1024 * 1024 // 5 MB

// EventType identifies the kind of telemetry event.
type EventType string

const (
	EventProviderResponse  EventType = "provider_response"
	EventConsensusComplete EventType = "consensus_complete"
	EventToolExecuted      EventType = "tool_executed"
	EventPipelineError     EventType = "pipeline_error"
	EventQueryStart        EventType = "query_start"
	EventUserFeedback      EventType = "user_feedback"
)

// Event is a single telemetry record written as one JSON line.
type Event struct {
	Timestamp    string    `json:"timestamp"`
	ProviderID   string    `json:"provider_id,omitempty"`
	EventType    EventType `json:"event_type"`
	LatencyMS    int64     `json:"latency_ms,omitempty"`
	InputTokens  int       `json:"input_tokens,omitempty"`
	OutputTokens int       `json:"output_tokens,omitempty"`
	ToolName     string    `json:"tool_name,omitempty"`
	Success      *bool     `json:"success,omitempty"`
	Error        string    `json:"error,omitempty"`
	Accepted     *bool     `json:"accepted,omitempty"` // user feedback: tool accepted/rejected
}

// Logger appends telemetry events to a JSONL file.
type Logger struct {
	file *os.File
	mu   sync.Mutex
}

// NewLogger opens (or creates) the telemetry log file at
// <ConfigDir>/telemetry.jsonl. If the file exceeds 10 MB it is
// truncated to the last 5 MB before the logger begins appending.
func NewLogger() (*Logger, error) {
	dir := config.ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	path := filepath.Join(dir, "telemetry.jsonl")

	// Rotate if the existing file is too large.
	if err := rotateIfNeeded(path); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}

	return &Logger{file: f}, nil
}

// Log records a single event. It sets the timestamp automatically and is
// safe for concurrent use.
func (l *Logger) Log(event Event) {
	event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)

	data, err := json.Marshal(event)
	if err != nil {
		// Best-effort: silently drop events that cannot be marshaled.
		return
	}
	data = append(data, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.file.Write(data)
}

// Close flushes and closes the underlying file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// rotateIfNeeded checks whether path exceeds maxFileSize and, if so,
// rewrites it with only the last keepSize bytes (aligned to a newline
// boundary so partial JSON lines are not kept).
func rotateIfNeeded(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to rotate
		}
		return err
	}

	if info.Size() <= int64(maxFileSize) {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Seek to (end - keepSize) and read from there.
	offset := info.Size() - int64(keepSize)
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return err
	}

	tail, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	// Drop the first (likely partial) line so we start on a clean boundary.
	for i := 0; i < len(tail); i++ {
		if tail[i] == '\n' {
			tail = tail[i+1:]
			break
		}
	}

	return os.WriteFile(path, tail, 0600)
}
