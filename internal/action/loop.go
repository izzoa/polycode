package action

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/izzoa/polycode/internal/provider"
)

// ToolDoneCallback is called when a tool call finishes executing.
type ToolDoneCallback func(toolName string, duration time.Duration, err string)

// ToolLoop manages the iterative cycle of executing tool calls and sending
// results back to the model until the model produces a final text response.
type ToolLoop struct {
	executor   *Executor
	primary    provider.Provider
	onToolDone ToolDoneCallback
}

// NewToolLoop creates a ToolLoop with the given executor and primary provider.
func NewToolLoop(executor *Executor, primary provider.Provider) *ToolLoop {
	return &ToolLoop{
		executor: executor,
		primary:  primary,
	}
}

// SetToolDoneCallback sets a callback that fires when each tool call completes.
func (l *ToolLoop) SetToolDoneCallback(cb ToolDoneCallback) {
	l.onToolDone = cb
}

// Run executes tool calls, appends results as native tool messages, and
// re-queries the model. It streams chunks to the output channel as they
// arrive from the provider. It repeats until the model stops issuing tool
// calls or the context is cancelled.
func (l *ToolLoop) Run(
	ctx context.Context,
	messages []provider.Message,
	toolCalls []provider.ToolCall,
	opts provider.QueryOpts,
	out chan<- provider.StreamChunk,
) error {
	_, err := l.RunWithMessages(ctx, messages, toolCalls, opts, out)
	return err
}

// RunWithMessages is like Run but also returns the messages appended during
// the tool loop (tool results + follow-up assistant messages). These can be
// used to preserve structured tool call history in the conversation state.
func (l *ToolLoop) RunWithMessages(
	ctx context.Context,
	messages []provider.Message,
	toolCalls []provider.ToolCall,
	opts provider.QueryOpts,
	out chan<- provider.StreamChunk,
) ([]provider.Message, error) {
	// Work on a copy so we don't mutate the caller's slice.
	msgs := make([]provider.Message, len(messages))
	copy(msgs, messages)
	initialLen := len(msgs)

	currentCalls := toolCalls

	appended := func() []provider.Message {
		return msgs[initialLen:]
	}

	for {
		if len(currentCalls) == 0 {
			select {
			case out <- provider.StreamChunk{Done: true}:
			case <-ctx.Done():
			}
			return appended(), nil
		}

		// The assistant message with ToolCalls is already in msgs:
		// - For iteration 0: passed in by the caller (app.go)
		// - For iteration 1+: appended by the previous iteration's response handler

		// Execute every pending tool call and collect results.
		for _, call := range currentCalls {
			// Send execution status to output (not persisted)
			select {
			case out <- provider.StreamChunk{
				Delta:  fmt.Sprintf("\nExecuting %s...\n", call.Name),
				Status: true,
			}:
			case <-ctx.Done():
				return appended(), ctx.Err()
			}

			callStart := time.Now()
			result := l.executor.Execute(call)
			callDuration := time.Since(callStart)

			// Notify callback of completion
			if l.onToolDone != nil {
				errStr := ""
				if result.Error != nil {
					errStr = result.Error.Error()
				}
				l.onToolDone(call.Name, callDuration, errStr)
			}

			content := result.Output
			if result.Error != nil {
				if content != "" {
					content += "\n"
				}
				content += fmt.Sprintf("Error: %s", result.Error.Error())
			}

			// Show truncated tool output (max 10 lines)
			displayOutput := strings.TrimRight(content, "\n")
			if lines := strings.Split(displayOutput, "\n"); len(lines) > 10 {
				displayOutput = strings.Join(lines[:10], "\n") + fmt.Sprintf("\n[+%d more lines]", len(lines)-10)
			} else if len(displayOutput) > 500 {
				displayOutput = displayOutput[:500] + "\n... (truncated)"
			}
			if displayOutput != "" {
				select {
				case out <- provider.StreamChunk{
					Delta:  fmt.Sprintf("```\n%s\n```\n", displayOutput),
					Status: true,
				}:
				case <-ctx.Done():
					return appended(), ctx.Err()
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
			return appended(), fmt.Errorf("tool loop: %w", err)
		}

		// Stream response chunks live to output, collecting for conversation state.
		var responseContent string
		var newToolCalls []provider.ToolCall
		for chunk := range stream {
			if chunk.Error != nil {
				return appended(), fmt.Errorf("tool loop: %w", chunk.Error)
			}
			if chunk.Delta != "" {
				responseContent += chunk.Delta
				select {
				case out <- provider.StreamChunk{Delta: chunk.Delta}:
				case <-ctx.Done():
					return appended(), ctx.Err()
				}
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
		select {
		case out <- provider.StreamChunk{Done: true}:
		case <-ctx.Done():
		}
		return appended(), nil
	}
}
