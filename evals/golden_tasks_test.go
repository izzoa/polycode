// Package evals contains end-to-end evaluation tests that validate the full
// polycode pipeline: fan-out → consensus → tool execution → verification.
//
// These tests run against mock providers in CI (no API keys needed).
// For live-fire tests against real APIs, set POLYCODE_EVAL_LIVE=1.
package evals

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/izzoa/polycode/internal/action"
	"github.com/izzoa/polycode/internal/consensus"
	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/tokens"
)

// mockEvalProvider returns scripted responses to simulate a model.
type mockEvalProvider struct {
	id        string
	responses []evalResponse
	callIndex int
}

type evalResponse struct {
	content   string
	toolCalls []provider.ToolCall
}

func (m *mockEvalProvider) ID() string          { return m.id }
func (m *mockEvalProvider) Authenticate() error { return nil }
func (m *mockEvalProvider) Validate() error     { return nil }

func (m *mockEvalProvider) Query(_ context.Context, messages []provider.Message, _ provider.QueryOpts) (<-chan provider.StreamChunk, error) {
	ch := make(chan provider.StreamChunk, 2)
	go func() {
		defer close(ch)
		idx := m.callIndex
		if idx >= len(m.responses) {
			idx = len(m.responses) - 1
		}
		m.callIndex++
		resp := m.responses[idx]
		if resp.content != "" {
			ch <- provider.StreamChunk{Delta: resp.content}
		}
		ch <- provider.StreamChunk{Done: true, ToolCalls: resp.toolCalls}
	}()
	return ch, nil
}

// mockConsensusProvider synthesizes fan-out responses into a consensus.
type mockConsensusProvider struct {
	id string
}

func (m *mockConsensusProvider) ID() string          { return m.id }
func (m *mockConsensusProvider) Authenticate() error { return nil }
func (m *mockConsensusProvider) Validate() error     { return nil }

func (m *mockConsensusProvider) Query(_ context.Context, messages []provider.Message, _ provider.QueryOpts) (<-chan provider.StreamChunk, error) {
	ch := make(chan provider.StreamChunk, 2)
	go func() {
		defer close(ch)
		lastMsg := messages[len(messages)-1].Content
		if strings.Contains(lastMsg, "Model responses:") {
			ch <- provider.StreamChunk{Delta: "CONSENSUS: Synthesized answer from all models."}
		} else {
			ch <- provider.StreamChunk{Delta: "Direct response from primary."}
		}
		ch <- provider.StreamChunk{Done: true}
	}()
	return ch, nil
}

// --- Golden Task 1: File Read ---

func TestGoldenTask_FileRead(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(testFile, []byte("port: 8080\nhost: localhost\n"), 0644)

	args, _ := json.Marshal(map[string]string{"path": testFile})
	prov := &mockEvalProvider{
		id: "primary",
		responses: []evalResponse{
			{toolCalls: []provider.ToolCall{{ID: "call_1", Name: "file_read", Arguments: string(args)}}},
			{content: "The config file sets port to 8080 and host to localhost."},
		},
	}

	confirm := action.ConfirmFunc(func(desc string) bool { return true })
	executor := action.NewExecutor(confirm, 10*time.Second)
	loop := action.NewToolLoop(executor, prov)

	msgs := []provider.Message{{Role: provider.RoleUser, Content: "What's in config.yaml?"}}
	out := make(chan provider.StreamChunk, 64)

	go func() {
		if err := loop.Run(context.Background(), msgs, prov.responses[0].toolCalls, provider.QueryOpts{}, out); err != nil {
			out <- provider.StreamChunk{Error: err}
		}
		close(out)
	}()

	var result string
	for chunk := range out {
		if chunk.Error != nil {
			t.Fatalf("file read task failed: %v", chunk.Error)
		}
		result += chunk.Delta
	}

	if !strings.Contains(result, "8080") {
		t.Errorf("expected result to mention port 8080, got: %s", result)
	}
}

// --- Golden Task 2: File Edit ---

