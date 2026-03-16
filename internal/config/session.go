package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ToolCallRecord is a serializable tool call.
type ToolCallRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolResultRecord is a serializable tool result.
type ToolResultRecord struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
}

// SessionMessage is a serializable conversation message.
type SessionMessage struct {
	Role       string            `json:"role"`
	Content    string            `json:"content"`
	ToolCalls  []ToolCallRecord  `json:"tool_calls,omitempty"`
	ToolResult *ToolResultRecord `json:"tool_result,omitempty"`
}

// SessionExchange is a serializable prompt/response pair for display history.
type SessionExchange struct {
	Prompt            string            `json:"prompt"`
	ConsensusResponse string            `json:"consensus_response"`
	Individual        map[string]string `json:"individual,omitempty"`
}

// Session holds the full conversation state for persistence.
type Session struct {
	Messages  []SessionMessage  `json:"messages"`
	Exchanges []SessionExchange `json:"exchanges"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// SessionPath returns the path to the session file.
func SessionPath() string {
	return filepath.Join(ConfigDir(), "session.json")
}

// LoadSession reads a saved session from disk.
// Returns nil (no error) if no session file exists.
func LoadSession() (*Session, error) {
	data, err := os.ReadFile(SessionPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading session: %w", err)
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing session: %w", err)
	}

	return &s, nil
}

// SaveSession writes the session to disk.
func SaveSession(s *Session) error {
	s.UpdatedAt = time.Now()

	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}

	return os.WriteFile(SessionPath(), data, 0600)
}

// ClearSession removes the saved session file.
func ClearSession() error {
	err := os.Remove(SessionPath())
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing session: %w", err)
	}
	return nil
}
