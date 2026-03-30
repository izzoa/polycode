## ADDED Requirements

### Requirement: MCP dashboard toggled from chat view
The TUI SHALL support toggling an MCP dashboard overlay from the chat view via the `m` key (when input is empty) or by selecting the MCP indicator in the tab bar.

#### Scenario: Toggle with m key
- **WHEN** the user presses `m` with an empty textarea in chat mode
- **THEN** the MCP dashboard overlay is shown

#### Scenario: Close with Esc
- **WHEN** the dashboard is open and the user presses `Esc`
- **THEN** the dashboard closes and the chat view is restored

#### Scenario: Toggle off with m key
- **WHEN** the dashboard is open and the user presses `m`
- **THEN** the dashboard closes

### Requirement: Dashboard shows server table
The dashboard SHALL display a table of all configured MCP servers with columns: Name, Transport, Status, Tools, Read-Only.

#### Scenario: Connected server
- **WHEN** a server named "filesystem" is connected via stdio with 3 tools and read_only=true
- **THEN** the table shows: filesystem | stdio | ✓ connected (green) | 3 | yes

#### Scenario: Failed server
- **WHEN** a server named "postgres" failed with "connection refused"
- **THEN** the table shows: postgres | sse | ✗ failed (red) | — | yes, with the error message below

#### Scenario: Disconnected server
- **WHEN** a server named "github" is configured but not connected
- **THEN** the table shows: github | stdio | disconnected (gray) | — | no

### Requirement: Dashboard shows per-server tools
The dashboard SHALL list discovered tools grouped by server, showing their prefixed names.

#### Scenario: Server with tools
- **WHEN** "filesystem" has 3 tools
- **THEN** the tools section shows: mcp_filesystem_read_file, mcp_filesystem_write_file, mcp_filesystem_list_dir

#### Scenario: Server with no tools
- **WHEN** a server is connected but has 0 tools
- **THEN** no tools are listed for that server

### Requirement: Dashboard shows aggregate stats
The dashboard SHALL show total tools across all servers and total MCP calls this session.

#### Scenario: Stats displayed
- **WHEN** there are 2 connected servers with 8 total tools and 12 calls made
- **THEN** the footer shows "8 tools across 2 servers | 12 calls this session"

### Requirement: Dashboard supports quick actions
The dashboard SHALL support cursor navigation (j/k) over servers and quick actions: `r` to reconnect, `t` to test.

#### Scenario: Reconnect selected server
- **WHEN** the user selects "postgres" and presses `r`
- **THEN** a reconnect is triggered for postgres and the dashboard updates

#### Scenario: Test selected server
- **WHEN** the user selects "github" and presses `t`
- **THEN** a connection test is triggered and the result updates the dashboard

### Requirement: Dashboard data refreshed on open
When the dashboard is opened, the TUI SHALL request fresh MCP data from the app layer.

#### Scenario: Data request on open
- **WHEN** the user opens the dashboard
- **THEN** a callback fires that sends current server status, tools, and stats to the TUI
