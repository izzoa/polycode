package consensus

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/tokens"
)

// Pipeline orchestrates the full consensus workflow: fan-out query to all
// providers, threshold check, and synthesis via the primary provider.
type Pipeline struct {
	engine         *Engine
	providers      []provider.Provider
	timeout        time.Duration
	tracker        *tokens.TokenTracker
	truncateBudget int           // max total chars for fan-out responses; 0 = no truncation
	onChunk        ChunkCallback // optional: called for each streaming chunk during fan-out

	// Read-only tool support for fan-out. When both are set, providers can
	// call read-only tools (e.g., file_read) during their fan-out response.
	fanOutTools    []provider.ToolDefinition
	fanOutToolExec FanOutToolExecutor
	toolCapable    map[string]bool // provider IDs known to support tool calling
}

// NewPipeline creates a Pipeline.
//   - providers is the full set of providers to fan-out to.
//   - primary is the provider used for the synthesis step.
//   - timeout is the per-phase timeout (fan-out and synthesis each get this).
//   - minResponses is the minimum number of successful fan-out responses
//     required before synthesis proceeds.
//   - tracker is optional (may be nil) for token usage tracking and limit enforcement.
//   - mode controls synthesis depth (quick/balanced/thorough).
func NewPipeline(
	providers []provider.Provider,
	primary provider.Provider,
	timeout time.Duration,
	minResponses int,
	tracker *tokens.TokenTracker,
	mode SynthesisMode,
) *Pipeline {
	// Derive truncation budget from the primary model's context limit.
	// Reserve ~25% for the synthesis prompt overhead and output tokens.
	var truncateBudget int
	if tracker != nil {
		pu := tracker.Get(primary.ID())
		if pu.Limit > 0 {
			// Rough chars-per-token estimate: ~4 chars/token.
			truncateBudget = (pu.Limit * 3 / 4) * 4
		}
	}

	engine := NewEngine(primary, timeout, minResponses)
	engine.mode = mode

	return &Pipeline{
		engine:         engine,
		providers:      providers,
		timeout:        timeout,
		tracker:        tracker,
		truncateBudget: truncateBudget,
	}
}

// SetChunkCallback sets a callback that fires for each streaming chunk from
// individual providers during fan-out. This enables real-time display of
// individual provider output while the fan-out is in progress.
func (p *Pipeline) SetChunkCallback(cb ChunkCallback) {
	p.onChunk = cb
}

// SetFanOutTools configures read-only tools available to providers during
// fan-out. The executor handles the actual tool execution (e.g., file_read).
// toolCapable maps provider IDs that support structured tool calling — others
// won't receive tools. Pass nil to send tools to all providers.
func (p *Pipeline) SetFanOutTools(tools []provider.ToolDefinition, exec FanOutToolExecutor, toolCapable map[string]bool) {
	p.fanOutTools = tools
	p.fanOutToolExec = exec
	p.toolCapable = toolCapable
}

// Run executes the full consensus pipeline:
//  1. Fan-out the query to every provider.
//  2. Check the minimum-response threshold.
//  3. If only the primary provider responded, return its response directly
//     without synthesis.
//  4. Otherwise, synthesize the collected responses through the primary.
//
// It returns the streaming consensus channel, the raw fan-out results (so
// the TUI can display individual responses), and any error.
func (p *Pipeline) Run(
	ctx context.Context,
	messages []provider.Message,
	opts provider.QueryOpts,
) (<-chan provider.StreamChunk, *FanOutResult, error) {
	// Check if primary would exceed its limit before even starting.
	primaryID := p.engine.primary.ID()
	if p.tracker != nil && p.tracker.WouldExceedLimit(primaryID) {
		return nil, nil, fmt.Errorf(
			"consensus: primary provider %q has exceeded its context limit — start a new session or increase max_context in config",
			primaryID,
		)
	}

	// Phase 1: fan-out (with read-only tools if configured).
	fanOutResult := FanOutWithTools(ctx, p.providers, messages, opts, p.timeout, p.tracker, p.onChunk, p.fanOutTools, p.fanOutToolExec, p.toolCapable)

	// Truncate fan-out responses to fit within the primary model's context.
	if p.truncateBudget > 0 && len(fanOutResult.Responses) > 0 {
		fanOutResult.Responses = TruncateResponses(fanOutResult.Responses, p.truncateBudget)
	}

	// Check minimum response threshold.
	if len(fanOutResult.Responses) < p.engine.minResponses {
		return nil, fanOutResult, fmt.Errorf(
			"consensus: received %d responses, need at least %d",
			len(fanOutResult.Responses),
			p.engine.minResponses,
		)
	}

	// If only the primary provider responded, return its response directly
	// as a streaming channel -- no synthesis needed.
	if len(fanOutResult.Responses) == 1 {
		if resp, ok := fanOutResult.Responses[primaryID]; ok {
			ch := make(chan provider.StreamChunk, 1)
			go func() {
				defer close(ch)
				ch <- provider.StreamChunk{Delta: resp, Done: true}
			}()
			return ch, fanOutResult, nil
		}
	}

	// Extract the original prompt from the last user message.
	originalPrompt := extractOriginalPrompt(messages)

	// Phase 2: synthesis.
	ch, err := p.engine.Synthesize(ctx, originalPrompt, fanOutResult.Responses, opts)
	if err != nil {
		return nil, fanOutResult, fmt.Errorf("consensus: synthesis failed: %w", err)
	}

	return ch, fanOutResult, nil
}

// extractOriginalPrompt pulls the user's question out of the message history.
// It returns the content of the last user-role message, or a fallback.
func extractOriginalPrompt(messages []provider.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == provider.RoleUser {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return "(no user prompt found)"
}
