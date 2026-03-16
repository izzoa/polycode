## Why

Polycode's consensus pipeline currently produces text output but does not execute the tool calls embedded in that output. The `internal/action/` package (tool loop, file ops, shell exec) exists but is not wired into the main app runtime. This means polycode cannot actually edit files or run commands based on consensus — it's a read-only assistant pretending to be an agent. Phase 1 of the roadmap requires hardening this into a trustworthy execution core with telemetry, deterministic failure handling, and end-to-end test coverage.

## What Changes

- **Wire tool execution into the consensus pipeline** — when the primary model's consensus response includes tool calls (file_read, file_write, shell_exec), they must be executed, results fed back, and the loop continued until a final text response
- **Full working state in synthesis** — the consensus prompt should include tool results and file context from prior turns, not just raw provider responses
- **Execution state machine** — formalize the lifecycle: prompt → fan-out → collect → synthesize → tool-loop → confirm → save-session
- **Per-provider telemetry** — log latency, token counts, error rates, and timeout rates to a local file for debugging and future routing decisions
- **Tool results in session persistence** — tool execution results must be saved in the session file so session resume restores full working state
- **Deterministic failure handling** — timeouts, provider errors, and tool failures produce clear, inspectable behavior with no silent data loss
- **End-to-end eval fixtures** — golden-task tests covering file read, file edit, shell exec, and session resume

## Capabilities

### New Capabilities
- `tool-execution-runtime`: Wiring the existing tool loop into the consensus pipeline for live tool execution with confirmation, state feedback, and session persistence
- `provider-telemetry`: Per-provider latency, token, and error instrumentation logged locally

### Modified Capabilities
_(none — this activates and hardens existing code rather than changing external behavior contracts)_

## Impact

- **`cmd/polycode/app.go`**: Major changes — submit handler must detect tool calls in consensus stream, instantiate ToolLoop, run execution cycle, feed results back
- **`internal/consensus/pipeline.go`**: Consensus synthesis may need to carry richer context (tool results from prior turns)
- **`internal/action/`**: Executor needs a TUI-compatible confirm callback; ToolLoop needs to work within the Bubble Tea message architecture
- **`internal/config/session.go`**: Session must persist tool call results alongside messages
- **New `internal/telemetry/`**: Lightweight per-provider instrumentation
- **New `internal/action/eval/`**: End-to-end test fixtures
