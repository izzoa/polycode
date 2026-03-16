package consensus

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/izzoa/polycode/internal/provider"
)

// mockProvider implements provider.Provider for testing.
type mockProvider struct {
	id       string
	response string
	delay    time.Duration
	err      error
}

func (m *mockProvider) ID() string { return m.id }
func (m *mockProvider) Authenticate() error { return nil }
func (m *mockProvider) Validate() error { return nil }

func (m *mockProvider) Query(ctx context.Context, messages []provider.Message, opts provider.QueryOpts) (<-chan provider.StreamChunk, error) {
	if m.err != nil {
		return nil, m.err
	}

	ch := make(chan provider.StreamChunk, 2)
	go func() {
		defer close(ch)
		if m.delay > 0 {
			select {
			case <-time.After(m.delay):
			case <-ctx.Done():
				ch <- provider.StreamChunk{Error: ctx.Err()}
				return
			}
		}
		ch <- provider.StreamChunk{Delta: m.response}
		ch <- provider.StreamChunk{Done: true}
	}()
	return ch, nil
}

func TestFanOutAllSucceed(t *testing.T) {
	providers := []provider.Provider{
		&mockProvider{id: "a", response: "hello from a"},
		&mockProvider{id: "b", response: "hello from b"},
	}

	result := FanOut(context.Background(), providers, nil, provider.QueryOpts{}, 5*time.Second, nil)

	if len(result.Responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(result.Responses))
	}
	if result.Responses["a"] != "hello from a" {
		t.Errorf("unexpected response from a: %q", result.Responses["a"])
	}
	if result.Responses["b"] != "hello from b" {
		t.Errorf("unexpected response from b: %q", result.Responses["b"])
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %d", len(result.Errors))
	}
}

func TestFanOutOneError(t *testing.T) {
	providers := []provider.Provider{
		&mockProvider{id: "a", response: "hello"},
		&mockProvider{id: "b", err: fmt.Errorf("rate limited")},
	}

	result := FanOut(context.Background(), providers, nil, provider.QueryOpts{}, 5*time.Second, nil)

	if len(result.Responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(result.Responses))
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors["b"] == nil {
		t.Error("expected error for provider b")
	}
}

func TestFanOutTimeout(t *testing.T) {
	providers := []provider.Provider{
		&mockProvider{id: "fast", response: "quick"},
		&mockProvider{id: "slow", response: "slow", delay: 5 * time.Second},
	}

	result := FanOut(context.Background(), providers, nil, provider.QueryOpts{}, 200*time.Millisecond, nil)

	if result.Responses["fast"] != "quick" {
		t.Errorf("fast provider should succeed, got: %q", result.Responses["fast"])
	}

	// The slow provider should have either errored or not responded
	if _, ok := result.Responses["slow"]; ok {
		t.Error("slow provider should not have completed within timeout")
	}
}
