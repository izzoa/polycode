package consensus

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/izzoa/polycode/internal/provider"
)

// Engine drives the consensus synthesis step. It takes the collected responses
// from a fan-out query and asks a primary provider to synthesize them into a
// single authoritative answer.
type Engine struct {
	primary      provider.Provider
	timeout      time.Duration
	minResponses int
}

// NewEngine creates a consensus Engine.
//   - primary is the provider used to synthesize the final answer.
//   - timeout is the maximum time allowed for the synthesis query.
//   - minResponses is the minimum number of successful responses required
//     before synthesis can proceed.
func NewEngine(primary provider.Provider, timeout time.Duration, minResponses int) *Engine {
	return &Engine{
		primary:      primary,
		timeout:      timeout,
		minResponses: minResponses,
	}
}

// BuildConsensusPrompt constructs the message slice sent to the primary
// provider for synthesis.
func (e *Engine) BuildConsensusPrompt(originalPrompt string, responses map[string]string) []provider.Message {
	var b strings.Builder

	b.WriteString("You are synthesizing responses from multiple AI models to produce the best possible answer.\n\n")
	b.WriteString(fmt.Sprintf("Original question: %s\n\n", originalPrompt))
	b.WriteString("Model responses:\n")

	// Sort provider IDs for deterministic prompt ordering.
	ids := make([]string, 0, len(responses))
	for id := range responses {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		b.WriteString("---\n")
		b.WriteString(fmt.Sprintf("[Model: %s]\n", id))
		b.WriteString(responses[id])
		b.WriteString("\n---\n\n")
	}

	b.WriteString("Analyze all responses. Identify areas of agreement, unique insights, and any errors.\n")
	b.WriteString("Produce a single, authoritative response that represents the best synthesis.\n")
	b.WriteString("If models disagree, use your judgment to determine the correct approach.\n")

	return []provider.Message{
		{
			Role:    provider.RoleUser,
			Content: b.String(),
		},
	}
}

// Synthesize sends the consensus prompt to the primary provider and returns
// the streaming response channel. The caller is responsible for consuming the
// channel.
func (e *Engine) Synthesize(
	ctx context.Context,
	originalPrompt string,
	responses map[string]string,
	opts provider.QueryOpts,
) (<-chan provider.StreamChunk, error) {
	ctx, cancel := context.WithTimeout(ctx, e.timeout)

	messages := e.BuildConsensusPrompt(originalPrompt, responses)

	ch, err := e.primary.Query(ctx, messages, opts)
	if err != nil {
		cancel()
		return nil, err
	}

	// Wrap the channel so we can release the timeout context when the stream
	// is fully consumed.
	out := make(chan provider.StreamChunk)
	go func() {
		defer cancel()
		defer close(out)
		for chunk := range ch {
			out <- chunk
		}
	}()

	return out, nil
}
