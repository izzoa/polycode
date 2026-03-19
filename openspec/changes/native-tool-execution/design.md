## Context

The tool execution pipeline has three components: the consensus engine produces tool calls in `StreamChunk.ToolCalls`, the `ToolLoop` in `internal/action/loop.go` executes them and re-queries the primary model, and `cmd/polycode/app.go` wires it into the TUI.

Current `provider.Message` is `{Role, Content}` only. The OpenAI tool calling protocol requires: (1) an assistant message with `tool_calls` array, (2) followed by one `tool` role message per result with `tool_call_id`. Anthropic uses a similar but distinct format. Without these fields, the follow-up model call is malformed and hangs.

The tool loop currently buffers the entire follow-up stream before replaying a single chunk, and starts from `conv.snapshot()` (raw chat history) instead of the synthesis conversation that produced the tool calls.

## Goals / Non-Goals

**Goals:**
- Make tool execution work end-to-end with OpenAI-compatible providers
- Extend `provider.Message` to represent the full tool calling protocol
- Fix conversation threading so the tool loop continues the correct turn
- Stream tool output and follow-up responses live to the TUI
- Give tool execution its own timeout budget

**Non-Goals:**
- Supporting parallel tool execution (calls are sequential today, keep it that way)
- Adding new tool types (file_read, shell_exec, file_write already exist)
- Changing how tools are defined or registered
- Supporting Anthropic's native tool format (they use a different structure; the openai_compat adapter handles all providers for now)

## Decisions

### 1. Extend `provider.Message` with optional tool fields

**Decision**: Add `ToolCalls []ToolCall` and `ToolCallID string` to `Message`. Add `RoleTool Role = "tool"`.

```go
type Message struct {
    Role       Role       `json:"role"`
    Content    string     `json:"content"`
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"`  // assistant requesting tools
    ToolCallID string     `json:"tool_call_id,omitempty"` // tool result message
}
```

**Rationale**: Additive change — existing code that only uses `Role`/`Content` continues to work. The OpenAI API requires these exact fields for tool continuation. `ToolCalls` is set on assistant messages; `ToolCallID` is set on tool result messages.

### 2. Serialize tool messages in provider adapters

**Decision**: Update `openai.go` and `openai_compat.go` to include `tool_calls` in assistant messages and accept `tool` role messages in the request. The existing SSE parser already extracts `tool_calls` from responses.

**Rationale**: The parsers already handle tool calls in responses (that's why auto-approve works). The gap is only in the *request* serialization — tool result messages are not encoded correctly.

### 3. Pass synthesis conversation into the tool loop

**Decision**: In `app.go`, after the consensus stream ends with tool calls, build the tool loop's initial message list from: (1) the system prompt, (2) the user's original prompt, (3) the fan-out individual responses as context, (4) the consensus assistant message including its `ToolCalls`. This mirrors what the model saw when it requested the tools.

**Rationale**: The current approach passes `conv.snapshot()` which is the raw multi-turn chat history — the model never sees the consensus synthesis context that triggered the tool calls. The tool loop needs to continue from the synthesis turn.

### 4. Use native tool messages in the tool loop

**Decision**: Replace the fake `[tool_result tool_call_id=...]` user messages with proper `{Role: RoleTool, Content: output, ToolCallID: call.ID}` messages. Before executing tools, append the assistant message with `ToolCalls` to the conversation.

**Rationale**: This is the only format OpenAI-compatible endpoints accept for tool continuations. The current fake format causes the follow-up call to hang or error.

### 5. Stream tool loop responses live

**Decision**: Change `ToolLoop.Run` to return a channel immediately and relay chunks as they arrive from the provider, instead of buffering the entire response. For multi-iteration loops, interleave tool execution status messages with response chunks.

**Rationale**: The current buffer approach means the TUI shows nothing until the entire tool loop completes. Users need to see progress, especially with slow model responses.

### 6. Show tool output in the TUI

**Decision**: After each tool execution, send a `ConsensusChunkMsg` with the command and truncated output (first 500 chars + "..." if longer). This appears inline in the consensus stream.

**Rationale**: Users currently see "Auto-approved: Execute command: ls -la" but never the output. Showing the actual result makes the tool loop transparent.

### 7. Separate timeout for tool execution

**Decision**: Create a fresh context with `toolTimeout = 5 * time.Minute` for the tool loop instead of sharing the query context. Each individual shell command still has its own 120s executor timeout.

**Rationale**: The shared timeout means the tool loop often has only seconds left after fan-out + synthesis. Tool loops need their own budget since they may run multiple iterations.

## Risks / Trade-offs

- **Message struct size increase**: Adding optional fields to every Message. → Mitigated by `omitempty` JSON tags; only tool-related messages carry the extra fields.
- **Provider adapter changes**: Each adapter needs to handle tool messages in request serialization. → Scoped to the `openaiMsg` struct and marshaling in each adapter. Anthropic adapter would need its own format but is deferred (Non-Goal).
- **Streaming complexity**: Live-streaming from a multi-iteration tool loop is more complex than buffering. → Mitigated by keeping the channel-based pattern already used throughout the codebase.
