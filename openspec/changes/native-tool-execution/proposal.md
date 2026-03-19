## Why

Tool execution in polycode is broken for OpenAI-compatible providers. When the consensus model requests tool calls (file_read, shell_exec, etc.), the commands execute successfully but the follow-up model call hangs or produces empty responses, causing the TUI to stall at "Synthesizing" until timeout. Three protocol-level bugs cause this:

1. **`provider.Message` lacks tool call support** — only has `role` + `content`. Tool results are sent back as fake `user` messages (`[tool_result tool_call_id=...]`) instead of native OpenAI `tool` role messages. Strict endpoints reject this.
2. **Wrong conversation context** — tool calls originate from the consensus synthesis prompt, but the tool loop receives `conv.snapshot()` (original chat history). The model doesn't see the turn that requested the tools.
3. **Buffered follow-up** — `ToolLoop.Run` fully drains the provider's response internally before replaying to the TUI. If the provider call hangs, the user sees nothing until timeout. Tool stdout/stderr is also never shown.

## What Changes

- **Extend `provider.Message`** to carry `ToolCalls` (assistant requesting tools) and `ToolCallID` (tool result messages with `role: "tool"`)
- **Add `RoleTool` role** to the Role type for tool result messages
- **Update all provider adapters** (OpenAI, Anthropic, Gemini, OpenAI-compat) to serialize/deserialize tool call messages in their native API formats
- **Fix conversation threading** — pass the actual synthesis conversation (including the assistant message with tool calls) into the tool loop, not the raw chat history
- **Stream tool loop live** — relay follow-up response chunks as they arrive instead of buffering the entire stream
- **Show tool output in TUI** — display command stdout/stderr in the consensus stream so users see what the tools produced
- **Separate timeout for tool loop** — give tool execution its own timeout budget instead of sharing the query context

## Capabilities

### New Capabilities
- `native-tool-messages`: Extend the provider message protocol with native tool call and tool result message types, enabling correct multi-turn tool use with all provider APIs
- `live-tool-streaming`: Stream tool execution output and model follow-up responses to the TUI in real-time instead of buffering

### Modified Capabilities
<!-- None — existing specs don't cover tool execution protocol -->

## Impact

- **Code**: `internal/provider/provider.go` — Message struct extension, new Role
- **Code**: `internal/provider/openai.go`, `openai_compat.go`, `anthropic.go`, `gemini.go` — serialize tool messages per API format
- **Code**: `internal/action/loop.go` — use native tool messages, stream live
- **Code**: `cmd/polycode/app.go` — fix conversation threading, show tool output, separate timeout
- **Code**: `internal/consensus/fanout.go` — preserve tool call metadata in fan-out results
- **Tests**: Update `internal/action/eval_test.go` and add provider-level tool message tests
- **No breaking changes**: The Message struct extension is additive (new optional fields)
