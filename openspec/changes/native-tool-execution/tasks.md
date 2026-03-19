## 1. Extend provider.Message

- [ ] 1.1 Add `RoleTool Role = "tool"` constant to `internal/provider/provider.go`
- [ ] 1.2 Add `ToolCalls []ToolCall` and `ToolCallID string` fields to `provider.Message` with `json:",omitempty"` tags
- [ ] 1.3 Verify `go build ./...` compiles (additive change, no breakage expected)

## 2. Update provider adapters — request serialization

- [ ] 2.1 In `internal/provider/openai.go`: update `openaiMsg` struct to include `tool_calls` and `tool_call_id` fields; update the message-to-openaiMsg conversion to populate them from `Message.ToolCalls` and `Message.ToolCallID`; handle `RoleTool` role
- [ ] 2.2 In `internal/provider/openai_compat.go`: same changes as 2.1 (reuses the same `openaiMsg` type or has its own — verify which)
- [ ] 2.3 In `internal/provider/anthropic.go`: map `RoleTool` messages to Anthropic's `tool_result` content blocks format (role stays `"user"` with tool_result content type); include `tool_use` blocks on assistant messages
- [ ] 2.4 In `internal/provider/gemini.go`: map tool messages to Gemini's function calling format (`functionCall` / `functionResponse` parts)
- [ ] 2.5 Verify `go build ./...` compiles after all adapter changes

## 3. Fix conversation threading in app.go

- [ ] 3.1 After the consensus stream ends with tool calls, build a synthesis-context message list for the tool loop: system prompt + user prompt + summarized individual responses + the assistant message (with `ToolCalls` populated from `pendingToolCalls`)
- [ ] 3.2 Pass this synthesis-context list to `toolLoop.Run()` instead of `conv.snapshot()`
- [ ] 3.3 Create a separate timeout context for the tool loop: `toolCtx, toolCancel := context.WithTimeout(context.Background(), 5*time.Minute)`

## 4. Rewrite ToolLoop to use native messages and stream live

- [ ] 4.1 Before executing tools, append an assistant `Message{Role: RoleAssistant, ToolCalls: currentCalls}` to the conversation
- [ ] 4.2 Replace the fake `[tool_result ...]` user messages with `Message{Role: RoleTool, Content: output, ToolCallID: call.ID}`
- [ ] 4.3 Change `ToolLoop.Run` to accept a `chan<- provider.StreamChunk` output channel parameter (or return a channel and write to it from a goroutine) — relay chunks as they arrive from the provider instead of buffering
- [ ] 4.4 For multi-iteration loops, send a status chunk between iterations (e.g., `"Executing tool: shell_exec..."`) before the next execution

## 5. Show tool output in TUI

- [ ] 5.1 In `app.go`, after each tool execution (inside the tool loop or via a callback), send a `ConsensusChunkMsg` with the command name and truncated output (first 500 chars)
- [ ] 5.2 Format tool output as a fenced code block in the consensus stream for readability

## 6. Tests

- [ ] 6.1 Add unit test: `Message` with `ToolCalls` serializes correctly to OpenAI JSON format
- [ ] 6.2 Add unit test: `Message` with `RoleTool` and `ToolCallID` serializes correctly
- [ ] 6.3 Update `internal/action/eval_test.go` to use native tool messages instead of fake user messages
- [ ] 6.4 Add integration test: mock OpenAI-compatible server that requires correct tool continuation format — verify the tool loop completes a round trip
- [ ] 6.5 Verify `go build ./...` compiles cleanly
- [ ] 6.6 Verify `go test ./... -count=1 -race` passes

## 7. Manual verification

- [ ] 7.1 Test tool execution in yolo mode: "analyse this repo" should execute shell commands AND show their output, then produce a follow-up analysis
- [ ] 7.2 Test tool execution in normal mode: confirm prompts should appear, and after approval the tool output and follow-up should stream live
- [ ] 7.3 Test with multiple providers — verify fan-out individual responses are not empty when models respond with tool calls
