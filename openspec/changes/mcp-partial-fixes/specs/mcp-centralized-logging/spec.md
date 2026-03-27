## ADDED Requirements

### Requirement: All MCP JSON-RPC traffic is logged when debug is enabled
When `mcp.debug: true` is set in config, every JSON-RPC request and response SHALL be logged to `~/.config/polycode/mcp-debug.log` with timestamp, server name, direction (→/←), method, and truncated params/result.

#### Scenario: Initialize handshake is logged
- **WHEN** debug is enabled and a server connects
- **THEN** the initialize request and response appear in the debug log

#### Scenario: Tool discovery is logged
- **WHEN** debug is enabled and tools/list is called
- **THEN** the request and response appear in the debug log

#### Scenario: Resource and prompt discovery are logged
- **WHEN** debug is enabled and resources/list or prompts/list is called
- **THEN** the requests and responses appear in the debug log

#### Scenario: HTTP transport requests are logged
- **WHEN** debug is enabled and an HTTP MCP server is called
- **THEN** the request and response appear in the debug log

### Requirement: Logging is centralized in the transport layer
LogRequest/LogResponse calls SHALL be placed inside `serverConn.sendRequest()` and `httpConn.sendRequest()` so all traffic is captured automatically without per-callsite instrumentation.

#### Scenario: New MCP methods are automatically logged
- **WHEN** a new MCP method is added (e.g., resources/read) and called through sendRequest
- **THEN** it is automatically logged without additional instrumentation code
