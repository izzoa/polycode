package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// ConsensusTrace captures the full fan-out/synthesis data for replay and comparison.
type ConsensusTrace struct {
	RoutingMode     string            `json:"routing_mode,omitempty"`
	RoutingReason   string            `json:"routing_reason,omitempty"`
	Providers       []string          `json:"providers,omitempty"`
	Latencies       map[string]int64  `json:"latencies_ms,omitempty"` // provider → ms
	TokenUsage      map[string][2]int `json:"token_usage,omitempty"`  // provider → [input, output]
	Errors          map[string]string `json:"errors,omitempty"`
	Skipped         []string          `json:"skipped,omitempty"`
	SynthesisModel  string            `json:"synthesis_model,omitempty"`
}

// ProviderTraceSection is a serializable trace section for one phase of
// provider activity within a single turn.
type ProviderTraceSection struct {
	Phase   string `json:"phase"`
	Content string `json:"content"`
}

// SessionExchange is a serializable prompt/response pair for display history.
type SessionExchange struct {
	Prompt            string                              `json:"prompt"`
	ConsensusResponse string                              `json:"consensus_response"`
	Individual        map[string]string                   `json:"individual,omitempty"`
	ProviderTraces    map[string][]ProviderTraceSection    `json:"provider_traces,omitempty"`
	Trace             *ConsensusTrace                     `json:"trace,omitempty"`
}

// Session holds the full conversation state for persistence.
type Session struct {
	Name      string            `json:"name,omitempty"` // user-assigned name (empty = default)
	Messages  []SessionMessage  `json:"messages"`
	Exchanges []SessionExchange `json:"exchanges"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// SessionPath returns the path to the session file, scoped to the current
// working directory so each project gets its own conversation history.
func SessionPath() string {
	wd, err := os.Getwd()
	if err != nil {
		// Fallback to a global session if we can't determine the working dir
		return filepath.Join(ConfigDir(), "sessions", "global.json")
	}
	hash := sha256.Sum256([]byte(wd))
	name := hex.EncodeToString(hash[:8]) // 16-char hex = enough uniqueness
	return filepath.Join(ConfigDir(), "sessions", name+".json")
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

// SaveSession writes the session to disk atomically using a temp file + rename
// to prevent corruption from crashes during write.
func SaveSession(s *Session) error {
	s.UpdatedAt = time.Now()

	sessionFile := SessionPath()
	dir := filepath.Dir(sessionFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating session dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}

	// Write to temp file then rename for atomicity.
	tmp, err := os.CreateTemp(dir, ".session-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp session file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing temp session file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp session file: %w", err)
	}

	if err := os.Rename(tmpName, sessionFile); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming temp session file: %w", err)
	}

	return nil
}

// ClearSession removes the saved session file.
func ClearSession() error {
	err := os.Remove(SessionPath())
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing session: %w", err)
	}
	return nil
}

// SessionInfo holds metadata about a saved session for listing.
type SessionInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	UpdatedAt time.Time `json:"updated_at"`
	Exchanges int       `json:"exchanges"`
	IsCurrent bool      `json:"is_current"`
}

// ListSessions returns info about all saved sessions in the sessions directory.
func ListSessions() ([]SessionInfo, error) {
	dir := filepath.Join(ConfigDir(), "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading sessions dir: %w", err)
	}

	currentPath := SessionPath()
	var sessions []SessionInfo

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var s Session
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}

		name := s.Name
		if name == "" {
			name = strings.TrimSuffix(entry.Name(), ".json")
		}

		sessions = append(sessions, SessionInfo{
			Name:      name,
			Path:      path,
			UpdatedAt: s.UpdatedAt,
			Exchanges: len(s.Exchanges),
			IsCurrent: path == currentPath,
		})
	}

	return sessions, nil
}

// LoadSessionByName loads a session by its user-assigned name.
// Scans all session files in the sessions directory.
func LoadSessionByName(name string) (*Session, string, error) {
	dir := filepath.Join(ConfigDir(), "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, "", fmt.Errorf("reading sessions dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var s Session
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		matchName := s.Name
		if matchName == "" {
			matchName = strings.TrimSuffix(entry.Name(), ".json")
		}
		if matchName == name {
			return &s, path, nil
		}
	}

	return nil, "", fmt.Errorf("session %q not found", name)
}

// DeleteSessionByName deletes a session by name.
func DeleteSessionByName(name string) error {
	_, path, err := LoadSessionByName(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// ExportSession writes the session as JSON to the given file path.
func ExportSession(s *Session, path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling session: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing export: %w", err)
	}
	return nil
}