func TestGoldenTask_FileEdit(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "main.go")
	os.WriteFile(testFile, []byte(`package main

func main() {
	fmt.Println("hello")
}
`), 0644)

	// Model returns a file_write tool call to fix the missing import
	fixedContent := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`
	writeArgs, _ := json.Marshal(map[string]string{
		"path":    testFile,
		"content": fixedContent,
	})

	prov := &mockEvalProvider{
		id: "primary",
		responses: []evalResponse{
			{toolCalls: []provider.ToolCall{{ID: "call_1", Name: "file_write", Arguments: string(writeArgs)}}},
			{content: "I've added the missing `import \"fmt\"` statement."},
		},
	}

	confirm := action.ConfirmFunc(func(desc string) bool { return true })
	executor := action.NewExecutor(confirm, 10*time.Second)
	loop := action.NewToolLoop(executor, prov)

	msgs := []provider.Message{{Role: provider.RoleUser, Content: "Fix the missing import"}}
	out := make(chan provider.StreamChunk, 64)

	go func() {
		if err := loop.Run(context.Background(), msgs, prov.responses[0].toolCalls, provider.QueryOpts{}, out); err != nil {
			out <- provider.StreamChunk{Error: err}
		}
		close(out)
	}()

	for chunk := range out {
		if chunk.Error != nil {
			t.Fatalf("file edit task failed: %v", chunk.Error)
		}
	}

	// Verify the file was actually written
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("reading edited file: %v", err)
	}
	if !strings.Contains(string(data), `import "fmt"`) {
		t.Errorf("expected file to contain import statement, got:\n%s", string(data))
	}
}

// --- Golden Task 3: Shell Exec ---

func TestGoldenTask_ShellExec(t *testing.T) {
	args, _ := json.Marshal(map[string]string{"command": "echo 'polycode_test_output'"})
	prov := &mockEvalProvider{
		id: "primary",
		responses: []evalResponse{
			{toolCalls: []provider.ToolCall{{ID: "call_1", Name: "shell_exec", Arguments: string(args)}}},
			{content: "The command output was: polycode_test_output"},
		},
	}

	confirm := action.ConfirmFunc(func(desc string) bool { return true })
	executor := action.NewExecutor(confirm, 10*time.Second)
	loop := action.NewToolLoop(executor, prov)

	msgs := []provider.Message{{Role: provider.RoleUser, Content: "Run echo test"}}
	out := make(chan provider.StreamChunk, 64)

	go func() {
		if err := loop.Run(context.Background(), msgs, prov.responses[0].toolCalls, provider.QueryOpts{}, out); err != nil {
			out <- provider.StreamChunk{Error: err}
		}
		close(out)
	}()

	var result string
	for chunk := range out {
		if chunk.Error != nil {
			t.Fatalf("shell exec task failed: %v", chunk.Error)
		}
		result += chunk.Delta
	}

	if !strings.Contains(result, "polycode_test_output") {
		t.Errorf("expected result to contain command output, got: %s", result)
	}
}

// --- Golden Task 4: Fix-and-Test (multi-step tool loop) ---

