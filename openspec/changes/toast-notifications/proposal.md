## Why

User-facing feedback for background events (config saves, MCP reconnections, clipboard copies, errors) currently either gets swallowed silently or appended as system messages into the chat log. This means important status information competes with conversation content, and transient confirmations clutter the history. A non-blocking toast notification system gives the user timely, contextual feedback without interrupting their workflow or polluting the chat.

## What Changes

- Add a toast notification overlay system: a stack of up to 3 toasts anchored to the bottom-right corner of the viewport
- Each toast has a variant (Info/blue, Success/green, Warning/yellow, Error/red), a message string, and an auto-dismiss timer (3s default, 5s for errors)
- Toasts animate in, stack upward, and fade out on dismissal
- Wire existing events to toasts: config save -> Success, MCP reconnect -> Info, clipboard copy -> Success, pipeline errors -> Error
- Add a `ToastMsg` Bubble Tea message type for any component to trigger notifications

## Capabilities

### New Capabilities
- `toast-overlay`: Core toast rendering, stacking, auto-dismiss timer, variant styling
- `toast-event-wiring`: Integration with existing TUI events to emit toast notifications

### Modified Capabilities
<!-- No existing spec-level behavior changes -->

## Impact

- **Files created** (1): `internal/tui/toast.go`
- **Files modified** (3): `model.go`, `update.go`, `view.go`
- **Dependencies**: None new
- **Scope**: ~200-300 lines
