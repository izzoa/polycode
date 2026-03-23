package consensus

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/tokens"
)

// FanOutResult holds the collected responses and errors from a fan-out query
// to multiple providers.
type FanOutResult struct {
	// Responses maps provider ID to the full assembled response text.
	Responses map[string]string
	// Errors maps provider ID to any error encountered during the query.
	Errors map[string]error
	// Usage maps provider ID to the token usage reported by that provider.
	Usage map[string]tokens.Usage
	// Latencies maps provider ID to the response wall-clock duration.
	Latencies map[string]time.Duration
	// Skipped lists provider IDs that were skipped due to context limits.
	Skipped []string
}

// ChunkCallback is called for each streaming chunk from a provider during fan-out.
// It receives the provider ID and the chunk. Called from provider goroutines —
// implementations must be safe for concurrent use.
type ChunkCallback func(providerID string, chunk provider.StreamChunk)

// FanOutToolExecutor executes a single tool call during fan-out.
// Only read-only tools should be wired here. It is called from concurrent
// provider goroutines, so the implementation must be safe for concurrent use.
type FanOutToolExecutor func(call provider.ToolCall) (output string, err error)

// maxFanOutToolRounds caps the number of tool-call → re-query cycles per
// provider during fan-out to prevent runaway loops.
const maxFanOutToolRounds = 3

// FanOut dispatches a query to all providers concurrently, collects their
// streaming responses into complete strings, and returns once every provider
// has finished or the timeout is reached.
//
// If onChunk is non-nil, it is called for every streaming chunk as it arrives
// from each provider, enabling real-time display of individual provider output.
//
// If a tracker is provided, providers that would exceed their context limit
// are skipped (recorded in result.Skipped).
func FanOut(
	ctx context.Context,
	providers []provider.Provider,
	messages []provider.Message,
	opts provider.QueryOpts,
	timeout time.Duration,
	tracker *tokens.TokenTracker,
	onChunk ChunkCallback,
) *FanOutResult {
	return FanOutWithTools(ctx, providers, messages, opts, timeout, tracker, onChunk, nil, nil)
}

// FanOutWithTools is like FanOut but allows read-only tools during fan-out.
// readOnlyTools are the tool definitions sent to providers (e.g., file_read).
// toolExec executes tool calls; if nil, tools are stripped from the request.
func FanOutWithTools(
	ctx context.Context,
	providers []provider.Provider,
	messages []provider.Message,
	opts provider.QueryOpts,
	timeout time.Duration,
	tracker *tokens.TokenTracker,
	onChunk ChunkCallback,
	readOnlyTools []provider.ToolDefinition,
	toolExec FanOutToolExecutor,
) *FanOutResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := &FanOutResult{
		Responses: make(map[string]string, len(providers)),
		Errors:    make(map[string]error, len(providers)),
		Usage:     make(map[string]tokens.Usage, len(providers)),
		Latencies: make(map[string]time.Duration, len(providers)),
	}

	// Build fan-out opts: use read-only tools if provided, otherwise strip all.
	fanOutOpts := opts
	if toolExec != nil && len(readOnlyTools) > 0 {
		fanOutOpts.Tools = readOnlyTools
	} else {
		fanOutOpts.Tools = nil
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, p := range providers {
		// Pre-dispatch context limit check.
		if tracker != nil && tracker.WouldExceedLimit(p.ID()) {
			result.Skipped = append(result.Skipped, p.ID())
			continue
		}

		wg.Add(1)
		go func(p provider.Provider) {
			defer wg.Done()

			id := p.ID()
			start := time.Now()
			resp, usage, err := queryWithToolLoop(ctx, p, messages, fanOutOpts, toolExec, onChunk, id)
			mu.Lock()
			if err != nil {
				result.Errors[id] = err
			} else {
				result.Responses[id] = resp
				result.Usage[id] = usage
			}
			result.Latencies[id] = time.Since(start)
			mu.Unlock()
		}(p)
	}

	wg.Wait()
	return result
}

// queryWithToolLoop queries a single provider and, if it returns tool calls
// that can be executed by toolExec, executes them and re-queries up to
// maxFanOutToolRounds times. Returns the accumulated text response.
func queryWithToolLoop(
	ctx context.Context,
	p provider.Provider,
	messages []provider.Message,
	opts provider.QueryOpts,
	toolExec FanOutToolExecutor,
	onChunk ChunkCallback,
	id string,
) (string, tokens.Usage, error) {
	msgs := make([]provider.Message, len(messages))
	copy(msgs, messages)

	var totalBuf strings.Builder
	var totalUsage tokens.Usage

	for round := 0; round <= maxFanOutToolRounds; round++ {
		ch, err := p.Query(ctx, msgs, opts)
		if err != nil {
			return "", totalUsage, err
		}

		var buf strings.Builder
		var toolCalls []provider.ToolCall

		for chunk := range ch {
			if chunk.Error != nil {
				if onChunk != nil {
					onChunk(id, chunk)
				}
				return "", totalUsage, chunk.Error
			}
			if chunk.Delta != "" {
				buf.WriteString(chunk.Delta)
				if onChunk != nil {
					onChunk(id, chunk)
				}
			}
			if chunk.Done {
				// Accumulate usage across all rounds.
				totalUsage.InputTokens += chunk.InputTokens
				totalUsage.OutputTokens += chunk.OutputTokens
				toolCalls = append(toolCalls, chunk.ToolCalls...)
			}
		}

		responseText := buf.String()
		totalBuf.WriteString(responseText)

		// No tool calls, no executor, or last round — we're done.
		if len(toolCalls) == 0 || toolExec == nil || round == maxFanOutToolRounds {
			if onChunk != nil {
				onChunk(id, provider.StreamChunk{Done: true})
			}
			return totalBuf.String(), totalUsage, nil
		}

		// Check context before executing tools.
		if ctx.Err() != nil {
			if onChunk != nil {
				onChunk(id, provider.StreamChunk{Done: true})
			}
			return totalBuf.String(), totalUsage, nil
		}

		// Execute tool calls and build follow-up messages.
		msgs = append(msgs, provider.Message{
			Role:      provider.RoleAssistant,
			Content:   responseText,
			ToolCalls: toolCalls,
		})

		for _, call := range toolCalls {
			if ctx.Err() != nil {
				break // respect fan-out timeout
			}
			output, execErr := toolExec(call)
			content := output
			if execErr != nil {
				if content != "" {
					content += "\n"
				}
				content += "Error: " + execErr.Error()
			}
			msgs = append(msgs, provider.Message{
				Role:       provider.RoleTool,
				Content:    content,
				ToolCallID: call.ID,
			})
		}
		// Loop to re-query with tool results.
	}

	// Should not reach here (loop exits via return), but just in case.
	if onChunk != nil {
		onChunk(id, provider.StreamChunk{Done: true})
	}
	return totalBuf.String(), totalUsage, nil
}