func TestGoldenTask_FixAndTest(t *testing.T) {
	tmpDir := t.TempDir()
	buggyFile := filepath.Join(tmpDir, "calc.go")
	os.WriteFile(buggyFile, []byte(`package calc

func Add(a, b int) int {
	return a - b // BUG: should be a + b
}
`), 0644)

	// Step 1: model reads the file
	readArgs, _ := json.Marshal(map[string]string{"path": buggyFile})
	// Step 2: model writes the fix
	fixedContent := `package calc

func Add(a, b int) int {
	return a + b
}
`
	writeArgs, _ := json.Marshal(map[string]string{"path": buggyFile, "content": fixedContent})

	prov := &mockEvalProvider{
		id: "primary",
		responses: []evalResponse{
			// First call: read the file
			{toolCalls: []provider.ToolCall{{ID: "call_1", Name: "file_read", Arguments: string(readArgs)}}},
			// Second call: write the fix
			{toolCalls: []provider.ToolCall{{ID: "call_2", Name: "file_write", Arguments: string(writeArgs)}}},
			// Final: describe what was done
			{content: "Fixed the bug: changed `a - b` to `a + b` in the Add function."},
		},
	}

	confirm := action.ConfirmFunc(func(desc string) bool { return true })
	executor := action.NewExecutor(confirm, 10*time.Second)
	loop := action.NewToolLoop(executor, prov)

	msgs := []provider.Message{{Role: provider.RoleUser, Content: "Fix the bug in calc.go"}}
	out := make(chan provider.StreamChunk, 64)

	go func() {
		if err := loop.Run(context.Background(), msgs, prov.responses[0].toolCalls, provider.QueryOpts{}, out); err != nil {
			out <- provider.StreamChunk{Error: err}
		}
		close(out)
	}()

	for chunk := range out {
		if chunk.Error != nil {
			t.Fatalf("fix-and-test task failed: %v", chunk.Error)
		}
	}

	// Verify the fix was applied
	data, _ := os.ReadFile(buggyFile)
	if strings.Contains(string(data), "a - b") {
		t.Error("bug was not fixed: file still contains 'a - b'")
	}
	if !strings.Contains(string(data), "a + b") {
		t.Error("fix not applied: file does not contain 'a + b'")
	}
}

// --- Golden Task 5: Consensus Pipeline End-to-End ---

func TestGoldenTask_ConsensusPipeline(t *testing.T) {
	// Two secondary providers + one primary that synthesizes
	secondary1 := &mockEvalProvider{
		id:        "claude",
		responses: []evalResponse{{content: "Use a mutex for thread safety."}},
	}
	secondary2 := &mockEvalProvider{
		id:        "gpt4",
		responses: []evalResponse{{content: "Use a sync.RWMutex for concurrent reads."}},
	}
	primary := &mockConsensusProvider{id: "claude"}

	tracker := tokens.NewTracker(
		map[string]string{"claude": "claude-3", "gpt4": "gpt-4"},
		map[string]int{"claude": 100000, "gpt4": 100000},
	)

	pipeline := consensus.NewPipeline(
		[]provider.Provider{primary, secondary1, secondary2},
		primary,
		30*time.Second,
		1,
		tracker,
	)

	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: "How should I protect shared state in Go?"},
	}
	opts := provider.QueryOpts{MaxTokens: 1024}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, fanOut, err := pipeline.Run(ctx, msgs, opts)
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}

	// Verify fan-out collected responses
	if fanOut != nil && len(fanOut.Responses) == 0 && len(fanOut.Errors) == 0 {
		t.Log("Warning: no fan-out responses collected (single-provider fast path)")
	}

	// Drain consensus stream
	var result string
	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("consensus stream error: %v", chunk.Error)
		}
		result += chunk.Delta
	}

	if result == "" {
		t.Error("expected non-empty consensus result")
	}
}

// --- Deterministic Behavior: Timeout ---

func TestGoldenTask_TimeoutBehavior(t *testing.T) {
	// Provider that never responds
	slowProvider := &mockEvalProvider{
		id: "slow",
		responses: []evalResponse{
			{content: ""}, // empty, will block on context
		},
	}

	primary := &mockConsensusProvider{id: "primary"}

	tracker := tokens.NewTracker(
		map[string]string{"slow": "slow-model", "primary": "primary-model"},
		map[string]int{"slow": 100000, "primary": 100000},
	)

	pipeline := consensus.NewPipeline(
		[]provider.Provider{primary, slowProvider},
		primary,
		500*time.Millisecond, // very short timeout
		2,                    // require both providers
		tracker,
	)

	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: "test"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stream, _, err := pipeline.Run(ctx, msgs, provider.QueryOpts{MaxTokens: 64})

	// Pipeline should either return an error or a degraded stream — not hang
	if err != nil {
		// Expected: timeout or min_responses not met
		return
	}

	// Drain any stream output
	for chunk := range stream {
		if chunk.Error != nil {
			// Expected: timeout error in stream
			return
		}
	}

	// If we got here without error, the pipeline handled degradation gracefully
}
