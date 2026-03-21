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

// FanOut dispatches a query to all providers concurrently, collects their
// streaming responses into complete strings, and returns once every provider
// has finished or the timeout is reached.
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
) *FanOutResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := &FanOutResult{
		Responses: make(map[string]string, len(providers)),
		Errors:    make(map[string]error, len(providers)),
		Usage:     make(map[string]tokens.Usage, len(providers)),
		Latencies: make(map[string]time.Duration, len(providers)),
	}

	// Strip tools from fan-out opts — individual providers should respond
	// with text analysis, not tool calls. Only the consensus synthesizer
	// (which runs after fan-out) should execute tools.
	fanOutOpts := opts
	fanOutOpts.Tools = nil

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
			ch, err := p.Query(ctx, messages, fanOutOpts)
			if err != nil {
				mu.Lock()
				result.Errors[id] = err
				result.Latencies[id] = time.Since(start)
				mu.Unlock()
				return
			}

			var buf strings.Builder
			var usage tokens.Usage
			for chunk := range ch {
				if chunk.Error != nil {
					mu.Lock()
					result.Errors[id] = chunk.Error
					result.Latencies[id] = time.Since(start)
					mu.Unlock()
					return
				}
				buf.WriteString(chunk.Delta)
				if chunk.Done {
					usage = tokens.Usage{
						InputTokens:  chunk.InputTokens,
						OutputTokens: chunk.OutputTokens,
					}
				}
			}

			mu.Lock()
			result.Responses[id] = buf.String()
			result.Usage[id] = usage
			result.Latencies[id] = time.Since(start)
			mu.Unlock()
		}(p)
	}

	wg.Wait()
	return result
}
