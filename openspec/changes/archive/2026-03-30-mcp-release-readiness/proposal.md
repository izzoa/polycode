## Why

The MCP enhancement implementation (18 features across 6 phases + partial fixes) is functionally complete and passing all tests, but a pre-release audit identified gaps that would cause issues for maintainers and users: critical new code paths have no unit tests, CLAUDE.md has no MCP guidance, config validation ignores MCP servers entirely, and the tool naming format has an undetected collision edge case. These must be addressed before the feature can ship confidently.

## What Changes

- **Unit tests**: Add tests for `Reconfigure()`, `TestConnection()`, `parseSSEResponse()`, HTTP transport, multiplexed reader, and resource/prompt discovery
- **CLAUDE.md**: Add MCP architecture, key patterns, tool naming, and "When Modifying" guidance
- **Config validation**: Validate MCP server configs in `Validate()` — name required, no duplicates, command or URL required, timeout positive
- **Tool name collision detection**: Detect and warn/error when two servers produce the same prefixed tool name during discovery
- **Resource/prompt discovery tests**: Test `discoverResources()` and `discoverPrompts()` against mock server

## Capabilities

### New Capabilities
- `mcp-test-coverage`: Unit tests for all untested MCP code paths
- `mcp-config-validation`: MCP server config validation in Config.Validate()
- `mcp-collision-detection`: Tool name collision detection during discovery
- `mcp-developer-docs`: CLAUDE.md MCP sections for maintainers

### Modified Capabilities

## Impact

- `internal/mcp/client_test.go` — new tests for Reconfigure, TestConnection, multiplexed reader, resource/prompt discovery
- `internal/mcp/http_transport_test.go` — new file: SSE parsing tests, HTTP transport tests
- `internal/config/config.go` — MCP validation in Validate()
- `internal/mcp/client.go` — collision detection in Connect()/discoverTools()
- `CLAUDE.md` — MCP sections added
