package config

import (
	"os"
	"testing"
)

func TestSessionToolCallRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	original := &Session{
		Messages: []SessionMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Read the file main.go"},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ToolCallRecord{
					{
						ID:        "call_001",
						Name:      "file_read",
						Arguments: `{"path":"main.go"}`,
					},
				},
			},
			{
				Role:    "tool",
				Content: "",
				ToolResult: &ToolResultRecord{
					ToolCallID: "call_001",
					Output:     "package main\n\nfunc main() {}",
				},
			},
			{Role: "assistant", Content: "The file main.go contains a minimal Go program."},
		},
	}

	if err := SaveSession(original); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	loaded, err := LoadSession()
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadSession returned nil")
	}

	if len(loaded.Messages) != len(original.Messages) {
		t.Fatalf("expected %d messages, got %d", len(original.Messages), len(loaded.Messages))
	}

	// Verify system message
	if loaded.Messages[0].Role != "system" || loaded.Messages[0].Content != "You are a helpful assistant." {
		t.Errorf("system message mismatch: %+v", loaded.Messages[0])
	}

	// Verify user message
	if loaded.Messages[1].Role != "user" || loaded.Messages[1].Content != "Read the file main.go" {
		t.Errorf("user message mismatch: %+v", loaded.Messages[1])
	}

	// Verify assistant message with tool calls
	assistantMsg := loaded.Messages[2]
	if assistantMsg.Role != "assistant" {
		t.Errorf("expected assistant role, got %q", assistantMsg.Role)
	}
	if len(assistantMsg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(assistantMsg.ToolCalls))
	}
	tc := assistantMsg.ToolCalls[0]
	if tc.ID != "call_001" {
		t.Errorf("tool call ID: expected call_001, got %q", tc.ID)
	}
	if tc.Name != "file_read" {
		t.Errorf("tool call name: expected file_read, got %q", tc.Name)
	}
	if tc.Arguments != `{"path":"main.go"}` {
		t.Errorf("tool call arguments mismatch: %q", tc.Arguments)
	}

	// Verify tool result message
	toolMsg := loaded.Messages[3]
	if toolMsg.Role != "tool" {
		t.Errorf("expected tool role, got %q", toolMsg.Role)
	}
	if toolMsg.ToolResult == nil {
		t.Fatal("expected non-nil ToolResult")
	}
	if toolMsg.ToolResult.ToolCallID != "call_001" {
		t.Errorf("tool result call ID: expected call_001, got %q", toolMsg.ToolResult.ToolCallID)
	}
	if toolMsg.ToolResult.Output != "package main\n\nfunc main() {}" {
		t.Errorf("tool result output mismatch: %q", toolMsg.ToolResult.Output)
	}
	if toolMsg.ToolResult.Error != "" {
		t.Errorf("expected empty error, got %q", toolMsg.ToolResult.Error)
	}

	// Verify final assistant message
	if loaded.Messages[4].Role != "assistant" || loaded.Messages[4].Content != "The file main.go contains a minimal Go program." {
		t.Errorf("final assistant message mismatch: %+v", loaded.Messages[4])
	}

	// Verify omitempty: tool calls and tool result should be nil/empty on plain messages
	if len(loaded.Messages[0].ToolCalls) != 0 {
		t.Error("system message should have no tool calls")
	}
	if loaded.Messages[0].ToolResult != nil {
		t.Error("system message should have nil tool result")
	}
}

func TestSessionToolCallWithError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	original := &Session{
		Messages: []SessionMessage{
			{Role: "user", Content: "Run tests"},
			{
				Role: "assistant",
				ToolCalls: []ToolCallRecord{
					{ID: "call_010", Name: "shell_exec", Arguments: `{"command":"go test ./..."}`},
				},
			},
			{
				Role: "tool",
				ToolResult: &ToolResultRecord{
					ToolCallID: "call_010",
					Output:     "FAIL main_test.go:12",
					Error:      "exit status 1",
				},
			},
			{Role: "assistant", Content: "The tests failed with exit status 1."},
		},
	}

	if err := SaveSession(original); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	loaded, err := LoadSession()
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	tr := loaded.Messages[2].ToolResult
	if tr == nil {
		t.Fatal("expected non-nil ToolResult")
	}
	if tr.Error != "exit status 1" {
		t.Errorf("expected error 'exit status 1', got %q", tr.Error)
	}
	if tr.Output != "FAIL main_test.go:12" {
		t.Errorf("expected output preserved alongside error, got %q", tr.Output)
	}
}

func TestSessionMultipleToolCalls(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	original := &Session{
		Messages: []SessionMessage{
			{Role: "user", Content: "Read both files"},
			{
				Role: "assistant",
				ToolCalls: []ToolCallRecord{
					{ID: "call_a", Name: "file_read", Arguments: `{"path":"a.go"}`},
					{ID: "call_b", Name: "file_read", Arguments: `{"path":"b.go"}`},
				},
			},
			{
				Role:       "tool",
				ToolResult: &ToolResultRecord{ToolCallID: "call_a", Output: "package a"},
			},
			{
				Role:       "tool",
				ToolResult: &ToolResultRecord{ToolCallID: "call_b", Output: "package b"},
			},
			{Role: "assistant", Content: "I read both files."},
		},
	}

	if err := SaveSession(original); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	loaded, err := LoadSession()
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	if len(loaded.Messages) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(loaded.Messages))
	}

	// Two tool calls in single assistant message
	if len(loaded.Messages[1].ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(loaded.Messages[1].ToolCalls))
	}
	if loaded.Messages[1].ToolCalls[0].ID != "call_a" || loaded.Messages[1].ToolCalls[1].ID != "call_b" {
		t.Error("tool call IDs not preserved")
	}

	// Two separate tool result messages
	if loaded.Messages[2].ToolResult.ToolCallID != "call_a" {
		t.Errorf("expected call_a, got %q", loaded.Messages[2].ToolResult.ToolCallID)
	}
	if loaded.Messages[3].ToolResult.ToolCallID != "call_b" {
		t.Errorf("expected call_b, got %q", loaded.Messages[3].ToolResult.ToolCallID)
	}
}

func TestClearSessionRemovesFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	s := &Session{
		Messages: []SessionMessage{
			{Role: "user", Content: "hello"},
		},
	}
	if err := SaveSession(s); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(SessionPath()); os.IsNotExist(err) {
		t.Fatal("session file should exist after save")
	}

	if err := ClearSession(); err != nil {
		t.Fatalf("ClearSession failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(SessionPath()); !os.IsNotExist(err) {
		t.Fatal("session file should not exist after clear")
	}

	// LoadSession should return nil after clear
	loaded, err := LoadSession()
	if err != nil {
		t.Fatalf("LoadSession after clear failed: %v", err)
	}
	if loaded != nil {
		t.Fatal("expected nil session after clear")
	}
}

func TestClearSessionNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Clearing when no file exists should not error
	if err := ClearSession(); err != nil {
		t.Fatalf("ClearSession with no file should not error: %v", err)
	}
}

func TestProviderTraceRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	original := &Session{
		Messages: []SessionMessage{
			{Role: "user", Content: "explain Go"},
		},
		Exchanges: []SessionExchange{
			{
				Prompt:            "explain Go",
				ConsensusResponse: "Go is great.",
				Individual: map[string]string{
					"claude": "Go is a language.",
					"gpt4":   "Go is compiled.",
				},
				ProviderTraces: map[string][]ProviderTraceSection{
					"claude": {
						{Phase: "fanout", Content: "Go is a language."},
						{Phase: "synthesis", Content: "Go is great."},
						{Phase: "tool", Content: "Running go test..."},
						{Phase: "verify", Content: "Verification passed."},
					},
					"gpt4": {
						{Phase: "fanout", Content: "Go is compiled."},
					},
				},
			},
		},
	}

	if err := SaveSession(original); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	loaded, err := LoadSession()
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadSession returned nil")
	}

	if len(loaded.Exchanges) != 1 {
		t.Fatalf("expected 1 exchange, got %d", len(loaded.Exchanges))
	}

	ex := loaded.Exchanges[0]

	// Verify provider traces are preserved
	if len(ex.ProviderTraces) != 2 {
		t.Fatalf("expected 2 provider traces, got %d", len(ex.ProviderTraces))
	}

	claudeTraces := ex.ProviderTraces["claude"]
	if len(claudeTraces) != 4 {
		t.Fatalf("expected 4 claude trace sections, got %d", len(claudeTraces))
	}
	if claudeTraces[0].Phase != "fanout" || claudeTraces[0].Content != "Go is a language." {
		t.Errorf("claude section 0 mismatch: %+v", claudeTraces[0])
	}
	if claudeTraces[1].Phase != "synthesis" || claudeTraces[1].Content != "Go is great." {
		t.Errorf("claude section 1 mismatch: %+v", claudeTraces[1])
	}
	if claudeTraces[2].Phase != "tool" || claudeTraces[2].Content != "Running go test..." {
		t.Errorf("claude section 2 mismatch: %+v", claudeTraces[2])
	}
	if claudeTraces[3].Phase != "verify" || claudeTraces[3].Content != "Verification passed." {
		t.Errorf("claude section 3 mismatch: %+v", claudeTraces[3])
	}

	gptTraces := ex.ProviderTraces["gpt4"]
	if len(gptTraces) != 1 {
		t.Fatalf("expected 1 gpt4 trace section, got %d", len(gptTraces))
	}
	if gptTraces[0].Phase != "fanout" {
		t.Errorf("gpt4 phase = %q, want %q", gptTraces[0].Phase, "fanout")
	}

	// Verify Individual is also preserved (backward compat)
	if ex.Individual["claude"] != "Go is a language." {
		t.Errorf("individual claude = %q", ex.Individual["claude"])
	}
}

func TestLegacySessionWithoutTracesLoads(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Save a session with only Individual (no ProviderTraces) — simulates legacy format
	legacy := &Session{
		Messages: []SessionMessage{
			{Role: "user", Content: "hello"},
		},
		Exchanges: []SessionExchange{
			{
				Prompt:            "hello",
				ConsensusResponse: "Hi there!",
				Individual: map[string]string{
					"provider1": "Hello!",
				},
				// No ProviderTraces — legacy session
			},
		},
	}

	if err := SaveSession(legacy); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	loaded, err := LoadSession()
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadSession returned nil")
	}

	ex := loaded.Exchanges[0]
	if ex.ProviderTraces != nil && len(ex.ProviderTraces) > 0 {
		t.Error("legacy session should have nil/empty ProviderTraces")
	}
	if ex.Individual["provider1"] != "Hello!" {
		t.Errorf("legacy Individual should be preserved: %q", ex.Individual["provider1"])
	}
}
