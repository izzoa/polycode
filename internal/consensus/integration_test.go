package consensus

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/izzoa/polycode/internal/provider"
)

// mockPrimaryProvider responds with a synthesis of whatever is in the prompt.
type mockPrimaryProvider struct {
	id string
}

func (m *mockPrimaryProvider) ID() string          { return m.id }
func (m *mockPrimaryProvider) Authenticate() error { return nil }
func (m *mockPrimaryProvider) Validate() error     { return nil }

func (m *mockPrimaryProvider) Query(ctx context.Context, messages []provider.Message, opts provider.QueryOpts) (<-chan provider.StreamChunk, error) {
	ch := make(chan provider.StreamChunk, 2)
	go func() {
		defer close(ch)
		// Simulate synthesis: echo back that we saw the model responses
		lastMsg := messages[len(messages)-1].Content
		if strings.Contains(lastMsg, "Model responses:") {
			ch <- provider.StreamChunk{Delta: "CONSENSUS: Combined insights from all models."}
		} else {
			ch <- provider.StreamChunk{Delta: "Direct response from primary."}
		}
		ch <- provider.StreamChunk{Done: true}
	}()
	return ch, nil
}

func TestPipelineEndToEnd(t *testing.T) {
	primary := &mockPrimaryProvider{id: "primary"}
	secondary := &mockProvider{id: "secondary", response: "Secondary says: use a hashmap."}

	providers := []provider.Provider{primary, secondary}

	pipeline := NewPipeline(providers, primary, 5*time.Second, 2, nil)

	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "What data structure should I use?"},
	}

	stream, fanOut, err := pipeline.Run(context.Background(), messages, provider.QueryOpts{})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	// Check fan-out results
	if fanOut == nil {
		t.Fatal("expected non-nil fan-out result")
	}
	if _, ok := fanOut.Responses["secondary"]; !ok {
		t.Error("expected response from secondary provider")
	}
	if _, ok := fanOut.Responses["primary"]; !ok {
		t.Error("expected response from primary provider")
	}

	// Consume consensus stream
	var consensus strings.Builder
	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("consensus stream error: %v", chunk.Error)
		}
		consensus.WriteString(chunk.Delta)
	}

	result := consensus.String()
	if !strings.Contains(result, "CONSENSUS") {
		t.Errorf("expected consensus synthesis, got: %q", result)
	}
}

func TestPipelineSingleProviderFallback(t *testing.T) {
	primary := &mockPrimaryProvider{id: "primary"}

	// Only the primary provider
	providers := []provider.Provider{primary}

	pipeline := NewPipeline(providers, primary, 5*time.Second, 1, nil)

	messages := []provider.Message{
		{Role: provider.RoleUser, Content: "Hello"},
	}

	stream, _, err := pipeline.Run(context.Background(), messages, provider.QueryOpts{})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	var result strings.Builder
	for chunk := range stream {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		result.WriteString(chunk.Delta)
	}

	if !strings.Contains(result.String(), "Direct response") {
		t.Errorf("expected direct response fallback, got: %q", result.String())
	}
}
