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

// Run executes tool calls, appends results as messages, and re-queries the
// model. It repeats until the model stops issuing tool calls or the iteration
// limit is reached. It returns the final streaming response channel.
func (l *ToolLoop) Run(
	ctx context.Context,
	messages []provider.Message,
	toolCalls []provider.ToolCall,
	opts provider.QueryOpts,
) (<-chan provider.StreamChunk, error) {
	// Work on a copy so we don't mutate the caller's slice.
	msgs := make([]provider.Message, len(messages))
	copy(msgs, messages)

	currentCalls := toolCalls

	for i := 0; i < maxIterations; i++ {
		if len(currentCalls) == 0 {
			// No more tool calls — return an empty, closed channel.
			ch := make(chan provider.StreamChunk, 1)
			ch <- provider.StreamChunk{Done: true}
			close(ch)
			return ch, nil
		}

		// Execute every pending tool call and collect results.
		for _, call := range currentCalls {
			result := l.executor.Execute(call)

			content := result.Output
			if result.Error != nil {
				if content != "" {
					content += "\n"
				}
				content += fmt.Sprintf("Error: %s", result.Error.Error())
			}

			msgs = append(msgs, provider.Message{
				Role:    provider.RoleUser,
				Content: fmt.Sprintf("[tool_result tool_call_id=%s]\n%s", result.ToolCallID, content),
			})
		}

		// Send updated conversation back to the model.
		stream, err := l.primary.Query(ctx, msgs, opts)
		if err != nil {
			return nil, fmt.Errorf("tool loop iteration %d: %w", i+1, err)
		}

		// Drain the stream to collect the full response and any new tool calls.
		var response provider.Response
		for chunk := range stream {
			if chunk.Error != nil {
				return nil, fmt.Errorf("tool loop iteration %d stream error: %w", i+1, chunk.Error)
			}
			response.Content += chunk.Delta
			response.ToolCalls = append(response.ToolCalls, chunk.ToolCalls...)
		}

		// Append the assistant's response to the conversation.
		if response.Content != "" {
			msgs = append(msgs, provider.Message{
				Role:    provider.RoleAssistant,
				Content: response.Content,
			})
		}

		// If the model made more tool calls, continue the loop.
		if len(response.ToolCalls) > 0 {
			currentCalls = response.ToolCalls
			continue
		}

		// No more tool calls — stream the final response back.
		// Since we already consumed the stream, replay it as a single chunk.
		ch := make(chan provider.StreamChunk, 2)
		if response.Content != "" {
			ch <- provider.StreamChunk{Delta: response.Content}
		}
		ch <- provider.StreamChunk{Done: true}
		close(ch)
		return ch, nil
	}

	return nil, fmt.Errorf("tool loop exceeded maximum iterations (%d)", maxIterations)
}
