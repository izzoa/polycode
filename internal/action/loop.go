package action

import (
	"context"
	"fmt"

	"github.com/izzoa/polycode/internal/provider"
)

const maxIterations = 10

// ToolLoop manages the iterative cycle of executing tool calls and sending
// results back to the model until the model produces a final text response.
type ToolLoop struct {
	executor *Executor
	primary  provider.Provider
}

// NewToolLoop creates a ToolLoop with the given executor and primary provider.
func NewToolLoop(executor *Executor, primary provider.Provider) *ToolLoop {
	return &ToolLoop{
		executor: executor,
		primary:  primary,
	}
}

// Run executes tool calls, appends results as native tool messages, and
// re-queries the model. It streams chunks to the output channel as they
// arrive from the provider. It repeats until the model stops issuing tool
// calls or the iteration limit is reached.
func (l *ToolLoop) Run(
	ctx context.Context,
	messages []provider.Message,
	toolCalls []provider.ToolCall,
	opts provider.QueryOpts,
	out chan<- provider.StreamChunk,
) error {
	// Work on a copy so we don't mutate the caller's slice.
	msgs := make([]provider.Message, len(messages))
	copy(msgs, messages)

	currentCalls := toolCalls

	for i := 0; i < maxIterations; i++ {
		if len(currentCalls) == 0 {
			out <- provider.StreamChunk{Done: true}
			return nil
		}

		// The assistant message with ToolCalls is already in msgs:
		// - For iteration 0: passed in by the caller (app.go)
		// - For iteration 1+: appended by the previous iteration's response handler

		// Execute every pending tool call and collect results.
		for _, call := range currentCalls {
			// Send execution status to output (not persisted)
			out <- provider.StreamChunk{
				Delta:  fmt.Sprintf("\nExecuting %s...\n", call.Name),
				Status: true,
			}

			result := l.executor.Execute(call)

			content := result.Output
			if result.Error != nil {
				if content != "" {
					content += "\n"
				}
				content += fmt.Sprintf("Error: %s", result.Error.Error())
			}

			// Show truncated tool output
			displayOutput := content
			if len(displayOutput) > 500 {
				displayOutput = displayOutput[:500] + "\n... (truncated)"
			}
			if displayOutput != "" {
				out <- provider.StreamChunk{
					Delta:  fmt.Sprintf("```\n%s\n```\n", displayOutput),
					Status: true,
				}
			}

			// Append native tool result message
			msgs = append(msgs, provider.Message{
				Role:       provider.RoleTool,
				Content:    content,
				ToolCallID: call.ID,
			})
		}

		// Send updated conversation back to the model.
		stream, err := l.primary.Query(ctx, msgs, opts)
		if err != nil {
			return fmt.Errorf("tool loop iteration %d: %w", i+1, err)
		}

		// Stream response chunks live to output, collecting for conversation state.
		var responseContent string
		var newToolCalls []provider.ToolCall
		for chunk := range stream {
			if chunk.Error != nil {
				return fmt.Errorf("tool loop iteration %d: %w", i+1, chunk.Error)
			}
			if chunk.Delta != "" {
				responseContent += chunk.Delta
				out <- provider.StreamChunk{Delta: chunk.Delta}
			}
			newToolCalls = append(newToolCalls, chunk.ToolCalls...)
		}

		// If the model made more tool calls, combine text + tool_calls
		// into a single assistant message (OpenAI requires this).
		if len(newToolCalls) > 0 {
			msgs = append(msgs, provider.Message{
				Role:      provider.RoleAssistant,
				Content:   responseContent,
				ToolCalls: newToolCalls,
			})
			currentCalls = newToolCalls
			continue
		}

		// No more tool calls — append text-only assistant response.
		if responseContent != "" {
			msgs = append(msgs, provider.Message{
				Role:    provider.RoleAssistant,
				Content: responseContent,
			})
		}

		// No more tool calls — done.
		out <- provider.StreamChunk{Done: true}
		return nil
	}

	return fmt.Errorf("tool loop exceeded maximum iterations (%d)", maxIterations)
}
