## Context

Polycode's TUI currently handles transient feedback inconsistently. Config saves produce no visible confirmation. MCP reconnections log to debug output. Clipboard copies show a brief inline message. Errors sometimes appear in the chat, sometimes in the status bar. There is no unified notification mechanism for ephemeral, non-conversational feedback.

## Goals / Non-Goals

**Goals:**
- Unified notification system for transient status messages
- Non-blocking: toasts never steal focus or require interaction
- Up to 3 concurrent toasts stacked vertically in bottom-right
- Auto-dismiss with configurable duration per variant
- Four visual variants with distinct colors: Info, Success, Warning, Error

**Non-Goals:**
- Persistent notification log or history
- Interactive toasts (buttons, links, dismiss on click)
- Toast queuing beyond the 3-visible limit (oldest evicted)
- Sound or system notifications

## Decisions

### 1. Toast as a standalone component in toast.go

**Decision**: All toast types, state, rendering, and tick logic live in a single `toast.go` file. Model holds a `[]Toast` slice.

**Rationale**: Toasts are self-contained. Keeping them in one file avoids scattering small pieces across model/view/update. The slice on Model is the only integration point.

### 2. Timer-based dismissal via Bubble Tea Tick

**Decision**: Each toast spawns a `tea.Tick` command on creation. When the tick fires, the toast is removed from the slice.

**Rationale**: This uses Bubble Tea's built-in tick mechanism rather than goroutines or manual time tracking. Each toast gets a unique ID to match ticks to toasts.

### 3. Newest toast at the bottom of the stack

**Decision**: New toasts push to the bottom of the visual stack (closest to status bar). Older toasts shift upward.

**Rationale**: The user's eye naturally rests near the bottom of the terminal. Newest = most relevant = most visible position.

### 4. ToastMsg as the universal trigger

**Decision**: A `ToastMsg{Variant, Text}` message type is the single way to create toasts. Any Update handler can return a `ToastMsg` command.

**Rationale**: Decouples toast creation from toast rendering. Existing event handlers just emit a ToastMsg alongside their normal logic.

## Risks / Trade-offs

- **[Risk] Toast overlaps content** -> Mitigation: Render as an overlay in the last step of View(), positioned absolutely. Reserve no layout space.
- **[Risk] Tick accumulation** -> Mitigation: Each tick carries a toast ID; expired/evicted toasts ignore their tick.
- **[Risk] Too many toasts during rapid events** -> Mitigation: Cap at 3 visible; new toast evicts oldest if full.
