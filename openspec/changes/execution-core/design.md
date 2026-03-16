## Context

Polycode has a complete `internal/action/` package with `Executor`, `ToolLoop`, file ops, and shell exec — but the main app in `cmd/polycode/app.go` never calls it. The submit handler streams consensus output and saves it as text, but ignores any tool calls in the `StreamChunk.ToolCalls` field. The tool loop exists, it just isn't plugged in.

The consensus pipeline (`internal/consensus/pipeline.go`) collects all provider responses and synthesizes them, returning a `<-chan StreamChunk`. The final chunk may contain `ToolCalls`. The app currently ignores these.

## Goals / Non-Goals

**Goals:**
- Wire `action.ToolLoop` into the submit handler so consensus tool calls execute
- Implement a TUI-compatible confirmation flow for file writes and shell commands
- Persist tool call results in the conversation state and session file
- Add per-provider telemetry (latency, tokens, errors) to a local log
- Build end-to-end eval tests using mock providers
- Make all failure modes (timeout, provider error, tool error) produce clear, recoverable behavior

**Non-Goals:**
- Changing the consensus algorithm or prompt structure (that's Phase 2)
- Adding new tool types beyond file_read, file_write, shell_exec
- Autonomous multi-step agent loops without user confirmation (every destructive action still requires confirmation)
- Distributed or cloud-based telemetry (local file only)

## Decisions

### 1. Tool execution integrated via TUI messages

**Choice**: When the consensus stream's final chunk contains `ToolCalls`, the app sends a `ToolCallsPendingMsg` to the TUI. The TUI displays the proposed actions and waits for confirmation. On confirm, the app executes the tool calls via `action.Executor`, sends results back to the primary model via `action.ToolLoop`, and streams the follow-up response.

**Rationale**: This keeps tool execution in the Bubble Tea message loop, consistent with how everything else works. The user sees what's about to happen and confirms, matching the UX of Claude Code and Codex.

**Alternatives considered**:
- **Auto-execute without confirmation**: Dangerous for file writes and shell commands
- **Execute outside the TUI**: Would break the streaming display and require a separate output channel

### 2. Confirmation via TUI prompt, not stdin

**Choice**: The `ConfirmFunc` callback used by `action.Executor` sends a `ConfirmActionMsg` to the TUI program. The TUI renders a confirmation prompt (y/n) and blocks the tool execution goroutine via a channel until the user responds.

```go
confirm := func(description string) bool {
    responseCh := make(chan bool, 1)
    program.Send(ConfirmActionMsg{
        Description: description,
        ResponseCh:  responseCh,
    })
    return <-responseCh
}
```

**Rationale**: The executor runs in a goroutine; the TUI runs in the main loop. A channel bridges them cleanly.

### 3. Telemetry: append-only JSONL file

**Choice**: Create `internal/telemetry/telemetry.go` with a `Logger` that appends one JSON line per event to `~/.config/polycode/telemetry.jsonl`. Events: `query_start`, `provider_response`, `consensus_complete`, `tool_executed`, `error`.

Each event includes: timestamp, provider ID, event type, latency_ms, input_tokens, output_tokens, error (if any).

**Rationale**: JSONL is simple to write (append-only, no locking needed beyond file append), simple to read (`jq`), and trivial to implement. No external dependencies.

### 4. Session persistence includes tool results

**Choice**: Extend `SessionMessage` to include an optional `ToolCalls` field and `ToolResults` field. When tool calls execute, both the calls and their results are persisted as messages in the session. On resume, these are replayed into the conversation state.

**Rationale**: Without tool results in the session, a resumed conversation loses context about what files were read or modified, causing the model to repeat actions or hallucinate about file state.

### 5. Eval fixtures use mock providers with scripted responses

**Choice**: End-to-end tests create mock providers that return scripted responses (including tool calls). The tests verify the full pipeline: fan-out → consensus → tool execution → result feedback → final response → session save → session load.

**Rationale**: Tests must not hit real APIs. Mock providers with deterministic responses let us verify the state machine without network dependencies.

## Risks / Trade-offs

- **Confirmation UX blocks the pipeline**: If the user doesn't respond to a confirmation prompt, the tool loop goroutine blocks indefinitely. → **Mitigation**: Add a confirmation timeout (default 5 minutes) that cancels the tool call.

- **Tool execution errors crash the session**: A bad shell command or file write could leave the conversation in an inconsistent state. → **Mitigation**: Tool errors are captured as messages, not panics. The conversation continues with the error in context.

- **Telemetry file grows unbounded**: → **Mitigation**: Rotate or truncate on startup if file exceeds 10MB. Document that telemetry is opt-in future work.
