package consensus

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
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
	return FanOutWithTools(ctx, providers, messages, opts, timeout, tracker, onChunk, nil, nil, nil)
}

// FanOutWithTools is like FanOut but allows read-only tools during fan-out.
// readOnlyTools are the tool definitions sent to providers (e.g., file_read).
// toolExec executes tool calls; if nil, tools are stripped from the request.
// toolCapable lists provider IDs that support structured tool calling.
// Providers not in this set receive no tools (even if readOnlyTools is set).
// If toolCapable is nil, all providers get tools (backward compat).
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
	toolCapable map[string]bool,
) *FanOutResult {
	// Extend timeout when tool loops are enabled — each round needs
	// a full LLM call + tool execution, so the original single-round
	// timeout is insufficient.
	effectiveTimeout := timeout
	if toolExec != nil && len(readOnlyTools) > 0 {
		effectiveTimeout = timeout * time.Duration(maxFanOutToolRounds+1)
	}
	ctx, cancel := context.WithTimeout(ctx, effectiveTimeout)
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

			// Recover from panics in provider goroutines so one provider
			// can't crash the entire application.
			defer func() {
				if r := recover(); r != nil {
					stack := string(debug.Stack())
					err := fmt.Errorf("provider panicked: %v\n%s", r, stack)
					mu.Lock()
					result.Errors[id] = err
					mu.Unlock()
					if onChunk != nil {
						onChunk(id, provider.StreamChunk{Error: fmt.Errorf("provider panicked: %v", r)})
					}
					// Log the full stack trace for debugging.
					fmt.Fprintf(os.Stderr, "PANIC in provider %s: %v\n%s\n", id, r, stack)
				}
			}()

			start := time.Now()

			// Only pass tools to providers that support structured tool calling.
			provOpts := fanOutOpts
			provExec := toolExec
			if toolCapable != nil && !toolCapable[id] {
				provOpts.Tools = nil
				provExec = nil
			}

			resp, usage, err := queryWithToolLoop(ctx, p, messages, provOpts, provExec, onChunk, id)
			mu.Lock()
			if err != nil {
				result.Errors[id] = err
				// Preserve partial content even on error so it's not lost.
				if resp != "" {
					result.Responses[id] = resp
				}
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
			// Surface the error to the TUI so the tab shows failure.
			if onChunk != nil {
				onChunk(id, provider.StreamChunk{Error: err})
			}
			return totalBuf.String(), totalUsage, err
		}

		var buf strings.Builder
		var toolCalls []provider.ToolCall

		for chunk := range ch {
			if chunk.Error != nil {
				if onChunk != nil {
					onChunk(id, chunk)
				}
				// Return partial content + error — don't mask as success.
				totalBuf.WriteString(buf.String())
				return totalBuf.String(), totalUsage, chunk.Error
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

			// Emit status chunk so the TUI shows tool execution progress.
			if onChunk != nil {
				onChunk(id, provider.StreamChunk{
					Delta:  fmt.Sprintf("\nExecuting %s...\n", call.Name),
					Status: true,
				})
			}

			output, execErr := toolExec(call)
			content := output
			if execErr != nil {
				if content != "" {
					content += "\n"
				}
				content += "Error: " + execErr.Error()
			}

			// Show truncated tool output in the provider tab.
			if onChunk != nil && content != "" {
				display := content
				if len(display) > 500 {
					display = display[:500] + "\n... (truncated)"
				}
				onChunk(id, provider.StreamChunk{
					Delta:  fmt.Sprintf("```\n%s\n```\n", display),
					Status: true,
				})
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
