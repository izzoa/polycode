## ADDED Requirements

### Requirement: Connect to MCP servers on startup
The system SHALL connect to MCP servers defined in config and discover their available tools.

#### Scenario: Stdio MCP server
- **WHEN** a config entry specifies an MCP server with `command` and `args`
- **THEN** the system spawns the process, connects via stdio, and calls `tools/list` to discover tools

#### Scenario: SSE MCP server
- **WHEN** a config entry specifies an MCP server with `url`
- **THEN** the system connects via HTTP SSE and calls `tools/list`

#### Scenario: MCP server unavailable
- **WHEN** an MCP server fails to connect
- **THEN** the system logs a warning and continues without that server's tools

### Requirement: MCP tools available to primary model
The system SHALL include discovered MCP tools in the tool definitions sent to the primary model during consensus.

#### Scenario: MCP tool called by model
- **WHEN** the primary model issues a tool call for an MCP tool
- **THEN** the system routes the call to the correct MCP server and returns the result
