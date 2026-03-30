## ADDED Requirements

### Requirement: Per-tool permission policies
The system SHALL enforce per-tool approval policies (allow, ask, deny) from a permissions config file.

#### Scenario: Tool set to allow
- **WHEN** a tool has policy `allow` and the model calls it
- **THEN** the tool executes without confirmation

#### Scenario: Tool set to deny
- **WHEN** a tool has policy `deny` and the model calls it
- **THEN** the tool call is rejected and an error is returned to the model

#### Scenario: Tool set to ask (default)
- **WHEN** a tool has policy `ask` or no policy configured
- **THEN** the TUI confirmation prompt is shown before execution

#### Scenario: Glob pattern matching for MCP tools
- **WHEN** a permission entry uses a glob pattern like `mcp_filesystem_*`
- **THEN** it matches all MCP tools from the filesystem server
