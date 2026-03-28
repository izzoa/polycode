## Context

The TUI already has overlay patterns: `showHelp` renders a full-screen help overlay (toggled by `?`), and `showProvenance` renders a provenance panel (toggled by `p`). The MCP dashboard follows the same pattern: a `showMCPDashboard` bool, a render method, and key routing that intercepts events when the dashboard is open.

All needed data is already exposed by MCPClient: `Status()` for server table, `Tools()` for per-server tool lists, `Resources()` / `Prompts()` for counts, `CallCount()` for session stats, `IsServerReadOnly()` for the read-only flag.

## Goals / Non-Goals

**Goals:**
- Toggle MCP dashboard from chat view via `m` key (when input empty) or clicking MCP indicator
- Show server table: name, transport, status (green/red/gray), tools, read-only, errors
- Show per-server tool list with prefixed names
- Show aggregate stats: total tools, total calls
- Quick actions: `r` to reconnect, `t` to test selected server
- Cursor navigation (j/k) to select a server for actions
- `Esc` closes the dashboard

**Non-Goals:**
- Full CRUD in the dashboard (add/edit/delete stay in `/settings` wizard)
- Real-time streaming updates (data refreshed on open and on reconnect/test results)
- Scrollable tool list (truncate if too many)

## Decisions

### D1: Full-screen overlay (like help), not a side panel

**Choice**: The dashboard takes over the full view (same as `renderHelp()`), not a split panel. Toggled by a bool, renders via `renderMCPDashboard()`, dismissed by Esc.

**Rationale**: Consistent with existing overlays. A side panel would require layout changes to the chat view. The dashboard is read-mostly — users glance at it and close it.

### D2: MCPDashboardData message carries all display data

**Choice**: A new `MCPDashboardDataMsg` carries server statuses, per-server tool names, resource/prompt counts, call count, and read-only flags — everything needed for rendering. Sent from app.go when dashboard is opened.

**Alternative**: Have the TUI model store references to MCPClient and pull data directly. Rejected — breaks the Bubble Tea message-passing architecture and creates import cycles.

### D3: `m` key toggles dashboard (when input empty)

**Choice**: Press `m` with empty textarea to toggle. Same guard pattern as `p` (provenance) and `?` (help).

**Rationale**: `m` for MCP is mnemonic and not used for any other shortcut in chat mode.

### D4: Cursor for server selection + quick actions

**Choice**: j/k navigates a cursor over servers. `r` reconnects the selected server, `t` tests it. Results update the dashboard data via existing `MCPTestResultMsg`/`MCPStatusMsg`.

### D5: Tab bar indicator becomes clickable

**Choice**: When the tab bar is focused (arrow keys mode) and the user navigates past the last provider tab, the MCP indicator becomes selectable. Pressing Enter on it opens the dashboard. This adds to the existing tab navigation without new interaction patterns.
