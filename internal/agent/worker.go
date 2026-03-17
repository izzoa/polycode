package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/tokens"
)

// RoleType identifies a worker's role within an agent team.
type RoleType string

const (
	RolePlanner    RoleType = "planner"
	RoleResearcher RoleType = "researcher"
	RoleImplementer RoleType = "implementer"
	RoleTester     RoleType = "tester"
	RoleReviewer   RoleType = "reviewer"
)

// Worker wraps a provider with a role-specific system prompt.
// It constructs messages and queries the provider, returning the full
// response text and token usage.
type Worker struct {
	Role         RoleType
	ProviderName string // which provider this worker uses
	Provider     provider.Provider
	SystemPrompt string
	MaxTokens    int
}

// Run executes the worker: constructs messages [system, user input],
// queries the provider, drains the stream, and returns output + usage.
func (w *Worker) Run(ctx context.Context, input string) (string, tokens.Usage, error) {
	if w.Provider == nil {
		return "", tokens.Usage{}, fmt.Errorf("worker %s: provider is nil", w.Role)
	}

	messages := []provider.Message{
		{Role: provider.RoleSystem, Content: w.SystemPrompt},
		{Role: provider.RoleUser, Content: input},
	}

	opts := provider.QueryOpts{}
	if w.MaxTokens > 0 {
		opts.MaxTokens = w.MaxTokens
	}

	ch, err := w.Provider.Query(ctx, messages, opts)
	if err != nil {
		return "", tokens.Usage{}, fmt.Errorf("worker %s query: %w", w.Role, err)
	}

	var buf strings.Builder
	var usage tokens.Usage

	for chunk := range ch {
		if chunk.Error != nil {
			return buf.String(), usage, fmt.Errorf("worker %s stream: %w", w.Role, chunk.Error)
		}
		if chunk.Delta != "" {
			buf.WriteString(chunk.Delta)
		}
		if chunk.Done {
			usage.InputTokens = chunk.InputTokens
			usage.OutputTokens = chunk.OutputTokens
		}
	}

	return buf.String(), usage, nil
}
