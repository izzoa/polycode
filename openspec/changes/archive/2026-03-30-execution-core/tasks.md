## 1. TUI Confirmation Flow

- [x] 1.1 Update `ConfirmActionMsg` in `internal/tui/update.go` to include a `ResponseCh chan bool` field for synchronous confirmation
- [x] 1.2 Add a `confirmPending` state to the TUI model that renders a confirmation prompt (description + y/n) and blocks other input
- [x] 1.3 Handle `y` and `n` keypresses during confirm state: send response on the channel and dismiss the prompt
- [x] 1.4 Add a 5-minute timeout on the confirmation channel — auto-cancel if user doesn't respond
- [x] 1.5 Render the confirmation prompt with the action description, file diff (for file_write), or command text (for shell_exec)

## 2. Wire Tool Loop into App Runtime

- [x] 2.1 Create a `confirmFunc` in `app.go` that sends `ConfirmActionMsg` to the TUI program and waits on the response channel
- [x] 2.2 Create an `action.Executor` in `startTUI` with the confirm callback and a 120-second command timeout
- [x] 2.3 Create an `action.ToolLoop` with the executor and the primary provider
- [x] 2.4 After streaming the consensus response, check the final `StreamChunk` for `ToolCalls` — if present, run `ToolLoop.Run()` with the current conversation messages and tool calls
- [x] 2.5 Stream the tool loop's follow-up response to the TUI via `ConsensusChunkMsg` (reuse the existing consensus panel)
- [x] 2.6 If the tool loop produces additional tool calls (multi-iteration), continue the loop up to `maxIterations`
- [x] 2.7 After tool execution completes, append all tool call messages and results to the conversation state

## 3. Working State in Synthesis

- [x] 3.1 Update `conversationState.snapshot()` to include tool result messages (they're already in the messages slice — verify this works end-to-end)
- [x] 3.2 Verify that the fan-out query sends the full conversation history (including prior tool results) to all providers
- [x] 3.3 Add a `ToolCallMsg` TUI message type to display which tool is being executed ("Reading file.go...", "Running `go test`...")

## 4. Session Persistence for Tool Results

- [x] 4.1 Extend `config.SessionMessage` with optional `ToolCalls []ToolCallRecord` and `ToolResult *ToolResultRecord` fields
- [x] 4.2 Define `ToolCallRecord` (ID, Name, Arguments) and `ToolResultRecord` (ToolCallID, Output, Error) in `config/session.go`
- [x] 4.3 When saving the session, serialize tool call messages and tool result messages with their structured fields
- [x] 4.4 When loading a session, reconstruct tool call/result messages in the conversation state
- [x] 4.5 Add a test: save session with tool calls → load session → verify conversation state includes tool results

## 5. Provider Telemetry

- [x] 5.1 Create `internal/telemetry/telemetry.go` with `Event` struct (Timestamp, ProviderID, EventType, LatencyMS, InputTokens, OutputTokens, ToolName, Success, Error)
- [x] 5.2 Implement `Logger` struct with `Log(event Event)` that appends JSONL to `~/.config/polycode/telemetry.jsonl`
- [x] 5.3 Implement `NewLogger()` that opens the file in append mode, checks size, truncates if >10MB
- [x] 5.4 Create the logger in `startTUI` and pass it to the submit handler
- [x] 5.5 Log `provider_response` events after fan-out (one per provider, with latency and token counts)
- [x] 5.6 Log `consensus_complete` event after synthesis (with primary provider latency and tokens)
- [x] 5.7 Log `tool_executed` events after each tool call (with tool name, duration, success/error)

## 6. Error Handling Hardening

- [x] 6.1 Ensure tool execution errors (non-zero exit, file not found, permission denied) are captured as messages and fed back to the model, not swallowed
- [x] 6.2 On any error during the pipeline (provider timeout, tool failure, consensus error), auto-save the session before displaying the error
- [x] 6.3 Add a `pipeline_error` telemetry event for all error conditions
- [x] 6.4 Verify that session resume after an error restores the conversation up to the point of failure

## 7. End-to-End Eval Fixtures

- [x] 7.1 Create `internal/action/eval_test.go` with a mock provider that returns scripted tool calls (file_read → response, file_write → confirm → response)
- [x] 7.2 Test: file_read tool call → executor reads file → result fed back → model produces final response
- [x] 7.3 Test: file_write tool call → confirm → write applied → result fed back → model produces final response
- [x] 7.4 Test: shell_exec tool call → confirm → command runs → output fed back → model produces final response
- [x] 7.5 Test: session save after tool execution → load session → conversation state includes tool results
- [x] 7.6 Test: provider timeout → error displayed → session saved with partial state
