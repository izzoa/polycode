package action

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/izzoa/polycode/internal/provider"
)

// mockToolProvider returns scripted responses, including tool calls.
type mockToolProvider struct {
	id        string
	responses []mockResponse // consumed in order
	callIndex int
}

type mockResponse struct {
	content   string
	toolCalls []provider.ToolCall
}

func (m *mockToolProvider) ID() string          { return m.id }
func (m *mockToolProvider) Authenticate() error { return nil }
func (m *mockToolProvider) Validate() error     { return nil }

func (m *mockToolProvider) Query(ctx context.Context, messages []provider.Message, opts provider.QueryOpts) (<-chan provider.StreamChunk, error) {
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

func TestToolLoopFileRead(t *testing.T) {
	// Create a temp file to read
	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.txt"
	if err := writeTestFile(testFile, "hello world"); err != nil {
		t.Fatal(err)
	}

	// Provider first returns a file_read tool call, then a final text response
	args, _ := json.Marshal(map[string]string{"path": testFile})
	prov := &mockToolProvider{
		id: "test",
		responses: []mockResponse{
			{toolCalls: []provider.ToolCall{{ID: "call_1", Name: "file_read", Arguments: string(args)}}},
			{content: "The file contains: hello world"},
		},
	}

	// Auto-confirm everything
	confirm := ConfirmFunc(func(desc string) bool { return true })
	executor := NewExecutor(confirm, 10*time.Second)
	loop := NewToolLoop(executor, prov)

	msgs := []provider.Message{{Role: provider.RoleUser, Content: "Read the test file"}}
	toolCalls := prov.responses[0].toolCalls

	stream, err := loop.Run(context.Background(), msgs, toolCalls, provider.QueryOpts{})
	if err != nil {
		t.Fatalf("tool loop failed: %v", err)
	}

	var result string
	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		result += chunk.Delta
	}

	if result != "The file contains: hello world" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestToolLoopShellExec(t *testing.T) {
	args, _ := json.Marshal(map[string]string{"command": "echo hello_from_shell"})
	prov := &mockToolProvider{
		id: "test",
		responses: []mockResponse{
			{toolCalls: []provider.ToolCall{{ID: "call_1", Name: "shell_exec", Arguments: string(args)}}},
			{content: "Shell said: hello_from_shell"},
		},
	}

	confirm := ConfirmFunc(func(desc string) bool { return true })
	executor := NewExecutor(confirm, 10*time.Second)
	loop := NewToolLoop(executor, prov)

	msgs := []provider.Message{{Role: provider.RoleUser, Content: "Run echo"}}
	toolCalls := prov.responses[0].toolCalls

	stream, err := loop.Run(context.Background(), msgs, toolCalls, provider.QueryOpts{})
	if err != nil {
		t.Fatalf("tool loop failed: %v", err)
	}

	var result string
	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		result += chunk.Delta
	}

	if result == "" {
		t.Error("expected non-empty result from shell exec tool loop")
	}
}

func TestToolLoopRejectedAction(t *testing.T) {
	args, _ := json.Marshal(map[string]string{"command": "rm -rf /"})
	prov := &mockToolProvider{
		id: "test",
		responses: []mockResponse{
			{toolCalls: []provider.ToolCall{{ID: "call_1", Name: "shell_exec", Arguments: string(args)}}},
			{content: "Action was rejected by user"},
		},
	}

	// Always reject
	confirm := ConfirmFunc(func(desc string) bool { return false })
	executor := NewExecutor(confirm, 10*time.Second)
	loop := NewToolLoop(executor, prov)

	msgs := []provider.Message{{Role: provider.RoleUser, Content: "Delete everything"}}
	toolCalls := prov.responses[0].toolCalls

	stream, err := loop.Run(context.Background(), msgs, toolCalls, provider.QueryOpts{})
	if err != nil {
		t.Fatalf("tool loop failed: %v", err)
	}

	var result string
	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		result += chunk.Delta
	}

	// The model should have received the rejection and responded
	if result == "" {
		t.Error("expected response after rejected action")
	}
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
