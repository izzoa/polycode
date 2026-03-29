## Why

When exploring a problem, users often want to try alternate approaches without losing their current conversation state. Currently, the only option is to start a fresh session and re-explain context. Session branching lets users fork at any point, preserving history up to the branch point while diverging from there. This is especially valuable for comparing consensus results versus single-model answers, or testing different tool execution strategies.

## What Changes

- `/branch` command creates a new session forked from the current session at the current exchange index
- Branch metadata tracks: parent session ID, source exchange index
- The new session starts with all conversation history up to the branch point
- Session picker shows branch relationships: child sessions indented under their parent
- Branch sessions are fully independent after forking (no shared state)

## Capabilities

### New Capabilities
- `session-fork`: Core branching logic -- forking sessions, copying history, tracking parent relationships
- `branch-picker`: Session picker enhancements to display branch hierarchy

### Modified Capabilities
<!-- No existing spec-level changes -->

## Impact

- **Files modified** (4): `internal/config/session.go`, `internal/tui/update.go`, `internal/tui/session_picker.go`, `cmd/polycode/app.go`
- **Files created**: None
- **Dependencies**: None new
- **Scope**: ~300-400 lines
