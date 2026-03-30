## Why

The `/compact` command is advertised in the command palette but returns "Context compaction not yet implemented — use /clear to reset". Users hitting 80% context pressure see a warning suggesting `/compact`, but the only recourse is `/clear` which destroys all conversation history. The auto-summarization function (`summarizeConversation`) already exists and triggers automatically at 80% — it just needs to be wired to the slash command as a user-triggered action.

## What Changes

- Wire `/compact` in `update.go` to trigger `summarizeConversation()` via the submit handler
- Send a toast notification showing before/after token counts
- Update the chat status message to reflect the compacted state

## Capabilities

### New Capabilities
<!-- None — wiring an existing internal function to an existing UI entry point -->

### Modified Capabilities
<!-- No spec-level changes -->

## Impact

- **Files created** (0)
- **Files modified** (2): `internal/tui/update.go` (remove stub), `cmd/polycode/app.go` (add compact handler)
- **Dependencies**: None
- **Config schema**: No changes
- **Scope**: ~50 lines
