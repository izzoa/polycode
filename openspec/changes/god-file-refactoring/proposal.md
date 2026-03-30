## Why

`cmd/polycode/app.go` (2,428 lines) and `internal/tui/update.go` (2,927 lines) are the two largest files in the project and growing with every feature. `app.go` contains a single `startTUI()` function that wires ~26 handler closures, the entire query pipeline, MCP notifications, session management, and helper functions. `update.go` dispatches all TUI message types and keyboard input across 7+ view modes in one function. Both files are maintenance hazards: difficult to navigate, prone to merge conflicts, and resistant to isolated testing.

Splitting these files by concern — without changing any behavior — reduces cognitive load, makes code review faster, and unblocks the headless-mode pipeline extraction.

## What Changes

- Split `app.go` into 4 focused files by handler category
- Split `update.go` into 5 focused files by view mode / message type
- No behavioral changes, no interface changes, no new packages
- All functions remain in their current packages

## Capabilities

### New Capabilities
<!-- None — this is a pure refactor -->

### Modified Capabilities
<!-- No capability changes — internal file reorganization only -->

## Impact

- **Files created** (7): `cmd/polycode/app_handlers.go`, `app_mcp.go`, `app_session.go`, `app_query.go`, `internal/tui/update_chat.go`, `update_approval.go`, `update_settings.go`
- **Files modified** (2): `cmd/polycode/app.go`, `internal/tui/update.go`
- **Dependencies**: None
- **Config schema**: No changes
- **Scope**: ~0 net new lines (pure move/split)
