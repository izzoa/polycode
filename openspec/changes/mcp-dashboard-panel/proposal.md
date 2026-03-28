## Why

The MCP status indicator (`MCP: 1/1 ✓`) in the tab bar is purely informational — there's no way to inspect MCP server details from the chat view. Users must navigate to `/settings` and Tab to the MCP section to see connection status, tools, or errors. For a feature used during active coding sessions, this requires too many steps.

## What Changes

- **MCP Dashboard overlay** — A toggleable panel (like the help or provenance overlays) that shows a comprehensive MCP dashboard directly from the chat view.
- **Toggle trigger** — Clicking the MCP indicator in the tab bar, or pressing `m` when the input is empty, toggles the dashboard.
- **Server table** — Each server with name, transport, status (color-coded), tool count, read-only flag, and error details.
- **Tools list** — Discovered tools grouped by server, showing prefixed names.
- **Resources/Prompts counts** — Per-server resource and prompt counts if any.
- **Aggregate stats** — Total tools across all servers, total MCP calls this session.
- **Quick actions** — `r` to reconnect selected server, `t` to test, `Esc` to close, `/settings` hint for full management.

## Capabilities

### New Capabilities
- `mcp-dashboard-panel`: Toggleable MCP dashboard overlay in the chat view

### Modified Capabilities

## Impact

- `internal/tui/view.go` — New `renderMCPDashboard()` method, toggle in view dispatch
- `internal/tui/model.go` — `showMCPDashboard bool` field, MCP tools/resources/prompts data fields
- `internal/tui/update.go` — `m` key handler to toggle dashboard, key routing when dashboard is open
- `internal/tui/mcp_wizard.go` — New `MCPDashboardDataMsg` type with full server+tool data
- `cmd/polycode/app.go` — Send dashboard data alongside status updates
